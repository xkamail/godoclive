package extractor

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/packages"
)

// StdlibExtractor extracts routes from net/http ServeMux registrations.
// Supports Go 1.22+ enhanced patterns ("GET /path/{id}") and pre-1.22 patterns.
type StdlibExtractor struct{}

// Extract walks all packages and extracts stdlib route registrations.
func (e *StdlibExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	var routes []RawRoute

	for _, pkg := range pkgs {
		if !isStdlibHTTPPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &stdlibWalker{
				fset:    pkg.Fset,
				file:    fpath,
				muxVars: make(map[string]bool),
				groups:  make(map[string]*groupInfo),
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				// Skip test and example functions.
				if strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Example") {
					continue
				}
				w.collectMuxParams(fn)
				w.walkBlock(fn.Body.List, nil)
			}
			routes = append(routes, w.routes...)
		}
	}

	return routes, nil
}

// isStdlibHTTPPackage returns true if the package imports net/http.
func isStdlibHTTPPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "net/http" {
			return true
		}
	}
	return false
}

// groupInfo tracks prefix and middleware for a mux group variable.
type groupInfo struct {
	prefix      string
	middlewares []ast.Expr
}

// stdlibWalker extracts stdlib routes from a single file.
type stdlibWalker struct {
	fset    *token.FileSet
	file    string
	routes  []RawRoute
	muxVars map[string]bool        // tracks variables assigned from http.NewServeMux()
	groups  map[string]*groupInfo  // tracks variables from mux.Group(prefix, mw...)
}

// walkBlock walks a list of statements looking for stdlib route registrations.
func (w *stdlibWalker) walkBlock(stmts []ast.Stmt, parentMW []ast.Expr) {
	scopeMW := copyExprs(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			w.handleAssign(s)
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			w.processCall(call, scopeMW)
		}
	}
}

// handleAssign detects mux := http.NewServeMux() and x := mux.Group(...) assignments.
func (w *stdlibWalker) handleAssign(assign *ast.AssignStmt) {
	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}

	// mux := http.NewServeMux()
	if ident, ok := sel.X.(*ast.Ident); ok {
		if ident.Name == "http" && sel.Sel.Name == "NewServeMux" {
			w.muxVars[lhs.Name] = true
			return
		}
	}

	// a := mux.Group("prefix", middleware...)
	// Accept Group() on any receiver — custom wrappers like httpmux.Mux also have Group().
	if sel.Sel.Name == "Group" && len(call.Args) >= 1 {
		recv, ok := sel.X.(*ast.Ident)
		if !ok {
			return
		}
		_ = recv

		gi := &groupInfo{}

		// First arg is the prefix string.
		if len(call.Args) >= 1 {
			gi.prefix = stringLitValue(call.Args[0])
		}

		// Remaining args are middleware expressions.
		if len(call.Args) >= 2 {
			gi.middlewares = append(gi.middlewares, call.Args[1:]...)
		}

		// Inherit parent group's prefix and middleware.
		if parent := w.groups[recv.Name]; parent != nil {
			gi.prefix = parent.prefix + gi.prefix
			gi.middlewares = append(copyExprs(parent.middlewares), gi.middlewares...)
		}

		w.groups[lhs.Name] = gi
	}
}

// collectMuxParams scans function parameters for *http.ServeMux and adds
// their names to muxVars so that mux.HandleFunc/mux.Handle calls are recognized
// when the ServeMux is passed as a parameter (e.g. func Mount(mux *http.ServeMux, ...)).
func (w *stdlibWalker) collectMuxParams(fn *ast.FuncDecl) {
	if fn.Type.Params == nil {
		return
	}
	for _, field := range fn.Type.Params.List {
		star, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}
		if ident.Name == "http" && sel.Sel.Name == "ServeMux" {
			for _, name := range field.Names {
				w.muxVars[name.Name] = true
			}
		}
	}
}

// processCall dispatches a call expression based on the method name.
func (w *stdlibWalker) processCall(call *ast.CallExpr, scopeMW []ast.Expr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name

	switch {
	case (name == "HandleFunc" || name == "Handle") && len(call.Args) >= 2:
		// Match http.HandleFunc, mux.HandleFunc, or any wrapper (e.g. httpmux.Mux)
		// that calls HandleFunc/Handle with an HTTP pattern string.
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "http" || w.muxVars[ident.Name] {
				w.addRoute(call, scopeMW)
				return
			}
			// Check if receiver is a group variable — apply prefix + middleware.
			if gi := w.groups[ident.Name]; gi != nil {
				groupMW := concatExprs(scopeMW, gi.middlewares)
				w.addRouteWithPrefix(call, groupMW, gi.prefix)
				return
			}
		}
		// Fallback: accept any x.HandleFunc/x.Handle where the first arg
		// is a string literal that looks like an HTTP route pattern.
		// This supports custom ServeMux wrappers (e.g. httpmux.Mux).
		if pattern := stringLitValue(call.Args[0]); pattern != "" && looksLikeHTTPPattern(pattern) {
			w.addRoute(call, scopeMW)
		}
	}
}

// addRoute parses the pattern string and records a route.
func (w *stdlibWalker) addRoute(call *ast.CallExpr, middlewares []ast.Expr) {
	pattern := stringLitValue(call.Args[0])
	if pattern == "" {
		return
	}

	method, path := parseStdlibPattern(pattern)

	// The handler is the last argument. Any args between pattern and handler
	// could be considered but stdlib only takes (pattern, handler).
	handler := call.Args[len(call.Args)-1]

	// Unwrap middleware wrapping: authMiddleware(handler) → collect authMiddleware, use inner handler.
	handler, wrappedMW := unwrapMiddleware(handler)
	allMW := concatExprs(middlewares, wrappedMW)

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        path,
		HandlerExpr: handler,
		Middlewares: copyExprs(allMW),
		File:        w.file,
		Line:        pos.Line,
	})
}

// addRouteWithPrefix parses the pattern, prepends a group prefix, and records a route.
func (w *stdlibWalker) addRouteWithPrefix(call *ast.CallExpr, middlewares []ast.Expr, prefix string) {
	pattern := stringLitValue(call.Args[0])
	if pattern == "" {
		return
	}

	method, path := parseStdlibPattern(pattern)

	// Prepend group prefix to the path.
	if prefix != "" {
		path = strings.TrimSuffix(prefix, "/") + path
	}

	handler := call.Args[len(call.Args)-1]
	handler, wrappedMW := unwrapMiddleware(handler)
	allMW := concatExprs(middlewares, wrappedMW)

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        path,
		HandlerExpr: handler,
		Middlewares: copyExprs(allMW),
		File:        w.file,
		Line:        pos.Line,
	})
}

// parseStdlibPattern parses a Go 1.22+ ServeMux pattern string.
// Patterns can be:
//   - "GET /users/{id}" → method="GET", path="/users/{id}"
//   - "POST /users"     → method="POST", path="/users"
//   - "/health"         → method="ANY", path="/health"
//   - "GET example.com/path" → method="GET", path="/path" (host ignored)
func parseStdlibPattern(pattern string) (method, path string) {
	pattern = strings.TrimSpace(pattern)

	// Check for method prefix: "GET /path" or "POST /path".
	parts := strings.SplitN(pattern, " ", 2)
	if len(parts) == 2 && isHTTPMethod(parts[0]) {
		method = parts[0]
		path = strings.TrimSpace(parts[1])
	} else {
		method = "ANY"
		path = pattern
	}

	// Strip host prefix if present: "example.com/path" → "/path".
	if !strings.HasPrefix(path, "/") {
		if idx := strings.Index(path, "/"); idx >= 0 {
			path = path[idx:]
		}
	}

	// Remove trailing {$} exact match marker (Go 1.22+).
	path = strings.TrimSuffix(path, "{$}")
	if path == "" {
		path = "/"
	}

	// Clean trailing slash for non-root paths to normalize,
	// but preserve it since in stdlib it means subtree matching.
	// We'll keep the path as-is for documentation purposes.

	return method, path
}

// isHTTPMethod returns true if s is a valid HTTP method.
func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// looksLikeHTTPPattern returns true if s looks like an HTTP route pattern.
// Matches "METHOD /path" (e.g. "GET /users") or a bare path starting with "/".
func looksLikeHTTPPattern(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "/") {
		return true
	}
	parts := strings.SplitN(s, " ", 2)
	return len(parts) == 2 && isHTTPMethod(parts[0]) && strings.HasPrefix(strings.TrimSpace(parts[1]), "/")
}

// unwrapMiddleware unwraps function call wrapping like authMiddleware(handler),
// collecting outer wrappers as middleware expressions.
// Returns the innermost handler and collected middleware.
func unwrapMiddleware(expr ast.Expr) (ast.Expr, []ast.Expr) {
	var middlewares []ast.Expr
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			break
		}
		// If the call has exactly one argument and the function is an identifier
		// or selector (not a method call on an object), treat it as middleware wrapping.
		if len(call.Args) != 1 {
			break
		}
		// The wrapper function itself is middleware.
		middlewares = append(middlewares, call.Fun)
		expr = call.Args[0]
	}
	return expr, middlewares
}
