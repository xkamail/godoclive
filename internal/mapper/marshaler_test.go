package mapper_test

import (
	"testing"

	"github.com/xkamail/godoclive/internal/model"
)

func TestMarshaler_MapLiteralStruct(t *testing.T) {
	_, td := loadType(t, testdataDir("marshaler"), "Paginate")

	if td.Kind != model.KindStruct {
		t.Fatalf("expected KindStruct from MarshalJSON map literal, got %s", td.Kind)
	}

	want := map[string]string{
		"page":       "uint64",
		"limit":      "uint64",
		"totalItems": "int64",
		"totalPages": "int64",
		"firstPage":  "bool",
		"lastPage":   "bool",
	}
	if len(td.Fields) != len(want) {
		t.Fatalf("expected %d fields, got %d", len(want), len(td.Fields))
	}
	for name, typ := range want {
		f := findField(td, name)
		if f == nil {
			t.Errorf("field %q not found", name)
			continue
		}
		if f.Type.Kind != model.KindPrimitive || f.Type.Name != typ {
			t.Errorf("field %q: expected primitive %q, got %s %q", name, typ, f.Type.Kind, f.Type.Name)
		}
	}
}

func TestMarshaler_StringerEnum(t *testing.T) {
	_, td := loadType(t, testdataDir("marshaler"), "Status")

	if td.Kind != model.KindPrimitive || td.Name != "string" {
		t.Fatalf("expected primitive string from stringer enum, got %s %q", td.Kind, td.Name)
	}
	want := []string{"not_paid", "in_progress", "success"}
	if len(td.Enum) != len(want) {
		t.Fatalf("expected enum %v, got %v", want, td.Enum)
	}
	for i, v := range want {
		if td.Enum[i] != v {
			t.Errorf("enum[%d]: expected %q, got %q", i, v, td.Enum[i])
		}
	}
	if td.Example != "not_paid" {
		t.Errorf("expected example %q, got %v", "not_paid", td.Example)
	}
}

func TestMarshaler_UnanalyzableFallsBackToObject(t *testing.T) {
	_, td := loadType(t, testdataDir("marshaler"), "Opaque")

	if td.Kind != model.KindInterface {
		t.Fatalf("expected KindInterface fallback, got %s", td.Kind)
	}
	if findField(td, "secret") != nil {
		t.Error("internal field 'secret' must not leak into the schema")
	}
}

func TestMarshaler_SliceExampleHonorsEnum(t *testing.T) {
	_, td := loadType(t, testdataDir("marshaler"), "ListResult")

	items := findField(td, "items")
	if items == nil {
		t.Fatal("field 'items' not found")
	}
	arr, ok := items.Example.([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("expected 1-element array example, got %#v", items.Example)
	}
	obj, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object element, got %#v", arr[0])
	}
	if obj["status"] != "not_paid" {
		t.Errorf("nested enum example: expected %q, got %#v", "not_paid", obj["status"])
	}
}

func TestMarshaler_EnumFieldInStruct(t *testing.T) {
	_, td := loadType(t, testdataDir("marshaler"), "OrderResult")

	status := findField(td, "status")
	if status == nil {
		t.Fatal("field 'status' not found")
	}
	if status.Type.Kind != model.KindPrimitive || status.Type.Name != "string" {
		t.Fatalf("status field: expected primitive string, got %s %q", status.Type.Kind, status.Type.Name)
	}
	if len(status.Type.Enum) != 3 {
		t.Errorf("status field: expected 3 enum values, got %v", status.Type.Enum)
	}
	if status.Example != "not_paid" {
		t.Errorf("status field: expected example %q, got %v", "not_paid", status.Example)
	}

	page := findField(td, "page")
	if page == nil {
		t.Fatal("field 'page' not found")
	}
	if page.Type.Kind != model.KindStruct || findField(&page.Type, "totalItems") == nil {
		t.Errorf("page field: expected derived Paginate struct with totalItems, got %s", page.Type.Kind)
	}
}
