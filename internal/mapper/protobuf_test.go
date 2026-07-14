package mapper_test

import (
	"testing"

	"github.com/xkamail/godoclive/internal/model"
)

func TestProtobuf_SkipsUnexportedFields(t *testing.T) {
	_, td := loadType(t, testdataDir("protobuf"), "Ticket")

	if td.Kind != model.KindStruct {
		t.Fatalf("expected KindStruct, got %s", td.Kind)
	}

	for _, hidden := range []string{"state", "sizeCache", "unknownFields"} {
		if findField(td, hidden) != nil {
			t.Errorf("unexported field %q must not appear in the schema", hidden)
		}
	}

	want := []string{"id", "player_name", "score"}
	if len(td.Fields) != len(want) {
		t.Fatalf("expected %d exported fields, got %d", len(want), len(td.Fields))
	}
	for _, name := range want {
		if findField(td, name) == nil {
			t.Errorf("exported field %q not found", name)
		}
	}
}

func TestProtobuf_NestedMessageSkipsUnexported(t *testing.T) {
	_, td := loadType(t, testdataDir("protobuf"), "Match")

	tickets := findField(td, "tickets")
	if tickets == nil {
		t.Fatal("field 'tickets' not found")
	}
	if tickets.Type.Kind != model.KindSlice || tickets.Type.Elem == nil {
		t.Fatalf("expected slice of tickets, got %s", tickets.Type.Kind)
	}
	elem := tickets.Type.Elem
	for _, hidden := range []string{"state", "sizeCache", "unknownFields"} {
		if findField(elem, hidden) != nil {
			t.Errorf("nested unexported field %q must not appear in the schema", hidden)
		}
	}
	if findField(elem, "id") == nil {
		t.Error("nested exported field 'id' not found")
	}
}
