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
