package commands

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBulkCreateDryRunDoesNotPostIssue(t *testing.T) {
	postIssue := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/rest/api/2/issue":
			postIssue++
			w.Write([]byte(`{"key":"QA-999"}`))
		case r.URL.Path == "/rest/api/2/field":
			w.Write([]byte(`[{"id":"summary","name":"Summary"},{"id":"customfield_10555","name":"Test Type"}]`))
		case r.URL.Path == "/rest/api/2/issue/QA-1234":
			w.Write([]byte(`{"key":"QA-1234","fields":{"project":{"key":"QA","id":"10000"},"issuetype":{"id":"10001","name":"Test"},"summary":"Template"},"names":{"customfield_10555":"Test Type"}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes":
			w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Test"}]}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes/10001":
			w.Write([]byte(`{"fields":{"summary":{"name":"Summary","required":true,"schema":{"type":"string"}},"customfield_10555":{"name":"Test Type","schema":{"type":"option"},"allowedValues":[{"id":"1","value":"Regression"}]}}}`))
		case r.URL.Path == "/rest/api/2/issue/QA-1234/editmeta":
			w.Write([]byte(`{"fields":{}}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	csvPath := filepath.Join(t.TempDir(), "testcases.csv")
	if err := os.WriteFile(csvPath, []byte("Title,Type\nLogin,Regression\nBad,Smoke\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := run(t, cfg, "--dry-run", "issue", "bulk-create", "--from-csv", csvPath, "--template-issue", "QA-1234")
	if !out["ok"].(bool) {
		t.Fatalf("dry-run failed: %#v", out)
	}
	if postIssue != 0 {
		t.Fatalf("dry-run posted %d issues", postIssue)
	}
	data := out["data"].(map[string]interface{})
	if data["valid_rows"].(float64) != 1 || data["invalid_rows"].(float64) != 1 {
		t.Fatalf("unexpected dry-run counts: %#v", data)
	}
	if len(data["preview_payloads"].([]interface{})) != 1 || len(data["errors"].([]interface{})) != 1 {
		t.Fatalf("missing preview/errors: %#v", data)
	}
	errRow := data["errors"].([]interface{})[0].(map[string]interface{})
	if errRow["row_number"].(float64) != 3 || errRow["code"].(string) != "invalid_option" {
		t.Fatalf("unexpected row error: %#v", errRow)
	}
}

func TestBulkCreateRequiresConfirmation(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	mappingPath := filepath.Join(dir, "mapping.json")
	if err := os.WriteFile(csvPath, []byte("Title\nLogin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mappingPath, []byte(`{"version":1,"mode":"jira_csv_bulk_create"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := run(t, cfg, "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath)
	if out["ok"].(bool) {
		t.Fatalf("bulk-create without --dry-run/--yes should fail: %#v", out)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "confirmation_required" {
		t.Fatalf("wrong error: %#v", out)
	}
}

func TestCreateMetaFromIssueSplitAndLegacyFallback(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/QA-1234":
			w.Write([]byte(`{"fields":{"project":{"key":"QA","id":"10000"},"issuetype":{"id":"10001","name":"Test"}}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes":
			w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Test"}]}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes/10001":
			w.Write([]byte(`{"fields":{"summary":{"name":"Summary","required":true}}}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	out := run(t, cfg, "issue", "createmeta", "--from-issue", "QA-1234")
	if !out["ok"].(bool) {
		t.Fatalf("split createmeta failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	if data["source"].(string) != "split" {
		t.Fatalf("wrong source: %#v", data)
	}

	legacyCfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/QA-1234":
			w.Write([]byte(`{"fields":{"project":{"key":"QA","id":"10000"},"issuetype":{"id":"10001","name":"Test"}}}`))
		case strings.Contains(r.URL.Path, "/issue/createmeta/10000/issuetypes"):
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			w.Write([]byte(`{"projects":[{"key":"QA","id":"10000","issuetypes":[{"id":"10001","name":"Test","fields":{"summary":{"name":"Summary"}}}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	out = run(t, legacyCfg, "issue", "createmeta", "--from-issue", "QA-1234")
	if !out["ok"].(bool) {
		t.Fatalf("legacy createmeta failed: %#v", out)
	}
	data = out["data"].(map[string]interface{})
	if data["source"].(string) != "legacy" {
		t.Fatalf("wrong fallback source: %#v", data)
	}
}
