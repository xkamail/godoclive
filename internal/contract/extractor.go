package contract

import (
	"go/ast"
	"go/types"

	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
	"golang.org/x/tools/go/packages"
)

// ExtractContract is the top-level orchestrator that calls all sub-extractors
// in order and assembles the complete request and response contract for a
// single endpoint.
//
// fn should be either *ast.FuncDecl or *ast.FuncLit.
func ExtractContract(
	route extractor.RawRoute,
	fn ast.Node,
	info *types.Info,
	paramNames resolver.HandlerParamNames,
	pkgs []*packages.Package,
) (model.RequestDef, []model.ResponseDef, []string) {

	var body *ast.BlockStmt
	var fnType *ast.FuncType

	switch f := fn.(type) {
	case *ast.FuncDecl:
		body = f.Body
		fnType = f.Type
	case *ast.FuncLit:
		body = f.Body
		fnType = f.Type
	}

	// If we couldn't get param names from the passed-in value, resolve them now.
	if paramNames == (resolver.HandlerParamNames{}) && fnType != nil {
		paramNames = resolver.ResolveHandlerParams(fnType, info)
	}

	// Check for arpc-style handler: func(context.Context, *T) (*U, error).
	// These handlers derive request/response types from the function signature
	// rather than from body-parsing calls inside the handler.
	if fnType != nil && paramNames == (resolver.HandlerParamNames{}) {
		if req, responses, ok := extractArpcContract(route, fnType, body, info); ok {
			return req, responses, nil
		}
	}

	var unresolved []string

	// 1. Path parameters.
	pathParams := ExtractPathParams(route.Path, body, info, paramNames)

	// 2. Query parameters.
	queryParams := ExtractQueryParams(body, info, paramNames)

	// 3. Headers.
	headers := ExtractHeaders(body, info, paramNames)

	// 4. Request body.
	bodyResult := ExtractBody(body, info, paramNames)
	unresolved = append(unresolved, bodyResult.Unresolved...)

	// Promote ShouldBindQuery fields to QueryParams.
	if bodyResult.BindQueryType != nil {
		promoted := promoteStructFields(bodyResult.BindQueryType, "form", "query")
		queryParams = append(queryParams, promoted...)
	}

	// Promote ShouldBindHeader fields to Headers.
	if bodyResult.BindHeaderType != nil {
		promoted := promoteStructFields(bodyResult.BindHeaderType, "header", "header")
		headers = append(headers, promoted...)
	}

	// Deduplicate: remove any query param whose name matches a path param.
	queryParams = deduplicateParams(pathParams, queryParams)

	req := model.RequestDef{
		PathParams:  pathParams,
		QueryParams: queryParams,
		Headers:     headers,
		ContentType: bodyResult.ContentType,
		IsMultipart: bodyResult.IsMultipart,
	}

	// If body extraction found file params, add them as path params might
	// contain file uploads.
	if len(bodyResult.FileParams) > 0 {
		// File params are special — they go alongside the body info.
		// For multipart endpoints, we don't set Body (it's the files).
		if bodyResult.BodyType == nil {
			// Pure file upload — no JSON body struct.
			req.Body = nil
		}
	}

	// Body type will be mapped to a TypeDef by the struct mapper later.
	// For now, store a reference if we have one.
	if bodyResult.BodyType != nil {
		req.Body = typeRefDef(bodyResult.BodyType)
	}

	// 5. Responses.
	responses, respUnresolved := ExtractResponses(body, info, paramNames, pkgs)
	unresolved = append(unresolved, respUnresolved...)

	return req, responses, unresolved
}

// extractArpcContract handles arpc-style handlers with the signature:
//
//	func(context.Context, *ParamsType) (*ResultType, error)
//
// The second parameter is the request body type and the first return value
// is the response body type. It also scans the handler body for arpc.NewError
// and arpc.NewErrorCode calls to extract specific error codes.
// Returns false if the signature does not match.
func extractArpcContract(route extractor.RawRoute, fnType *ast.FuncType, body *ast.BlockStmt, info *types.Info) (model.RequestDef, []model.ResponseDef, bool) {
	if info == nil || fnType == nil {
		return model.RequestDef{}, nil, false
	}

	// Require exactly 2 params: (context.Context, *T)
	if fnType.Params == nil || fnType.Params.NumFields() != 2 {
		return model.RequestDef{}, nil, false
	}

	// Require exactly 2 returns: (*U, error)
	if fnType.Results == nil || fnType.Results.NumFields() != 2 {
		return model.RequestDef{}, nil, false
	}

	// First param must be context.Context.
	firstParamType := info.TypeOf(fnType.Params.List[0].Type)
	if firstParamType == nil || !isContextType(firstParamType) {
		return model.RequestDef{}, nil, false
	}

	// Second return must be error.
	lastReturnType := info.TypeOf(fnType.Results.List[len(fnType.Results.List)-1].Type)
	if lastReturnType == nil || !isErrorType(lastReturnType) {
		return model.RequestDef{}, nil, false
	}

	// Second param is the request body type.
	bodyType := info.TypeOf(fnType.Params.List[1].Type)
	if bodyType == nil {
		return model.RequestDef{}, nil, false
	}
	// Dereference pointer to get the underlying struct type.
	if ptr, ok := bodyType.(*types.Pointer); ok {
		bodyType = ptr.Elem()
	}

	// First return is the response body type.
	resultType := info.TypeOf(fnType.Results.List[0].Type)
	if resultType == nil {
		return model.RequestDef{}, nil, false
	}
	if ptr, ok := resultType.(*types.Pointer); ok {
		resultType = ptr.Elem()
	}

	// Build path params from route pattern.
	pathParams := ExtractPathParams(route.Path, nil, info, resolver.HandlerParamNames{})

	req := model.RequestDef{
		PathParams:  pathParams,
		Body:        typeRefDef(bodyType),
		ContentType: "application/json",
	}

	// Build arpc envelope response: {"ok": true, "result": <ResultType>}
	resultRef := typeRefDef(resultType)
	okResponse := &model.TypeDef{
		Name: "arpc.OKResponse",
		Kind: model.KindStruct,
		Fields: []model.FieldDef{
			{
				Name:     "OK",
				JSONName: "ok",
				Type:     model.TypeDef{Name: "bool", Kind: model.KindPrimitive},
				Required: true,
			},
			{
				Name:     "Result",
				JSONName: "result",
				Type:     *resultRef,
			},
		},
	}

	// Scan handler body for arpc error codes.
	arpcErrors := extractArpcErrors(body, info)

	responses := []model.ResponseDef{
		{
			StatusCode:  200,
			Body:        okResponse,
			Description: "OK",
			ContentType: "application/json",
		},
	}

	// Add specific error responses from arpc.NewError/NewErrorCode calls.
	// These are OKErrors — HTTP 200 with ok=false.
	for _, ae := range arpcErrors {
		responses = append(responses, model.ResponseDef{
			StatusCode:  200,
			Body:        arpcErrorResponseDef(ae.code, ae.message),
			Description: ae.description(),
			ContentType: "application/json",
			Source:      "arpc",
		})
	}

	return req, responses, true
}

// arpcErrorDef represents an error found in an arpc handler body.
type arpcErrorDef struct {
	code    string // from NewErrorCode first arg, empty for NewError
	message string // error message
}

func (e arpcErrorDef) description() string {
	if e.code != "" {
		return e.code + ": " + e.message
	}
	return e.message
}

// arpcErrorResponseDef builds the TypeDef for an arpc error envelope with
// specific code and message values.
func arpcErrorResponseDef(code, message string) *model.TypeDef {
	fields := []model.FieldDef{
		{
			Name:     "OK",
			JSONName: "ok",
			Type:     model.TypeDef{Name: "bool", Kind: model.KindPrimitive},
			Required: true,
			Example:  false,
		},
		{
			Name:     "Error",
			JSONName: "error",
			Type: model.TypeDef{
				Name: "ErrorDetail",
				Kind: model.KindStruct,
				Fields: []model.FieldDef{
					{Name: "Code", JSONName: "code", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}, Example: code},
					{Name: "Message", JSONName: "message", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}, Example: message},
				},
			},
		},
	}
	return &model.TypeDef{
		Name:   "arpc.ErrorResponse",
		Kind:   model.KindStruct,
		Fields: fields,
	}
}

// extractArpcErrors scans a function body for arpc.NewError and arpc.NewErrorCode
// calls, including package-level variable references like:
//
//	var ErrNotFound = arpc.NewErrorCode("order/not-found", "order not found")
//	return nil, ErrNotFound
func extractArpcErrors(body *ast.BlockStmt, info *types.Info) []arpcErrorDef {
	if body == nil {
		return nil
	}

	seen := make(map[string]bool)
	var errors []arpcErrorDef

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check for arpc.NewErrorCode(code, message) or arpc.NewError(message)
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		// Match by package name "arpc" — works for both direct calls and aliased imports.
		if ident.Name != "arpc" {
			return true
		}

		switch sel.Sel.Name {
		case "NewErrorCode":
			if len(call.Args) >= 2 {
				code := extractStringLit(call.Args[0])
				msg := extractStringLit(call.Args[1])
				if code != "" && !seen[code] {
					seen[code] = true
					errors = append(errors, arpcErrorDef{code: code, message: msg})
				}
			}
		case "NewError":
			if len(call.Args) >= 1 {
				msg := extractStringLit(call.Args[0])
				if msg != "" && !seen[msg] {
					seen[msg] = true
					errors = append(errors, arpcErrorDef{message: msg})
				}
			}
		}

		return true
	})

	return errors
}

// isContextType returns true if t is context.Context.
func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		// context.Context is an interface — check underlying.
		if iface, ok := t.Underlying().(*types.Interface); ok {
			_ = iface
			// Check by string representation as fallback.
			return t.String() == "context.Context"
		}
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

// isErrorType returns true if t is the built-in error interface.
func isErrorType(t types.Type) bool {
	// The error type is a named interface in the universe scope.
	named, ok := t.(*types.Named)
	if ok {
		return named.Obj().Name() == "error" && named.Obj().Pkg() == nil
	}
	return t.String() == "error"
}

// deduplicateParams removes query params whose name matches a path param.
// This avoids double-reporting when r.FormValue() is used for a path param.
func deduplicateParams(pathParams, queryParams []model.ParamDef) []model.ParamDef {
	if len(pathParams) == 0 || len(queryParams) == 0 {
		return queryParams
	}
	pathNames := make(map[string]bool, len(pathParams))
	for _, p := range pathParams {
		pathNames[p.Name] = true
	}
	var filtered []model.ParamDef
	for _, q := range queryParams {
		if !pathNames[q.Name] {
			filtered = append(filtered, q)
		}
	}
	return filtered
}
