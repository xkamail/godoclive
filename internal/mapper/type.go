package mapper

import (
	"go/types"

	"github.com/xkamail/godoclive/internal/model"
	"golang.org/x/tools/go/packages"
)

// MapType converts a go/types.Type into a model.TypeDef, recursively mapping
// struct fields, slices, maps, pointers, and primitives. It handles cycles
// by tracking visited *types.Named by identity.
func MapType(t types.Type, pkg *packages.Package) model.TypeDef {
	visited := make(map[*types.Named]bool)
	return mapType(t, pkg, visited)
}

// MapTypeWithPkgs is like MapType but accepts all loaded packages so it can
// resolve AST-level details (doc comments) for types in other packages.
func MapTypeWithPkgs(t types.Type, pkgs []*packages.Package) model.TypeDef {
	visited := make(map[*types.Named]bool)
	// Find the best matching package for the root type.
	pkg := findPackageForGoType(t, pkgs)
	return mapTypeWithPkgs(t, pkg, pkgs, visited)
}

// findPackageForGoType returns the *packages.Package that defines the given type.
func findPackageForGoType(t types.Type, pkgs []*packages.Package) *packages.Package {
	named, ok := t.(*types.Named)
	if !ok {
		if ptr, ok := t.(*types.Pointer); ok {
			named, _ = ptr.Elem().(*types.Named)
		}
	}
	if named == nil || named.Obj().Pkg() == nil {
		if len(pkgs) > 0 {
			return pkgs[0]
		}
		return nil
	}
	typePkg := named.Obj().Pkg()
	var result *packages.Package
	packages.Visit(pkgs, func(p *packages.Package) bool {
		if p.Types == typePkg {
			result = p
			return false
		}
		return result == nil
	}, nil)
	if result != nil {
		return result
	}
	if len(pkgs) > 0 {
		return pkgs[0]
	}
	return nil
}

// wellKnownType returns a TypeDef for types that should be treated as
// primitives rather than being recursively mapped (e.g. time.Time, decimal.Decimal).
func wellKnownType(named *types.Named) (model.TypeDef, bool) {
	obj := named.Obj()
	if obj.Pkg() == nil {
		return model.TypeDef{}, false
	}
	key := obj.Pkg().Path() + "." + obj.Name()
	switch key {
	case "time.Time":
		return model.TypeDef{
			Kind:    model.KindPrimitive,
			Name:    "string",
			Example: "2024-01-15T10:30:00Z",
		}, true
	case "github.com/shopspring/decimal.Decimal":
		return model.TypeDef{
			Kind:    model.KindPrimitive,
			Name:    "number",
			Example: "123.45",
		}, true
	case "encoding/json.RawMessage":
		return model.TypeDef{
			Kind:    model.KindInterface,
			Name:    "object",
			Example: map[string]interface{}{},
		}, true
	}
	return model.TypeDef{}, false
}

func mapType(t types.Type, pkg *packages.Package, visited map[*types.Named]bool) model.TypeDef {
	// Dereference pointer.
	if ptr, ok := t.(*types.Pointer); ok {
		def := mapType(ptr.Elem(), pkg, visited)
		def.IsPointer = true
		return def
	}

	// Check for named types to detect cycles and well-known types.
	if named, ok := t.(*types.Named); ok {
		// Well-known types that should be treated as primitives.
		if def, ok := wellKnownType(named); ok {
			return def
		}
		if visited[named] {
			return model.TypeDef{
				Kind: model.KindStruct,
				Name: named.Obj().Name(),
			}
		}
		visited[named] = true
		defer func() { delete(visited, named) }()
	}

	switch u := t.Underlying().(type) {
	case *types.Struct:
		named, _ := t.(*types.Named)
		return mapStruct(named, u, pkg, visited)
	case *types.Slice:
		elem := mapType(u.Elem(), pkg, visited)
		return model.TypeDef{Kind: model.KindSlice, Elem: &elem}
	case *types.Map:
		return model.TypeDef{Kind: model.KindMap, Name: t.String()}
	case *types.Basic:
		return model.TypeDef{Kind: model.KindPrimitive, Name: u.Name()}
	case *types.Interface:
		return model.TypeDef{Kind: model.KindInterface, Name: "interface{}"}
	default:
		return model.TypeDef{Kind: model.KindUnknown, Name: t.String()}
	}
}

func mapTypeWithPkgs(t types.Type, pkg *packages.Package, pkgs []*packages.Package, visited map[*types.Named]bool) model.TypeDef {
	if ptr, ok := t.(*types.Pointer); ok {
		def := mapTypeWithPkgs(ptr.Elem(), pkg, pkgs, visited)
		def.IsPointer = true
		return def
	}

	if named, ok := t.(*types.Named); ok {
		if def, ok := wellKnownType(named); ok {
			return def
		}
		if visited[named] {
			return model.TypeDef{
				Kind: model.KindStruct,
				Name: named.Obj().Name(),
			}
		}
		visited[named] = true
		defer func() { delete(visited, named) }()
		// Resolve the correct package for this named type.
		pkg = findPackageForGoType(named, pkgs)
	}

	switch u := t.Underlying().(type) {
	case *types.Struct:
		named, _ := t.(*types.Named)
		return mapStructWithPkgs(named, u, pkg, pkgs, visited)
	case *types.Slice:
		elem := mapTypeWithPkgs(u.Elem(), pkg, pkgs, visited)
		return model.TypeDef{Kind: model.KindSlice, Elem: &elem}
	case *types.Map:
		return model.TypeDef{Kind: model.KindMap, Name: t.String()}
	case *types.Basic:
		return model.TypeDef{Kind: model.KindPrimitive, Name: u.Name()}
	case *types.Interface:
		return model.TypeDef{Kind: model.KindInterface, Name: "interface{}"}
	default:
		return model.TypeDef{Kind: model.KindUnknown, Name: t.String()}
	}
}
