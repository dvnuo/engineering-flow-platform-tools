package tests

import (
	"encoding/json"
	"testing"
)

func TestLogSchemaAnalyzeMatchesRealFlags(t *testing.T) {
	_, obj := runLog(t, "schema", "analyze", "--json")
	data := obj["data"].(map[string]any)
	requireFlags(t, data, "source", "run", "format-hint", "max-bytes", "max-line-bytes", "json", "format", "verbose")
	requireRequired(t, data, "source", "run")
	flags := schemaFlagMap(data)
	for _, name := range []string{"source", "run", "format-hint", "max-bytes", "max-line-bytes"} {
		flag := flags[name]
		if flag == nil {
			b, _ := json.Marshal(data)
			t.Fatalf("missing %s in %s", name, string(b))
		}
		if flag["description"] == "" {
			t.Fatalf("flag %s missing description", name)
		}
	}
}

func TestLogSchemaSearchRequiredMatchesRuntimeValidation(t *testing.T) {
	_, obj := runLog(t, "schema", "search", "--json")
	data := obj["data"].(map[string]any)
	requireFlags(t, data, "run", "query", "regex", "level", "template-id", "since", "until", "limit", "cursor", "json", "format", "verbose")
	requireRequired(t, data, "run", "query")

	_, missingRun := runLog(t, "search", "--query", "timeout", "--json")
	if missingRun["ok"] != false {
		t.Fatalf("expected missing run to fail: %#v", missingRun)
	}
	errObj := missingRun["error"].(map[string]any)
	if errObj["code"] != "invalid_args" {
		t.Fatalf("expected invalid_args, got %#v", errObj)
	}
}
