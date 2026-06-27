package openapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/xkamail/godoclive/internal/openapi"
	"github.com/xkamail/godoclive/internal/pipeline"
)

func testdataDir(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestIntegration_ChiBasic(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if len(eps) == 0 {
		t.Fatal("expected endpoints from chi-basic")
	}

	doc := openapi.Generate(eps, openapi.Config{
		Title:   "Chi Basic API",
		Version: "1.0.0",
	})

	if doc.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %s", doc.OpenAPI)
	}
	if len(doc.Paths) == 0 {
		t.Error("expected paths in document")
	}

	// Verify JSON output is valid.
	data, err := openapi.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Check structure.
	if _, ok := parsed["paths"]; !ok {
		t.Error("expected 'paths' in JSON output")
	}
	if _, ok := parsed["info"]; !ok {
		t.Error("expected 'info' in JSON output")
	}
}

func TestIntegration_GinBasic(t *testing.T) {
	dir := testdataDir("gin-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if len(eps) == 0 {
		t.Fatal("expected endpoints from gin-basic")
	}

	doc := openapi.Generate(eps, openapi.Config{
		Title:   "Gin Basic API",
		Version: "1.0.0",
	})

	if len(doc.Paths) == 0 {
		t.Error("expected paths")
	}

	data, err := openapi.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestIntegration_StdlibBasic(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if len(eps) == 0 {
		t.Fatal("expected endpoints from stdlib-basic")
	}

	doc := openapi.Generate(eps, openapi.Config{
		Title:   "Stdlib Basic API",
		Version: "1.0.0",
	})

	if len(doc.Paths) == 0 {
		t.Error("expected paths")
	}

	data, err := openapi.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

// TestIntegration_ArpcDuplicateResultNames verifies that two arpc handlers
// whose result types share a short name (game.ListResult and user.ListResult,
// each wrapping a distinct ListItem) get distinct envelope schemas instead of
// collapsing into one. Regression test for the arpc envelope dropping the
// result type's package, which made both endpoints $ref the same component and
// reflect the wrong fields.
func TestIntegration_ArpcDuplicateResultNames(t *testing.T) {
	dir := testdataDir("stdlib-arpc")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	doc := openapi.Generate(eps, openapi.Config{Title: "Arpc", Version: "1.0.0"})
	schemas := doc.Components.Schemas

	// itemPropsFor walks an endpoint's 200 envelope → result → items array
	// element and returns its property names.
	itemPropsFor := func(path string) []string {
		t.Helper()
		op := doc.Paths[path].Post
		if op == nil {
			t.Fatalf("no POST operation for %s", path)
		}
		resolve := func(ref string) *openapi.Schema {
			s := schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
			if s == nil {
				t.Fatalf("unresolved ref %s", ref)
			}
			return s
		}
		env := resolve(op.Responses["200"].Content["application/json"].Schema.Ref)
		result := resolve(env.Properties["result"].Ref)
		items := result.Properties["items"]
		elem := resolve(items.Items.Ref)
		props := make([]string, 0, len(elem.Properties))
		for name := range elem.Properties {
			props = append(props, name)
		}
		return props
	}

	gameProps := itemPropsFor("/game.list")
	userProps := itemPropsFor("/user.list")

	// game.ListItem has slug/category/priority/hidden; user.ListItem has
	// email/username. The two must not be confused.
	assertHas := func(label string, props []string, want string) {
		for _, p := range props {
			if p == want {
				return
			}
		}
		t.Errorf("%s items expected field %q, got %v", label, want, props)
	}
	assertHas("game.list", gameProps, "slug")
	assertHas("user.list", userProps, "email")

	for _, p := range gameProps {
		if p == "email" {
			t.Errorf("game.list items leaked user.ListItem fields: %v", gameProps)
		}
	}
}

func TestIntegration_WriteJSON(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	doc := openapi.Generate(eps, openapi.Config{Title: "Write Test", Version: "1.0.0"})

	tmpFile := filepath.Join(t.TempDir(), "openapi.json")
	err = openapi.Write(doc, openapi.WriteConfig{
		OutputPath: tmpFile,
		Format:     openapi.FormatJSON,
		Indent:     true,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(data), `"openapi": "3.1.0"`) {
		t.Error("expected openapi version in output file")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON in file: %v", err)
	}
}

func TestIntegration_WriteYAML(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	doc := openapi.Generate(eps, openapi.Config{Title: "YAML Test", Version: "1.0.0"})

	tmpFile := filepath.Join(t.TempDir(), "openapi.yaml")
	err = openapi.Write(doc, openapi.WriteConfig{
		OutputPath: tmpFile,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "openapi: \"3.1.0\"") && !strings.Contains(content, "openapi: 3.1.0") {
		t.Error("expected openapi version in YAML output")
	}
	if !strings.Contains(content, "title: YAML Test") {
		t.Error("expected title in YAML output")
	}
}
