package contract_test

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xkamail/godoclive/internal/contract"
	"github.com/xkamail/godoclive/internal/extractor"
	"github.com/xkamail/godoclive/internal/loader"
	"github.com/xkamail/godoclive/internal/resolver"
	"golang.org/x/tools/go/packages"
)

func testdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

// resolveRoute is a test helper that resolves a route to its handler FuncDecl/FuncLit
// and returns the function body, type info, and param names.
type resolvedHandler struct {
	body       *ast.BlockStmt
	funcType   *ast.FuncType
	paramNames resolver.HandlerParamNames
}

func resolveRoutes(t *testing.T, dir string, ext extractor.Extractor) ([]extractor.RawRoute, map[string]resolvedHandler) {
	t.Helper()
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo
	handlers := make(map[string]resolvedHandler)

	for _, route := range routes {
		key := route.Method + " " + route.Path
		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Logf("ResolveHandler(%s) failed: %v", key, err)
			continue
		}

		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}

		pn := resolver.ResolveHandlerParams(fnType, info)
		handlers[key] = resolvedHandler{body: body, funcType: fnType, paramNames: pn}
	}

	return routes, handlers
}

// --- Path Parameter Tests ---

func TestExtractPathParams_ChiBasic(t *testing.T) {
	dir := testdataDir("chi-basic")
	routes, handlers := resolveRoutes(t, dir, &extractor.ChiExtractor{})

	for _, route := range routes {
		key := route.Method + " " + route.Path
		h := handlers[key]
		params := contract.ExtractPathParams(route.Path, h.body, nil, h.paramNames)

		switch route.Path {
		case "/users":
			if len(params) != 0 {
				t.Errorf("%s: expected 0 path params, got %d", key, len(params))
			}
		case "/users/{id}":
			if len(params) != 1 {
				t.Errorf("%s: expected 1 path param, got %d", key, len(params))
				continue
			}
			p := params[0]
			if p.Name != "id" {
				t.Errorf("%s: expected name 'id', got %q", key, p.Name)
			}
			if !p.Required {
				t.Errorf("%s: path param should be required", key)
			}
			if p.Type != "uuid" {
				t.Errorf("%s: expected type 'uuid' (from name heuristic), got %q", key, p.Type)
			}
			if p.In != "path" {
				t.Errorf("%s: expected In='path', got %q", key, p.In)
			}
		}
	}
}

func TestExtractPathParams_TypeUpgrade(t *testing.T) {
	// chi-basic's GetUser uses chi.URLParam(r, "id") without strconv
	// so type stays as "uuid" from heuristic.
	// ListUsers uses strconv.Atoi on "page" query param — but that's a query param, not path.
	// We test the type inference directly with pattern-only extraction.
	params := contract.ExtractPathParams("/items/{itemId}/reviews/{page}", nil, nil, resolver.HandlerParamNames{})

	if len(params) != 2 {
		t.Fatalf("expected 2 path params, got %d", len(params))
	}

	// itemId should be "uuid" (ends with Id)
	if params[0].Name != "itemId" || params[0].Type != "uuid" {
		t.Errorf("param 0: expected itemId/uuid, got %s/%s", params[0].Name, params[0].Type)
	}

	// page should be "integer"
	if params[1].Name != "page" || params[1].Type != "integer" {
		t.Errorf("param 1: expected page/integer, got %s/%s", params[1].Name, params[1].Type)
	}
}

func TestExtractPathParams_Examples(t *testing.T) {
	params := contract.ExtractPathParams("/users/{id}/posts/{slug}", nil, nil, resolver.HandlerParamNames{})
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}

	if params[0].Example == "" {
		t.Error("id should have an example")
	}
	if params[1].Example != "my-resource" {
		t.Errorf("slug example: expected 'my-resource', got %q", params[1].Example)
	}
}

func TestExtractPathParams_GorillaBasic(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	info := pkgs[0].TypesInfo

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Find GET /users/{id} handler
	for _, route := range routes {
		if route.Method != "GET" || route.Path != "/users/{id}" {
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler failed: %v", err)
		}
		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}
		pn := resolver.ResolveHandlerParams(fnType, info)

		params := contract.ExtractPathParams("/users/{id}", body, info, pn)
		if len(params) != 1 {
			t.Fatalf("expected 1 path param, got %d", len(params))
		}
		if params[0].Name != "id" {
			t.Errorf("expected param name 'id', got %q", params[0].Name)
		}
		return
	}
	t.Fatal("route GET /users/{id} not found")
}

func TestExtractPathParams_GorillaRegexParam(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	info := pkgs[0].TypesInfo

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Find GET /api/v1/items/{id} handler
	for _, route := range routes {
		if route.Method != "GET" || route.Path != "/api/v1/items/{id}" {
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler failed: %v", err)
		}
		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}
		pn := resolver.ResolveHandlerParams(fnType, info)

		params := contract.ExtractPathParams("/api/v1/items/{id}", body, info, pn)
		if len(params) != 1 {
			t.Fatalf("expected 1 path param, got %d", len(params))
		}
		if params[0].Name != "id" {
			t.Errorf("expected param name 'id', got %q", params[0].Name)
		}
		// With types.Info, strconv.Atoi upgrade resolves mux.Vars indirection
		if params[0].Type != "integer" {
			t.Errorf("expected type 'integer' (strconv.Atoi upgrade), got %q", params[0].Type)
		}
		return
	}
	t.Fatal("route GET /api/v1/items/{id} not found")
}

func TestExtractPathParams_GorillaTypeUpgrade(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	info := pkgs[0].TypesInfo

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Find DELETE /users/{id} — uses mux.Vars(r)["id"]
	for _, route := range routes {
		if route.Method != "DELETE" || route.Path != "/users/{id}" {
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler failed: %v", err)
		}
		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}
		pn := resolver.ResolveHandlerParams(fnType, info)

		params := contract.ExtractPathParams("/users/{id}", body, info, pn)
		if len(params) != 1 {
			t.Fatalf("expected 1 path param, got %d", len(params))
		}
		if params[0].Name != "id" {
			t.Errorf("expected param name 'id', got %q", params[0].Name)
		}
		return
	}
	t.Fatal("route DELETE /users/{id} not found")
}

// --- Query Parameter Tests ---

func TestExtractQueryParams_ChiBasic(t *testing.T) {
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "GET /users" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		params := contract.ExtractQueryParams(fd.Body, info, pn)

		if len(params) != 2 {
			t.Fatalf("ListUsers: expected 2 query params, got %d", len(params))
		}

		// Find page and limit params.
		paramMap := make(map[string]int)
		for i, p := range params {
			paramMap[p.Name] = i
		}

		pageIdx, ok := paramMap["page"]
		if !ok {
			t.Fatal("missing 'page' query param")
		}
		if !params[pageIdx].Required {
			t.Error("page should be required (has guard block)")
		}
		if params[pageIdx].In != "query" {
			t.Errorf("page In: expected 'query', got %q", params[pageIdx].In)
		}

		limitIdx, ok := paramMap["limit"]
		if !ok {
			t.Fatal("missing 'limit' query param")
		}
		if params[limitIdx].Required {
			t.Error("limit should not be required")
		}
	}
}

func TestExtractQueryParams_GinBasic(t *testing.T) {
	dir := testdataDir("gin-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GinExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "GET /api/v1/items" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		params := contract.ExtractQueryParams(fd.Body, info, pn)

		if len(params) != 2 {
			t.Fatalf("ListItems: expected 2 query params, got %d", len(params))
		}

		paramMap := make(map[string]int)
		for i, p := range params {
			paramMap[p.Name] = i
		}

		// search via c.Query("search")
		searchIdx, ok := paramMap["search"]
		if !ok {
			t.Fatal("missing 'search' query param")
		}
		if params[searchIdx].Default != nil {
			t.Error("search should not have a default")
		}

		// limit via c.DefaultQuery("limit", "20")
		limitIdx, ok := paramMap["limit"]
		if !ok {
			t.Fatal("missing 'limit' query param")
		}
		if params[limitIdx].Default == nil {
			t.Fatal("limit should have a default")
		}
		if *params[limitIdx].Default != "20" {
			t.Errorf("limit default: expected '20', got %q", *params[limitIdx].Default)
		}
	}
}

// --- Header Tests ---

func TestExtractHeaders_ChiBasic(t *testing.T) {
	// chi-basic handlers don't have explicit header access (just Content-Type
	// which is set via w.Header().Set, not r.Header.Get). So we expect 0 headers.
	dir := testdataDir("chi-basic")
	_, handlers := resolveRoutes(t, dir, &extractor.ChiExtractor{})

	pkgs, _ := loader.LoadPackages(dir, "./...")
	info := pkgs[0].TypesInfo

	for key, h := range handlers {
		headers := contract.ExtractHeaders(h.body, info, h.paramNames)
		if len(headers) != 0 {
			t.Errorf("%s: expected 0 headers, got %d", key, len(headers))
		}
	}
}

func TestExtractHeaders_AuthorizationExcluded(t *testing.T) {
	// The JWTAuth middleware in chi-basic uses r.Header.Get("Authorization").
	// This should be excluded. We test this by loading chi-basic and checking
	// that none of the handlers report Authorization as a header param.
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	// Walk all function decls and check for headers.
	for _, file := range pkgs[0].Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			pn := resolver.ResolveHandlerParams(fd.Type, info)
			headers := contract.ExtractHeaders(fd.Body, info, pn)
			for _, h := range headers {
				if h.Name == "Authorization" {
					t.Errorf("func %s: Authorization header should be excluded", fd.Name.Name)
				}
			}
		}
	}
}

// --- Body Tests ---

func TestExtractBody_ChiBasicJSON(t *testing.T) {
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "POST /users" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		result := contract.ExtractBody(fd.Body, info, pn)

		if result.BodyType == nil {
			t.Fatal("CreateUser: expected body type, got nil")
		}
		if result.ContentType != "application/json" {
			t.Errorf("CreateUser: expected content type 'application/json', got %q", result.ContentType)
		}
		if result.IsMultipart {
			t.Error("CreateUser: should not be multipart")
		}

		// Verify the resolved type name.
		named, ok := result.BodyType.(*types.Named)
		if !ok {
			t.Fatalf("CreateUser: expected named type, got %T", result.BodyType)
		}
		if named.Obj().Name() != "CreateUserRequest" {
			t.Errorf("CreateUser: expected type name 'CreateUserRequest', got %q", named.Obj().Name())
		}
	}
}

func TestExtractBody_GinBasicJSON(t *testing.T) {
	dir := testdataDir("gin-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GinExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "POST /api/v1/items" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		result := contract.ExtractBody(fd.Body, info, pn)

		if result.BodyType == nil {
			t.Fatal("CreateItem: expected body type, got nil")
		}
		if result.ContentType != "application/json" {
			t.Errorf("CreateItem: expected content type 'application/json', got %q", result.ContentType)
		}

		named, ok := result.BodyType.(*types.Named)
		if !ok {
			t.Fatalf("CreateItem: expected named type, got %T", result.BodyType)
		}
		if named.Obj().Name() != "CreateItemRequest" {
			t.Errorf("CreateItem: expected type name 'CreateItemRequest', got %q", named.Obj().Name())
		}
	}
}

func TestExtractBody_Multipart(t *testing.T) {
	dir := testdataDir("multipart")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		result := contract.ExtractBody(fd.Body, info, pn)

		switch key {
		case "POST /users/{id}/avatar":
			if !result.IsMultipart {
				t.Errorf("%s: expected IsMultipart=true", key)
			}
			if result.ContentType != "multipart/form-data" {
				t.Errorf("%s: expected content type 'multipart/form-data', got %q", key, result.ContentType)
			}
			if len(result.FileParams) != 1 {
				t.Errorf("%s: expected 1 file param, got %d", key, len(result.FileParams))
			} else if result.FileParams[0].Name != "avatar" {
				t.Errorf("%s: expected file param 'avatar', got %q", key, result.FileParams[0].Name)
			}

		case "PUT /users/{id}/profile":
			if result.IsMultipart {
				t.Errorf("%s: should not be multipart", key)
			}
			if result.BodyType == nil {
				t.Errorf("%s: expected body type, got nil", key)
			}
			if result.ContentType != "application/json" {
				t.Errorf("%s: expected content type 'application/json', got %q", key, result.ContentType)
			}

		case "POST /documents":
			if !result.IsMultipart {
				t.Errorf("%s: expected IsMultipart=true", key)
			}
			if len(result.FileParams) != 1 {
				t.Errorf("%s: expected 1 file param, got %d", key, len(result.FileParams))
			} else if result.FileParams[0].Name != "document" {
				t.Errorf("%s: expected file param 'document', got %q", key, result.FileParams[0].Name)
			}
		}
	}
}

func TestExtractBody_NoBody(t *testing.T) {
	// GET /users in chi-basic has no body — should return empty result.
	dir := testdataDir("chi-basic")
	_, handlers := resolveRoutes(t, dir, &extractor.ChiExtractor{})

	pkgs, _ := loader.LoadPackages(dir, "./...")
	info := pkgs[0].TypesInfo

	h := handlers["GET /users"]
	result := contract.ExtractBody(h.body, info, h.paramNames)
	if result.BodyType != nil {
		t.Error("GET /users should not have a body type")
	}
	if result.IsMultipart {
		t.Error("GET /users should not be multipart")
	}
}

// --- Multipart with query param (FormValue) ---

func TestExtractQueryParams_FormValue(t *testing.T) {
	dir := testdataDir("multipart")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "POST /documents" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		params := contract.ExtractQueryParams(fd.Body, info, pn)

		if len(params) != 1 {
			t.Fatalf("UploadDocument: expected 1 query param, got %d", len(params))
		}
		if params[0].Name != "title" {
			t.Errorf("expected param name 'title', got %q", params[0].Name)
		}
	}
}

// --- arpc Contract Tests ---

func TestExtractContract_ArpcHandler(t *testing.T) {
	dir := testdataDir("stdlib-arpc")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.StdlibExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	tested := make(map[string]bool)

	for _, route := range routes {
		key := route.Method + " " + route.Path

		// Find TypesInfo for the package containing this route's file.
		info := findInfoForRoute(route, pkgs)
		if info == nil {
			t.Logf("%s: no TypesInfo found", key)
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Errorf("ResolveHandler(%s) failed: %v", key, err)
			continue
		}

		var fn ast.Node
		var handlerInfo *types.Info
		if fd != nil {
			fn = fd
			handlerInfo = findHandlerInfo(fd, pkgs)
			if handlerInfo == nil {
				handlerInfo = info
			}
		} else if fl != nil {
			fn = fl
			handlerInfo = info
		}

		pn := resolver.HandlerParamNames{}
		if fd != nil {
			pn = resolver.ResolveHandlerParams(fd.Type, handlerInfo)
		} else if fl != nil {
			pn = resolver.ResolveHandlerParams(fl.Type, handlerInfo)
		}

		req, responses, _ := contract.ExtractContract(route, fn, handlerInfo, pn, pkgs)

		tested[key] = true

		switch key {
		case "POST /site.list":
			if req.Body == nil {
				t.Errorf("%s: expected request body (ListParams), got nil", key)
			} else if req.Body.Name != "ListParams" {
				t.Errorf("%s: expected body type 'ListParams', got %q", key, req.Body.Name)
			}
			if req.ContentType != "application/json" {
				t.Errorf("%s: expected content-type 'application/json', got %q", key, req.ContentType)
			}
			// arpc response: 200 OK + 200 error (site/invalid-page)
			if len(responses) < 2 {
				t.Errorf("%s: expected at least 2 responses, got %d", key, len(responses))
			} else {
				// 200 OK envelope
				if responses[0].Body == nil || responses[0].Body.Name != "ListResultResponse" {
					t.Errorf("%s: expected envelope 'ListResultResponse'", key)
				} else {
					resultField := responses[0].Body.Fields[1]
					if resultField.Type.Name != "ListResult" {
						t.Errorf("%s: expected result type 'ListResult', got %q", key, resultField.Type.Name)
					}
				}
				// Error response from arpc.NewErrorCode("site/invalid-page", ...)
				if responses[1].Description != "site/invalid-page: page must be >= 1" {
					t.Errorf("%s: expected error 'site/invalid-page: page must be >= 1', got %q", key, responses[1].Description)
				}
			}

		case "POST /site.create":
			if req.Body == nil {
				t.Errorf("%s: expected request body (CreateParams), got nil", key)
			} else if req.Body.Name != "CreateParams" {
				t.Errorf("%s: expected body type 'CreateParams', got %q", key, req.Body.Name)
			}
			// Should have: 200 OK + NewError("name is required") + NewErrorCode("site/invalid-domain", ...)
			if len(responses) < 3 {
				t.Errorf("%s: expected at least 3 responses, got %d", key, len(responses))
			} else {
				if responses[0].Body != nil {
					resultField := responses[0].Body.Fields[1]
					if resultField.Type.Name != "CreateResult" {
						t.Errorf("%s: expected result type 'CreateResult', got %q", key, resultField.Type.Name)
					}
				}
				// NewError("name is required")
				if responses[1].Description != "name is required" {
					t.Errorf("%s: expected error 'name is required', got %q", key, responses[1].Description)
				}
				// NewErrorCode("site/invalid-domain", "domain is required")
				if responses[2].Description != "site/invalid-domain: domain is required" {
					t.Errorf("%s: expected error 'site/invalid-domain: domain is required', got %q", key, responses[2].Description)
				}
			}

		case "POST /auth.me":
			// Context-only handler: func(context.Context) (*MeResult, error)
			// Should have no request body.
			if req.Body != nil {
				t.Errorf("%s: expected no request body, got %q", key, req.Body.Name)
			}
			if req.ContentType != "" {
				t.Errorf("%s: expected empty content-type, got %q", key, req.ContentType)
			}
			if len(responses) > 0 && responses[0].Body != nil {
				resultField := responses[0].Body.Fields[1]
				if resultField.Type.Name != "MeResult" {
					t.Errorf("%s: expected result type 'MeResult', got %q", key, resultField.Type.Name)
				}
			}

		case "GET /auth/{provider}":
			// Standard HTTP handler — should NOT have arpc body extraction.
			if len(req.PathParams) == 0 {
				t.Errorf("%s: expected path param 'provider'", key)
			}
		}
	}

	// Ensure all expected arpc routes were resolved and tested.
	for _, expected := range []string{"POST /site.list", "POST /site.create", "POST /auth.me"} {
		if !tested[expected] {
			t.Errorf("route %s was not resolved or tested", expected)
		}
	}
}

// findInfoForRoute returns the types.Info for the package containing the route.
func findInfoForRoute(route extractor.RawRoute, pkgs []*packages.Package) *types.Info {
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, f := range pkg.GoFiles {
			if f == route.File {
				return pkg.TypesInfo
			}
		}
	}
	for _, pkg := range pkgs {
		if pkg.TypesInfo != nil {
			return pkg.TypesInfo
		}
	}
	return nil
}

// findHandlerInfo finds TypesInfo for the package containing fd.
func findHandlerInfo(fd *ast.FuncDecl, pkgs []*packages.Package) *types.Info {
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if decl == fd {
					return pkg.TypesInfo
				}
			}
		}
	}
	return nil
}
