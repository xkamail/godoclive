package mapper

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/xkamail/godoclive/internal/model"
	"golang.org/x/tools/go/packages"
)

// marshalerShape derives a model.TypeDef from a named type's custom MarshalJSON
// method by statically analyzing the method body, so the documented schema
// reflects the real JSON rather than the type's internal Go fields (which are
// often unexported and unrelated). It recognizes the common patterns:
//
//	return json.Marshal(map[string]T{ "key": val, ... })  → object with those keys
//	return json.Marshal(s.String())                        → string (+ enum values)
//	return json.Marshal(structValue)                       → that struct's fields
//
// Anything it cannot analyze falls back to a generic JSON object. The bool
// result is false only when the type has no custom MarshalJSON at all, in which
// case the caller maps it normally.
func marshalerShape(named *types.Named, pkgs []*packages.Package, visited map[*types.Named]bool) (model.TypeDef, bool) {
	fn := findMarshalJSON(named)
	if fn == nil {
		return model.TypeDef{}, false
	}

	// Generic object: the safe fallback when the body is too complex to read.
	fallback := model.TypeDef{Kind: model.KindInterface, Name: "object", Example: map[string]interface{}{}}

	decl, pkg := findMethodDecl(fn, pkgs)
	if decl == nil || pkg == nil || pkg.TypesInfo == nil {
		return fallback, true
	}

	arg := findMarshalArg(decl.Body, pkg.TypesInfo)
	if arg == nil {
		return fallback, true
	}

	def, ok := shapeFromMarshalArg(arg, decl.Body, named, pkg, pkgs, visited)
	if !ok {
		return fallback, true
	}
	return def, true
}

// findMarshalJSON returns the type's MarshalJSON method (value or pointer
// receiver) if it has the json.Marshaler signature func() ([]byte, error).
func findMarshalJSON(named *types.Named) *types.Func {
	for _, t := range []types.Type{named, types.NewPointer(named)} {
		ms := types.NewMethodSet(t)
		for i := 0; i < ms.Len(); i++ {
			fn, ok := ms.At(i).Obj().(*types.Func)
			if !ok || fn.Name() != "MarshalJSON" {
				continue
			}
			if isMarshalJSONSig(fn) {
				return fn
			}
		}
	}
	return nil
}

// isMarshalJSONSig reports whether fn has signature func() ([]byte, error).
func isMarshalJSONSig(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 2 {
		return false
	}
	b, ok := sig.Results().At(0).Type().(*types.Slice)
	if !ok {
		return false
	}
	if basic, ok := b.Elem().(*types.Basic); !ok || basic.Kind() != types.Byte {
		return false
	}
	named, ok := sig.Results().At(1).Type().(*types.Named)
	return ok && named.Obj().Name() == "error"
}

// findMethodDecl locates the *ast.FuncDecl for a method, searching only the
// package that defines it, and returns that package for TypesInfo lookups.
func findMethodDecl(fn *types.Func, pkgs []*packages.Package) (*ast.FuncDecl, *packages.Package) {
	fnPkg := fn.Pkg()
	pos := fn.Pos()
	var (
		decl *ast.FuncDecl
		out  *packages.Package
	)
	packages.Visit(pkgs, func(p *packages.Package) bool {
		if decl != nil {
			return false
		}
		if fnPkg != nil && p.Types != fnPkg {
			return true
		}
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if ok && fd.Name != nil && fd.Name.Pos() == pos {
					decl, out = fd, p
					return false
				}
			}
		}
		return true
	}, nil)
	return decl, out
}

// findMarshalArg returns the single argument passed to a json.Marshal(...) call
// inside the method body, or nil if no such call is found.
func findMarshalArg(body *ast.BlockStmt, info *types.Info) ast.Expr {
	var arg ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		if arg != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Marshal" {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		pn, ok := info.Uses[x].(*types.PkgName)
		if !ok || pn.Imported().Path() != "encoding/json" {
			return true
		}
		arg = call.Args[0]
		return false
	})
	return arg
}

// shapeFromMarshalArg maps the expression handed to json.Marshal into a TypeDef.
func shapeFromMarshalArg(expr ast.Expr, body *ast.BlockStmt, named *types.Named, pkg *packages.Package, pkgs []*packages.Package, visited map[*types.Named]bool) (model.TypeDef, bool) {
	info := pkg.TypesInfo

	// json.Marshal(&x) marshals the same shape as x.
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = u.X
	}

	// Inline map literal: keys become object fields.
	if cl, ok := expr.(*ast.CompositeLit); ok {
		if isMapType(info.TypeOf(cl)) {
			return shapeFromMapLit(cl, named, pkg, pkgs, visited)
		}
	}

	// Identifier holding a map literal (e.g. m := map[...]{...}; json.Marshal(m)).
	if id, ok := expr.(*ast.Ident); ok && isMapType(info.TypeOf(id)) {
		if lit := findAssignedMapLit(body, id.Name); lit != nil {
			return shapeFromMapLit(lit, named, pkg, pkgs, visited)
		}
	}

	// Everything else: map the Go type of the marshaled value. This handles
	// json.Marshal(s.String()) → string, json.Marshal(structValue) → fields,
	// json.Marshal(alias) where alias has a different underlying type, etc.
	t := info.TypeOf(expr)
	if t == nil {
		return model.TypeDef{}, false
	}
	def := mapTypeWithPkgs(t, pkg, pkgs, visited)

	// A stringer-style enum (named integer marshaled via its String()) renders
	// as a string with the const line-comment values as its enum set.
	if def.Kind == model.KindPrimitive && def.Name == "string" {
		if vals := enumValues(named, pkgs); len(vals) > 0 {
			def.Enum = vals
			def.Example = vals[0]
		}
	}
	return def, true
}

// shapeFromMapLit builds a struct TypeDef from a map[string]T literal, using the
// static type of each value expression for the field type and example.
func shapeFromMapLit(lit *ast.CompositeLit, named *types.Named, pkg *packages.Package, pkgs []*packages.Package, visited map[*types.Named]bool) (model.TypeDef, bool) {
	info := pkg.TypesInfo
	def := model.TypeDef{Kind: model.KindStruct}
	if named.Obj() != nil {
		def.Name = named.Obj().Name()
		if named.Obj().Pkg() != nil {
			def.Package = named.Obj().Pkg().Path()
		}
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return model.TypeDef{}, false
		}
		key, ok := stringLitValue(kv.Key)
		if !ok {
			return model.TypeDef{}, false
		}
		fd := model.FieldDef{Name: key, JSONName: key}
		if vt := info.TypeOf(kv.Value); vt != nil {
			fd.Type = mapTypeWithPkgs(vt, pkg, pkgs, visited)
			fd.Example = generateExample(vt, key)
		} else {
			fd.Type = model.TypeDef{Kind: model.KindInterface, Name: "interface{}"}
		}
		def.Fields = append(def.Fields, fd)
	}
	return def, true
}

// enumValues returns the JSON string values of a stringer-style enum, taken from
// the line comments on its const declarations (the stringer -linecomment
// convention, e.g. `StatusNotPaid Status = iota // not_paid`). It returns nil
// when the type has no such line-commented consts, so callers only attach an
// enum when the values are known reliably.
func enumValues(named *types.Named, pkgs []*packages.Package) []string {
	pkg := findPackageForGoType(named, pkgs)
	if pkg == nil || pkg.TypesInfo == nil {
		return nil
	}
	var vals []string
	for _, f := range pkg.Syntax {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || vs.Comment == nil {
					continue
				}
				c, ok := pkg.TypesInfo.Defs[vs.Names[0]].(*types.Const)
				if !ok || !types.Identical(c.Type(), named) {
					continue
				}
				if cmt := strings.TrimSpace(vs.Comment.Text()); cmt != "" {
					vals = append(vals, cmt)
				}
			}
		}
	}
	return vals
}

// findAssignedMapLit finds the map composite literal assigned to a local
// variable of the given name within body.
func findAssignedMapLit(body *ast.BlockStmt, name string) *ast.CompositeLit {
	var lit *ast.CompositeLit
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name != name || i >= len(as.Rhs) {
				continue
			}
			if cl, ok := as.Rhs[i].(*ast.CompositeLit); ok {
				lit = cl
			}
		}
		return true
	})
	return lit
}

// stringLitValue returns the string value of a string-literal expression.
func stringLitValue(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// isMapType reports whether t's underlying type is a map.
func isMapType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Map)
	return ok
}
