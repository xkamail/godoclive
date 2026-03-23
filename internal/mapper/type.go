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
