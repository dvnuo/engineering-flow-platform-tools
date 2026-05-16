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

func TestBulkCreateRequiresConfirmMappingForActualCreate(t *testing.T) {
	postIssue := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/rest/api/2/issue" {
			postIssue++
			w.Write([]byte(`{"key":"QA-200"}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	mappingPath := filepath.Join(dir, "mapping.json")
	if err := os.WriteFile(csvPath, []byte("Title\nLogin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mapping := `{"version":1,"mode":"jira_csv_bulk_create","jira":{"project":"QA","issuetype":"Test"},"field_mappings":[{"csv_column":"Title","jira_field_id":"summary","jira_field_name":"Summary","required":true,"phase":"create","transform":"string","confidence":0.8}],"required_fields":[{"jira_field_id":"summary","jira_field_name":"Summary"}],"requires_confirmation":[{"csv_column":"Title","jira_field_id":"summary","reason":"mapping confidence below 0.90"}]}`
	if err := os.WriteFile(mappingPath, []byte(mapping), 0o600); err != nil {
		t.Fatal(err)
	}

	out := run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath)
	if out["ok"].(bool) {
		t.Fatalf("bulk-create should require --confirm-mapping: %#v", out)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "mapping_requires_confirmation" || !strings.Contains(errObj["message"].(string), "1") {
		t.Fatalf("wrong mapping confirmation error: %#v", out)
	}
	if !strings.Contains(errObj["hint"].(string), "--confirm-mapping") {
		t.Fatalf("missing confirm hint: %#v", out)
	}
	if postIssue != 0 {
		t.Fatalf("unexpected POST before confirmation: %d", postIssue)
	}

	dryRun := run(t, cfg, "--dry-run", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath)
	if !dryRun["ok"].(bool) {
		t.Fatalf("dry-run should allow unconfirmed mapping: %#v", dryRun)
	}
	warnings := dryRun["data"].(map[string]interface{})["warnings"].([]interface{})
	if len(warnings) == 0 || warnings[0].(map[string]interface{})["code"].(string) != "mapping_requires_confirmation" {
		t.Fatalf("dry-run did not report mapping warning: %#v", dryRun)
	}

	out = run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath, "--confirm-mapping")
	if !out["ok"].(bool) {
		t.Fatalf("bulk-create with --confirm-mapping failed: %#v", out)
	}
	if postIssue != 1 {
		t.Fatalf("expected one POST after confirmation, got %d", postIssue)
	}
}

func TestBulkCreateBlocksAmbiguousAndMissingRequiredPlans(t *testing.T) {
	postIssue := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/rest/api/2/issue" {
			postIssue++
		}
		w.Write([]byte(`{"key":"QA-200"}`))
	})
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	if err := os.WriteFile(csvPath, []byte("Title\nLogin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ambiguousPath := filepath.Join(dir, "ambiguous.json")
	ambiguous := `{"version":1,"mode":"jira_csv_bulk_create","jira":{"project":"QA","issuetype":"Test"},"field_mappings":[{"csv_column":"Title","jira_field_id":"summary","jira_field_name":"Summary","required":true,"phase":"create","transform":"string","confidence":0.98}],"required_fields":[{"jira_field_id":"summary","jira_field_name":"Summary"}],"ambiguous_columns":[{"csv_column":"Area","candidates":[{"jira_field_id":"components","jira_field_name":"Components","confidence":0.7}]}]}`
	if err := os.WriteFile(ambiguousPath, []byte(ambiguous), 0o600); err != nil {
		t.Fatal(err)
	}
	out := run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", ambiguousPath)
	if out["ok"].(bool) || out["error"].(map[string]interface{})["code"].(string) != "mapping_ambiguous" {
		t.Fatalf("ambiguous mapping should fail: %#v", out)
	}

	missingRequiredPath := filepath.Join(dir, "missing-required.json")
	missingRequired := `{"version":1,"mode":"jira_csv_bulk_create","jira":{"project":"QA","issuetype":"Test"},"field_mappings":[{"csv_column":"Title","jira_field_id":"summary","jira_field_name":"Summary","required":true,"phase":"create","transform":"string","confidence":0.98}],"required_fields":[{"jira_field_id":"summary","jira_field_name":"Summary"}],"warnings":[{"code":"required_field_missing","message":"customfield_1 is required"}]}`
	if err := os.WriteFile(missingRequiredPath, []byte(missingRequired), 0o600); err != nil {
		t.Fatal(err)
	}
	out = run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", missingRequiredPath, "--confirm-mapping")
	if out["ok"].(bool) || out["error"].(map[string]interface{})["code"].(string) != "required_field_missing" {
		t.Fatalf("missing required field should fail even with confirmation: %#v", out)
	}
	if postIssue != 0 {
		t.Fatalf("blocked plans should not POST, got %d", postIssue)
	}
}

func TestBulkCreateDryRunIncludesPlannedPostCreateUpdatesWithoutPut(t *testing.T) {
	putIssue := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			putIssue++
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	csvPath, mappingPath := writePostCreateFixture(t)
	out := run(t, cfg, "--dry-run", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath)
	if !out["ok"].(bool) {
		t.Fatalf("dry-run failed: %#v", out)
	}
	if putIssue != 0 {
		t.Fatalf("dry-run called PUT %d times", putIssue)
	}
	data := out["data"].(map[string]interface{})
	planned := data["planned_post_create_updates"].([]interface{})
	if len(planned) != 1 {
		t.Fatalf("planned updates = %#v", data)
	}
	payload := planned[0].(map[string]interface{})["payload"].(map[string]interface{})
	fields := payload["fields"].(map[string]interface{})
	if fields["customfield_20000"].(map[string]interface{})["name"].(string) != "alice" {
		t.Fatalf("wrong planned payload: %#v", planned[0])
	}
	warnings := data["warnings"].([]interface{})
	if warnings[0].(map[string]interface{})["code"].(string) != "post_create_updates_planned_not_applied" {
		t.Fatalf("wrong warning: %#v", warnings)
	}
}

func TestBulkCreatePostCreateUpdatesAreOptIn(t *testing.T) {
	postIssue := 0
	putIssue := 0
	var putPath string
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/rest/api/2/issue":
			postIssue++
			w.Write([]byte(`{"key":"QA-200"}`))
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/"):
			putIssue++
			putPath = r.URL.Path
			w.Write([]byte(`{"ok":true}`))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	})
	csvPath, mappingPath := writePostCreateFixture(t)

	out := run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath)
	if !out["ok"].(bool) {
		t.Fatalf("bulk-create failed: %#v", out)
	}
	if postIssue != 1 || putIssue != 0 {
		t.Fatalf("without apply flag got POST=%d PUT=%d", postIssue, putIssue)
	}
	warnings := out["data"].(map[string]interface{})["warnings"].([]interface{})
	if warnings[0].(map[string]interface{})["code"].(string) != "post_create_updates_planned_not_applied" {
		t.Fatalf("missing not-applied warning: %#v", out)
	}

	out = run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath, "--apply-post-create-updates")
	if !out["ok"].(bool) {
		t.Fatalf("bulk-create with post updates failed: %#v", out)
	}
	if postIssue != 2 || putIssue != 1 || putPath != "/rest/api/2/issue/QA-200" {
		t.Fatalf("with apply flag got POST=%d PUT=%d path=%s", postIssue, putIssue, putPath)
	}
	created := out["data"].(map[string]interface{})["created"].([]interface{})[0].(map[string]interface{})
	if created["post_create_update_status"].(string) != "applied" {
		t.Fatalf("wrong created status: %#v", created)
	}
}

func TestBulkCreatePostCreateUpdateFailureIsReportedWithoutDelete(t *testing.T) {
	deleteIssue := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/rest/api/2/issue":
			w.Write([]byte(`{"key":"QA-200"}`))
		case r.Method == "PUT" && r.URL.Path == "/rest/api/2/issue/QA-200":
			w.WriteHeader(500)
			w.Write([]byte(`{"error":"update failed"}`))
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/"):
			deleteIssue++
			w.Write([]byte(`{"ok":true}`))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	})
	csvPath, mappingPath := writePostCreateFixture(t)
	out := run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath, "--apply-post-create-updates")
	if !out["ok"].(bool) {
		t.Fatalf("bulk-create should report update failure in success envelope: %#v", out)
	}
	if deleteIssue != 0 {
		t.Fatalf("created issue should not be deleted after post update failure")
	}
	created := out["data"].(map[string]interface{})["created"].([]interface{})[0].(map[string]interface{})
	if !created["created"].(bool) || created["post_create_update_status"].(string) != "failed" {
		t.Fatalf("wrong created failure status: %#v", created)
	}
	errObj := created["error"].(map[string]interface{})
	if errObj["code"].(string) != "server_error" {
		t.Fatalf("wrong update error: %#v", created)
	}
}

func writePostCreateFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	mappingPath := filepath.Join(dir, "mapping.json")
	if err := os.WriteFile(csvPath, []byte("Title,Reviewer\nLogin,alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mapping := `{"version":1,"mode":"jira_csv_bulk_create","jira":{"project":"QA","issuetype":"Test"},"field_mappings":[{"csv_column":"Title","jira_field_id":"summary","jira_field_name":"Summary","required":true,"phase":"create","transform":"string","confidence":0.98},{"csv_column":"Reviewer","jira_field_id":"customfield_20000","jira_field_name":"Reviewer","phase":"post_create_update","transform":"user","confidence":0.98}],"required_fields":[{"jira_field_id":"summary","jira_field_name":"Summary"}]}`
	if err := os.WriteFile(mappingPath, []byte(mapping), 0o600); err != nil {
		t.Fatal(err)
	}
	return csvPath, mappingPath
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
