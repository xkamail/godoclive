package mapper

import (
	"go/types"
	"strings"

	"github.com/xkamail/godoclive/internal/model"
)

// nameHints maps common JSON field names to realistic example values.
var nameHints = map[string]interface{}{
	"id":           "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"uuid":         "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"email":        "user@example.com",
	"name":         "John Doe",
	"first_name":   "John",
	"last_name":    "Doe",
	"username":     "johndoe",
	"phone":        "+1-555-0100",
	"url":          "https://example.com",
	"website":      "https://example.com",
	"avatar":       "https://example.com/avatar.png",
	"image":        "https://example.com/image.png",
	"token":        "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	"password":     "••••••••",
	"created_at":   "2024-01-15T10:30:00Z",
	"updated_at":   "2024-01-15T10:30:00Z",
	"deleted_at":   nil,
	"description":  "A brief description",
	"title":        "Example Title",
	"slug":         "example-title",
	"status":       "active",
	"role":         "user",
	"count":        42,
	"total":        100,
	"page":         1,
	"limit":        20,
	"offset":       0,
}

// suffixHints maps field name suffixes to example values.
var suffixHints = map[string]interface{}{
	"_id":  "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"_at":  "2024-01-15T10:30:00Z",
	"_url": "https://example.com",
}

// wellKnownTypeExample returns an example value for well-known named types
// (time.Time, decimal.Decimal, xid.ID). Pointers to these render as null,
// since a pointer field represents an optional/nullable value.
func wellKnownTypeExample(t types.Type) (interface{}, bool) {
	if ptr, ok := t.(*types.Pointer); ok {
		if _, ok := wellKnownNamedExample(ptr.Elem()); ok {
			return nil, true
		}
		return nil, false
	}
	return wellKnownNamedExample(t)
}

func wellKnownNamedExample(t types.Type) (interface{}, bool) {
	named, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}
	obj := named.Obj()
	if obj.Pkg() == nil {
		return nil, false
	}
	switch obj.Pkg().Path() + "." + obj.Name() {
	case "time.Time":
		return "2024-01-15T10:30:00Z", true
	case "github.com/shopspring/decimal.Decimal":
		return 123.45, true
	case "github.com/rs/xid.ID":
		return "9m4e2mr0ui3e8a215n4g", true
	}
	return nil, false
}

// fieldExample computes a field's example, composing from the mapped TypeDef for
// composite kinds (so enums and marshaler-derived shapes are honored) and falling
// back to name/type heuristics for scalars.
func fieldExample(t types.Type, jsonName string, td *model.TypeDef) interface{} {
	switch td.Kind {
	case model.KindStruct, model.KindSlice, model.KindMap, model.KindInterface:
		return exampleFromTypeDef(td)
	default:
		if len(td.Enum) > 0 {
			return td.Enum[0]
		}
		return generateExample(t, jsonName)
	}
}

// exampleFromTypeDef composes an example value from an already-mapped TypeDef,
// reusing the per-field examples computed during mapping. Unlike generateExample
// (which re-walks raw Go types and is blind to custom json.Marshaler shapes and
// stringer enums), this honors the derived schema — so a slice of enum values or
// of marshaler-derived structs renders correctly.
func exampleFromTypeDef(td *model.TypeDef) interface{} {
	switch td.Kind {
	case model.KindStruct:
		obj := make(map[string]interface{})
		for _, f := range td.Fields {
			if f.JSONName == "" || f.JSONName == "-" {
				continue
			}
			obj[f.JSONName] = f.Example
		}
		return obj
	case model.KindSlice:
		if td.Elem == nil {
			return []interface{}{}
		}
		ev := exampleFromTypeDef(td.Elem)
		if ev == nil {
			return []interface{}{}
		}
		return []interface{}{ev}
	case model.KindPrimitive:
		if len(td.Enum) > 0 {
			return td.Enum[0]
		}
		return td.Example
	case model.KindMap, model.KindInterface:
		if td.Example != nil {
			return td.Example
		}
		return map[string]interface{}{}
	default:
		return td.Example
	}
}

// generateExample produces a realistic example value for a field based on
// its JSON name (via heuristics) and then falls back to the Go type.
func generateExample(t types.Type, jsonName string) interface{} {
	// Well-known named types take priority so they render correctly
	// regardless of field name (and pointers to them render as null).
	if v, ok := wellKnownTypeExample(t); ok {
		return v
	}

	lower := strings.ToLower(jsonName)

	// Direct name match.
	if v, ok := nameHints[lower]; ok {
		return v
	}

	// Suffix-based match.
	for suffix, val := range suffixHints {
		if strings.HasSuffix(lower, suffix) {
			return val
		}
	}

	// Type fallback.
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			return "string"
		case u.Info()&types.IsInteger != 0:
			return 0
		case u.Info()&types.IsFloat != 0:
			return 0.0
		case u.Info()&types.IsBoolean != 0:
			return false
		}
	case *types.Slice:
		// For slices of structs/named types, generate a one-element example.
		elem := u.Elem()
		if ptr, ok := elem.(*types.Pointer); ok {
			elem = ptr.Elem()
		}
		if named, ok := elem.(*types.Named); ok {
			if st, ok := named.Underlying().(*types.Struct); ok {
				obj := make(map[string]interface{})
				for i := 0; i < st.NumFields(); i++ {
					field := st.Field(i)
					if !field.Exported() {
						continue
					}
					tag := st.Tag(i)
					jsonName := field.Name()
					if idx := strings.Index(tag, `json:"`); idx >= 0 {
						rest := tag[idx+6:]
						if end := strings.Index(rest, `"`); end >= 0 {
							name := rest[:end]
							if comma := strings.Index(name, ","); comma >= 0 {
								name = name[:comma]
							}
							if name != "-" && name != "" {
								jsonName = name
							}
						}
					}
					obj[jsonName] = generateExample(field.Type(), jsonName)
				}
				if len(obj) > 0 {
					return []interface{}{obj}
				}
			}
		}
		return []interface{}{}
	case *types.Map:
		return map[string]interface{}{}
	case *types.Pointer:
		return generateExample(u.Elem(), jsonName)
	}
	return nil
}
