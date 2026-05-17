package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/httpclient"
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

func TestBulkCreateDoesNotPostWhenSummaryMissingFromCreatePayload(t *testing.T) {
	postIssue := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/rest/api/2/issue" {
			postIssue++
		}
		w.Write([]byte(`{"ok":true}`))
	})
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	mappingPath := filepath.Join(dir, "mapping.json")
	if err := os.WriteFile(csvPath, []byte("Title\nLogin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mapping := `{"version":1,"mode":"jira_csv_bulk_create","jira":{"project":"QA","issuetype":"Test"},"field_mappings":[{"csv_column":"Title","jira_field_id":"summary","jira_field_name":"Summary","required":true,"phase":"post_create_update","transform":"string","confidence":0.98}]}`
	if err := os.WriteFile(mappingPath, []byte(mapping), 0o600); err != nil {
		t.Fatal(err)
	}

	dryRun := run(t, cfg, "--dry-run", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath)
	if !dryRun["ok"].(bool) {
		t.Fatalf("dry-run should return row validation data: %#v", dryRun)
	}
	data := dryRun["data"].(map[string]interface{})
	if data["invalid_rows"].(float64) != 1 {
		t.Fatalf("summary-missing row should be invalid: %#v", data)
	}
	errRow := data["errors"].([]interface{})[0].(map[string]interface{})
	if errRow["code"].(string) != "summary_required_missing" {
		t.Fatalf("wrong row error: %#v", data)
	}

	out := run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath, "--confirm-mapping")
	if out["ok"].(bool) || out["error"].(map[string]interface{})["code"].(string) != "invalid_args" {
		t.Fatalf("actual create should fail before POST: %#v", out)
	}
	if postIssue != 0 {
		t.Fatalf("unexpected POST with missing summary: %d", postIssue)
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

func TestBulkCreateAppliesReporterPostCreateUpdate(t *testing.T) {
	postIssue := 0
	putIssue := 0
	var postPayload map[string]interface{}
	var putPayload map[string]interface{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/rest/api/2/issue":
			postIssue++
			if err := json.NewDecoder(r.Body).Decode(&postPayload); err != nil {
				t.Fatalf("decode post payload: %v", err)
			}
			w.Write([]byte(`{"key":"QA-300"}`))
		case r.Method == "PUT" && r.URL.Path == "/rest/api/2/issue/QA-300":
			putIssue++
			if err := json.NewDecoder(r.Body).Decode(&putPayload); err != nil {
				t.Fatalf("decode put payload: %v", err)
			}
			w.Write([]byte(`{"ok":true}`))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	})
	csvPath, mappingPath := writeReporterPostCreateFixture(t)

	out := run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath, "--apply-post-create-updates")
	if !out["ok"].(bool) {
		t.Fatalf("bulk-create with reporter update failed: %#v", out)
	}
	if postIssue != 1 || putIssue != 1 {
		t.Fatalf("got POST=%d PUT=%d", postIssue, putIssue)
	}
	postFields := postPayload["fields"].(map[string]interface{})
	if _, ok := postFields["reporter"]; ok {
		t.Fatalf("reporter leaked into create payload: %#v", postPayload)
	}
	putFields := putPayload["fields"].(map[string]interface{})
	reporter := putFields["reporter"].(map[string]interface{})
	if reporter["name"].(string) != "XXXXX" {
		t.Fatalf("wrong reporter update payload: %#v", putPayload)
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

func writeReporterPostCreateFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	mappingPath := filepath.Join(dir, "mapping.json")
	if err := os.WriteFile(csvPath, []byte("Title,Reporter\nLogin,XXXXX\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mapping := `{"version":1,"mode":"jira_csv_bulk_create","jira":{"project":"QA","issuetype":"Test"},"field_mappings":[{"csv_column":"Title","jira_field_id":"summary","jira_field_name":"Summary","required":true,"phase":"create","transform":"string","confidence":0.98},{"csv_column":"Reporter","jira_field_id":"reporter","jira_field_name":"Reporter","phase":"post_create_update","transform":"user","confidence":0.99}],"required_fields":[{"jira_field_id":"summary","jira_field_name":"Summary"}]}`
	if err := os.WriteFile(mappingPath, []byte(mapping), 0o600); err != nil {
		t.Fatal(err)
	}
	return csvPath, mappingPath
}

func TestCreateMetaFromIssueRetriesFullReadWhenMinimalReadReportsMissing(t *testing.T) {
	minimalRead := 0
	fullRead := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/MMGFX-13887":
			switch {
			case r.URL.Query().Get("fields") == "project,issuetype" && r.URL.Query().Get("expand") == "names,schema":
				minimalRead++
				w.WriteHeader(404)
				w.Write([]byte(`Issue Does Not Exist`))
			case r.URL.Query().Get("fields") == "*all" && r.URL.Query().Get("expand") == "names,schema,editmeta":
				fullRead++
				w.Write([]byte(`{"fields":{"project":{"key":"MMGFX","id":"10000"},"issuetype":{"id":"10001","name":"Bug"}}}`))
			default:
				w.WriteHeader(404)
				w.Write([]byte(`{"error":"unexpected issue query"}`))
			}
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			w.Write([]byte(`{"projects":[{"key":"MMGFX","id":"10000","issuetypes":[{"id":"10001","name":"Bug","fields":{"summary":{"name":"Summary"}}}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})

	out := run(t, cfg, "issue", "createmeta", "--from-issue", "MMGFX-13887", "--legacy")
	if !out["ok"].(bool) {
		t.Fatalf("legacy createmeta should succeed after full issue read retry: %#v", out)
	}
	if minimalRead != 1 || fullRead != 1 {
		t.Fatalf("expected one minimal and one full issue read, got minimal=%d full=%d", minimalRead, fullRead)
	}
	fields := out["data"].(map[string]interface{})["fields"].(map[string]interface{})
	if _, ok := fields["summary"]; !ok {
		t.Fatalf("missing summary field: %#v", out)
	}
}

func TestCreateMetaDryRunShowsFallbackFlow(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run should not call server: %s", r.URL.String())
	})

	out := run(t, cfg, "--dry-run", "issue", "createmeta", "--from-issue", "MMGFX-13887")
	if !out["ok"].(bool) {
		t.Fatalf("dry-run failed: %#v", out)
	}
	requests := out["data"].(map[string]interface{})["requests"].([]interface{})
	paths := make([]string, 0, len(requests))
	fallbackCandidates := 0
	for _, req := range requests {
		m := req.(map[string]interface{})
		paths = append(paths, m["path"].(string))
		if m["fallback_candidate"] == true {
			fallbackCandidates++
		}
	}
	wantPaths := []string{
		"issue/MMGFX-13887",
		"issue/MMGFX-13887",
		"issue/createmeta/{projectIdOrKey}/issuetypes",
		"issue/createmeta/{projectIdOrKey}/issuetypes/{issueTypeId}",
		"issue/createmeta",
	}
	if strings.Join(paths, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("unexpected dry-run flow: %#v", paths)
	}
	if fallbackCandidates != 2 {
		t.Fatalf("expected full issue read and legacy createmeta as fallback candidates, got %d", fallbackCandidates)
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

	var splitAuthHeader string
	legacyCfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/QA-1234":
			w.Write([]byte(`{"fields":{"project":{"key":"QA","id":"10000"},"issuetype":{"id":"10001","name":"Test"}}}`))
		case strings.Contains(r.URL.Path, "/issue/createmeta/10000/issuetypes"):
			splitAuthHeader = r.Header.Get("Authorization")
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"Issue Does Not Exist","authorization":"` + splitAuthHeader + `","token":"plain-secret"}`))
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
	if data["source"].(string) != "legacy_after_split_fallback_error" {
		t.Fatalf("wrong fallback source: %#v", data)
	}
	if _, ok := data["fields"].(map[string]interface{})["summary"]; !ok {
		t.Fatalf("missing fallback summary field: %#v", data)
	}
	warnings := data["warnings"].([]interface{})
	if len(warnings) != 1 || warnings[0].(map[string]interface{})["code"].(string) != "split_createmeta_fallback_error" {
		t.Fatalf("missing split fallback warning: %#v", data)
	}
	detail := warnings[0].(map[string]interface{})["detail"].(string)
	if !strings.Contains(detail, "Issue Does Not Exist") {
		t.Fatalf("missing split error detail: %#v", warnings[0])
	}
	if strings.Contains(detail, splitAuthHeader) || strings.Contains(detail, "plain-secret") {
		t.Fatalf("warning detail leaked credentials: %s", detail)
	}
}

func TestCreateMetaFallsBackToLegacyWhenSplitFieldsEmpty(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Story"}}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes":
			w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Story"}]}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes/10001":
			w.Write([]byte(`{"fields":{}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			w.Write([]byte(`{"projects":[{"key":"EX","id":"10000","issuetypes":[{"id":"10001","name":"Story","fields":{"summary":{"name":"Summary","required":true,"schema":{"type":"string"}}}}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})

	out := run(t, cfg, "issue", "createmeta", "--from-issue", "EX-1")
	if !out["ok"].(bool) {
		t.Fatalf("legacy fallback after empty split failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	if data["source"].(string) != "legacy_after_empty_split" {
		t.Fatalf("wrong source: %#v", data)
	}
	fields := data["fields"].(map[string]interface{})
	if _, ok := fields["summary"]; !ok {
		t.Fatalf("missing summary field: %#v", fields)
	}
	warnings := data["warnings"].([]interface{})
	if len(warnings) != 1 || warnings[0].(map[string]interface{})["code"].(string) != "split_createmeta_empty_fields" {
		t.Fatalf("missing empty split warning: %#v", data)
	}
}

func TestCreateMetaFailsWhenSplitAndLegacyFieldsEmpty(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Story"}}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes":
			w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Story"}]}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes/10001":
			w.Write([]byte(`{"fields":{}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			w.Write([]byte(`{"projects":[{"key":"EX","id":"10000","issuetypes":[{"id":"10001","name":"Story","fields":{}}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})

	out := run(t, cfg, "issue", "createmeta", "--from-issue", "EX-1")
	if out["ok"].(bool) {
		t.Fatalf("empty split and legacy fields should fail: %#v", out)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "createmeta_fields_empty" {
		t.Fatalf("wrong error: %#v", out)
	}
	if !strings.Contains(errObj["hint"].(string), "--legacy") {
		t.Fatalf("missing legacy hint: %#v", out)
	}
}

func TestCreateMetaFailsWhenSplitErrorFallbackLegacyFieldsEmpty(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Test"}}}`))
		case strings.Contains(r.URL.Path, "/issue/createmeta/10000/issuetypes"):
			w.WriteHeader(404)
			w.Write([]byte(`Issue Does Not Exist`))
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			w.Write([]byte(`{"projects":[{"key":"EX","id":"10000","issuetypes":[{"id":"10001","name":"Test","fields":{}}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})

	out := run(t, cfg, "issue", "createmeta", "--from-issue", "EX-1")
	if out["ok"].(bool) {
		t.Fatalf("split error fallback with empty legacy fields should fail: %#v", out)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "createmeta_fields_empty" {
		t.Fatalf("wrong error: %#v", out)
	}
	if !strings.Contains(errObj["message"].(string), "no creatable fields") {
		t.Fatalf("wrong message: %#v", out)
	}
}

func TestCreateMetaFromIssueFailsWhenMinimalAndFullIssueReadsAreMissing(t *testing.T) {
	createmetaHits := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/MMGFX-13887":
			w.WriteHeader(404)
			w.Write([]byte(`Issue Does Not Exist`))
		case strings.Contains(r.URL.Path, "/rest/api/2/issue/createmeta"):
			createmetaHits++
			w.Write([]byte(`{"projects":[]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})

	out := run(t, cfg, "issue", "createmeta", "--from-issue", "MMGFX-13887")
	if out["ok"].(bool) {
		t.Fatalf("missing from issue should fail: %#v", out)
	}
	if createmetaHits != 0 {
		t.Fatalf("createmeta should not be requested when from issue cannot be read, got %d hits", createmetaHits)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "not_found" {
		t.Fatalf("wrong error code: %#v", out)
	}
	if !strings.Contains(errObj["message"].(string), "from issue MMGFX-13887 could not be read") {
		t.Fatalf("message should identify unreadable from issue: %#v", out)
	}
}

func TestCreateMetaDoesNotFallbackForSplitPermissionDenied(t *testing.T) {
	legacyHits := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Test"}}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes":
			w.WriteHeader(403)
			w.Write([]byte(`{"error":"Forbidden"}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			legacyHits++
			w.Write([]byte(`{"projects":[{"key":"EX","id":"10000","issuetypes":[{"id":"10001","name":"Test","fields":{"summary":{"name":"Summary"}}}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})

	out := run(t, cfg, "issue", "createmeta", "--from-issue", "EX-1")
	if out["ok"].(bool) {
		t.Fatalf("permission denied split createmeta should fail: %#v", out)
	}
	if legacyHits != 0 {
		t.Fatalf("permission denied should not fall back to legacy, got %d legacy hits", legacyHits)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "permission_denied" {
		t.Fatalf("wrong error: %#v", out)
	}
}

func TestCreateMetaFallbackErrorClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "404 status", err: &httpclient.HTTPError{Code: "not_found", Message: "request failed", Status: 404}, want: true},
		{name: "not found code", err: &httpclient.HTTPError{Code: "not_found", Message: "request failed", Status: 400}, want: true},
		{name: "405 status", err: &httpclient.HTTPError{Code: "invalid_args", Message: "request failed", Status: 405}, want: true},
		{name: "501 status", err: &httpclient.HTTPError{Code: "server_error", Message: "request failed", Status: 501}, want: true},
		{name: "issue does not exist body", err: &httpclient.HTTPError{Code: "invalid_args", Message: "request failed: Issue Does Not Exist", Status: 400}, want: true},
		{name: "null for uri body", err: &httpclient.HTTPError{Code: "invalid_args", Message: "request failed: null for uri", Status: 400}, want: true},
		{name: "createmeta body", err: &httpclient.HTTPError{Code: "invalid_args", Message: "request failed: createmeta endpoint unavailable", Status: 400}, want: true},
		{name: "auth failed", err: &httpclient.HTTPError{Code: "auth_failed", Message: "Issue Does Not Exist", Status: 401}, want: false},
		{name: "permission denied", err: &httpclient.HTTPError{Code: "permission_denied", Message: "createmeta forbidden", Status: 403}, want: false},
		{name: "plain server error", err: &httpclient.HTTPError{Code: "server_error", Message: "request failed", Status: 500}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCreateMetaFallbackError(tc.err); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCreateMetaLegacyFlagFailsWhenLegacyFieldsEmpty(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Test"}}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			w.Write([]byte(`{"projects":[{"key":"EX","id":"10000","issuetypes":[{"id":"10001","name":"Test","fields":{}}]}]}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})

	out := run(t, cfg, "issue", "createmeta", "--from-issue", "EX-1", "--legacy")
	if out["ok"].(bool) {
		t.Fatalf("--legacy with empty legacy fields should fail: %#v", out)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "createmeta_fields_empty" {
		t.Fatalf("wrong error: %#v", out)
	}
}

func TestMapCSVAutoCreateMetaFallsBackWhenSplitFieldsEmpty(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/field":
			w.Write([]byte(`[{"id":"summary","name":"Summary","schema":{"type":"string"}}]`))
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Story"},"summary":"Template"},"names":{"summary":"Summary"}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes":
			w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Story"}]}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta/10000/issuetypes/10001":
			w.Write([]byte(`{"fields":{}}`))
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			w.Write([]byte(`{"projects":[{"key":"EX","id":"10000","issuetypes":[{"id":"10001","name":"Story","fields":{"summary":{"name":"Summary","required":true,"schema":{"type":"string"}}}}]}]}`))
		case r.URL.Path == "/rest/api/2/issue/EX-1/editmeta":
			w.Write([]byte(`{"fields":{}}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	csvPath := filepath.Join(t.TempDir(), "testcases.csv")
	if err := os.WriteFile(csvPath, []byte("Title\nLogin\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := run(t, cfg, "issue", "map-csv", "--from-csv", csvPath, "--template-issue", "EX-1")
	if !out["ok"].(bool) {
		t.Fatalf("map-csv legacy fallback failed: %#v", out)
	}
	plan := out["data"].(map[string]interface{})["plan"].(map[string]interface{})
	mappings := plan["field_mappings"].([]interface{})
	if len(mappings) != 1 {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
	mapping := mappings[0].(map[string]interface{})
	if mapping["jira_field_id"].(string) != "summary" || mapping["phase"].(string) != "create" {
		t.Fatalf("summary was not mapped for create: %#v", mapping)
	}
}

func TestMapCSVRejectsSuppliedCreateMetaWithEmptyFields(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/field":
			w.Write([]byte(`[{"id":"summary","name":"Summary","schema":{"type":"string"}}]`))
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Story"},"summary":"Template"},"names":{"summary":"Summary"}}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	createMetaPath := filepath.Join(dir, "create-meta.json")
	if err := os.WriteFile(csvPath, []byte("Title\nLogin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(createMetaPath, []byte(`{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Story"},"fields":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	out := run(t, cfg, "issue", "map-csv", "--from-csv", csvPath, "--template-issue", "EX-1", "--create-meta", createMetaPath)
	if out["ok"].(bool) {
		t.Fatalf("map-csv should reject empty create-meta fields: %#v", out)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "createmeta_fields_empty" {
		t.Fatalf("wrong error: %#v", out)
	}
	if errObj["hint"].(string) != "Regenerate create metadata with `jira issue createmeta --from-issue <KEY> --legacy --json`." {
		t.Fatalf("wrong hint: %#v", out)
	}
}

func TestMapCSVRejectsSummaryPostCreateOnlyMapping(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/field":
			w.Write([]byte(`[{"id":"summary","name":"Summary","schema":{"type":"string"}},{"id":"priority","name":"Priority","schema":{"type":"priority"}}]`))
		case r.URL.Path == "/rest/api/2/issue/EX-1":
			w.Write([]byte(`{"fields":{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Story"},"summary":"Template"},"names":{"summary":"Summary","priority":"Priority"}}`))
		case r.URL.Path == "/rest/api/2/issue/EX-1/editmeta":
			w.Write([]byte(`{"fields":{"summary":{"name":"Summary","schema":{"type":"string"}}}}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	createMetaPath := filepath.Join(dir, "create-meta.json")
	if err := os.WriteFile(csvPath, []byte("Title\nLogin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(createMetaPath, []byte(`{"project":{"key":"EX","id":"10000"},"issuetype":{"id":"10001","name":"Story"},"fields":{"priority":{"name":"Priority","schema":{"type":"priority"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	out := run(t, cfg, "issue", "map-csv", "--from-csv", csvPath, "--template-issue", "EX-1", "--create-meta", createMetaPath)
	if out["ok"].(bool) {
		t.Fatalf("map-csv should reject summary as post-create-only: %#v", out)
	}
	errObj := out["error"].(map[string]interface{})
	if errObj["code"].(string) != "summary_not_creatable" {
		t.Fatalf("wrong error: %#v", out)
	}
}

func TestMapCSVAutoFallsBackToEditMetaDegradedWhenCreateMetaUnavailable(t *testing.T) {
	cfg, _ := setup(t, editMetaDegradedMetadataServer(t))
	csvPath := writeEditMetaDegradedCSV(t)

	out := run(t, cfg, "issue", "map-csv", "--from-csv", csvPath, "--template-issue", "MMGFX-13887", "--metadata-mode", "auto")
	if !out["ok"].(bool) {
		t.Fatalf("map-csv auto fallback failed: %#v", out)
	}
	plan := out["data"].(map[string]interface{})["plan"].(map[string]interface{})
	if plan["metadata_mode"].(string) != "editmeta_degraded" {
		t.Fatalf("metadata mode = %#v", plan["metadata_mode"])
	}
	if !jsonWarningsContain(plan["warnings"].([]interface{}), "createmeta_unavailable_using_editmeta_degraded") {
		t.Fatalf("missing fallback warning: %#v", plan["warnings"])
	}
	mappings := plan["field_mappings"].([]interface{})
	phases := map[string]string{}
	transforms := map[string]string{}
	for _, item := range mappings {
		m := item.(map[string]interface{})
		phases[m["jira_field_id"].(string)] = m["phase"].(string)
		transforms[m["jira_field_id"].(string)] = m["transform"].(string)
	}
	if phases["summary"] != "create" || phases["priority"] != "post_create_update" || phases["components"] != "post_create_update" || phases["reporter"] != "post_create_update" || phases["customfield_26388"] != "post_create_update" {
		t.Fatalf("wrong degraded phases: %#v", phases)
	}
	if transforms["reporter"] != "user" {
		t.Fatalf("reporter transform = %s", transforms["reporter"])
	}
}

func TestMapCSVCreateMetaModeFailsWhenCreateMetaUnavailable(t *testing.T) {
	cfg, _ := setup(t, editMetaDegradedMetadataServer(t))
	csvPath := writeEditMetaDegradedCSV(t)

	out := run(t, cfg, "issue", "map-csv", "--from-csv", csvPath, "--template-issue", "MMGFX-13887", "--metadata-mode", "createmeta")
	if out["ok"].(bool) {
		t.Fatalf("strict createmeta should fail: %#v", out)
	}
	code := out["error"].(map[string]interface{})["code"].(string)
	if code != "not_found" && code != "createmeta_fields_empty" {
		t.Fatalf("wrong strict failure: %#v", out)
	}
}

func TestMapCSVEditMetaDegradedDoesNotCallCreateMeta(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/field":
			w.Write([]byte(`[{"id":"summary","name":"Summary","schema":{"type":"string"}},{"id":"priority","name":"Priority","schema":{"type":"priority"}},{"id":"components","name":"Components","schema":{"type":"array","items":"component"}},{"id":"reporter","name":"Reporter","schema":{"type":"user","system":"reporter"}},{"id":"customfield_26388","name":"Story Type","schema":{"type":"option"}}]`))
		case r.URL.Path == "/rest/api/2/issue/MMGFX-13887":
			w.Write([]byte(editMetaDegradedIssueJSON()))
		case strings.Contains(r.URL.Path, "/createmeta"):
			t.Fatalf("editmeta-degraded mode must not call createmeta: %s", r.URL.String())
		case strings.HasSuffix(r.URL.Path, "/editmeta"):
			t.Fatalf("editmeta-degraded mode should use full issue expand editmeta, got %s", r.URL.String())
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	csvPath := writeEditMetaDegradedCSV(t)

	out := run(t, cfg, "issue", "map-csv", "--from-csv", csvPath, "--template-issue", "MMGFX-13887", "--metadata-mode", "editmeta-degraded")
	if !out["ok"].(bool) {
		t.Fatalf("editmeta-degraded map-csv failed: %#v", out)
	}
	plan := out["data"].(map[string]interface{})["plan"].(map[string]interface{})
	if plan["metadata_mode"].(string) != "editmeta_degraded" {
		t.Fatalf("metadata mode = %#v", plan["metadata_mode"])
	}
}

func TestBulkCreateAutoFallbackDryRunSucceedsWithEditMeta(t *testing.T) {
	cfg, _ := setup(t, editMetaDegradedMetadataServer(t))
	csvPath := writeEditMetaDegradedCSV(t)

	out := run(t, cfg, "--dry-run", "issue", "bulk-create", "--from-csv", csvPath, "--template-issue", "MMGFX-13887", "--metadata-mode", "auto")
	if !out["ok"].(bool) {
		t.Fatalf("bulk-create auto fallback dry-run failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	if data["metadata_mode"].(string) != "editmeta_degraded" {
		t.Fatalf("metadata mode = %#v", data["metadata_mode"])
	}
	planned := data["planned_post_create_updates"].([]interface{})
	if len(planned) != 1 {
		t.Fatalf("missing planned updates: %#v", data)
	}
	createFields := data["preview_payloads"].([]interface{})[0].(map[string]interface{})["create_preview"].(map[string]interface{})["fields"].(map[string]interface{})
	for _, field := range []string{"reporter", "priority", "components", "customfield_26388"} {
		if _, ok := createFields[field]; ok {
			t.Fatalf("%s leaked into create preview: %#v", field, createFields)
		}
	}
	if createFields["summary"].(string) != "Login works" {
		t.Fatalf("summary missing from create preview: %#v", createFields)
	}
}

func TestBulkCreateEditMetaDegradedRequiresAndAppliesPostCreateUpdates(t *testing.T) {
	postIssue := 0
	putIssue := 0
	var postPayload map[string]interface{}
	var putPayload map[string]interface{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/rest/api/2/issue":
			postIssue++
			if err := json.NewDecoder(r.Body).Decode(&postPayload); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			w.Write([]byte(`{"key":"MMGFX-20000"}`))
		case r.Method == "PUT" && r.URL.Path == "/rest/api/2/issue/MMGFX-20000":
			putIssue++
			if err := json.NewDecoder(r.Body).Decode(&putPayload); err != nil {
				t.Fatalf("decode put: %v", err)
			}
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	})
	csvPath, mappingPath := writeEditMetaDegradedCreateFixture(t)

	out := run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath)
	if out["ok"].(bool) {
		t.Fatalf("bulk-create should require post-create apply flag: %#v", out)
	}
	if out["error"].(map[string]interface{})["code"].(string) != "post_create_updates_required" {
		t.Fatalf("wrong error: %#v", out)
	}
	if postIssue != 0 || putIssue != 0 {
		t.Fatalf("should not create without apply flag, got POST=%d PUT=%d", postIssue, putIssue)
	}

	out = run(t, cfg, "--yes", "issue", "bulk-create", "--from-csv", csvPath, "--mapping", mappingPath, "--apply-post-create-updates")
	if !out["ok"].(bool) {
		t.Fatalf("bulk-create with post updates failed: %#v", out)
	}
	if postIssue != 1 || putIssue != 1 {
		t.Fatalf("got POST=%d PUT=%d", postIssue, putIssue)
	}
	postFields := postPayload["fields"].(map[string]interface{})
	for _, field := range []string{"reporter", "priority", "components", "customfield_26388"} {
		if _, ok := postFields[field]; ok {
			t.Fatalf("%s leaked into create payload: %#v", field, postPayload)
		}
	}
	if postFields["summary"].(string) != "Login works" {
		t.Fatalf("missing summary in create payload: %#v", postPayload)
	}
	putFields := putPayload["fields"].(map[string]interface{})
	if putFields["reporter"].(map[string]interface{})["name"].(string) != "alice" {
		t.Fatalf("wrong reporter update: %#v", putPayload)
	}
	if putFields["components"].([]interface{})[0].(map[string]interface{})["name"].(string) != "Web" {
		t.Fatalf("wrong component update: %#v", putPayload)
	}
}

func editMetaDegradedMetadataServer(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/field":
			w.Write([]byte(`[{"id":"summary","name":"Summary","schema":{"type":"string"}},{"id":"priority","name":"Priority","schema":{"type":"priority"}},{"id":"components","name":"Components","schema":{"type":"array","items":"component"}},{"id":"reporter","name":"Reporter","schema":{"type":"user","system":"reporter"}},{"id":"customfield_26388","name":"Story Type","schema":{"type":"option"}}]`))
		case r.URL.Path == "/rest/api/2/issue/MMGFX-13887":
			w.Write([]byte(editMetaDegradedIssueJSON()))
		case strings.Contains(r.URL.Path, "/createmeta"):
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"createmeta unavailable"}`))
		case strings.HasSuffix(r.URL.Path, "/editmeta"):
			t.Fatalf("editmeta-degraded fallback should use full issue expand editmeta, got %s", r.URL.String())
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}
}

func writeEditMetaDegradedCSV(t *testing.T) string {
	t.Helper()
	csvPath := filepath.Join(t.TempDir(), "testcases.csv")
	if err := os.WriteFile(csvPath, []byte("Summary,Priority,Component,Reporter,Story Type\nLogin works,High,Web,alice,Feature\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return csvPath
}

func writeEditMetaDegradedCreateFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "testcases.csv")
	mappingPath := filepath.Join(dir, "mapping.json")
	if err := os.WriteFile(csvPath, []byte("Summary,Priority,Component,Reporter,Story Type\nLogin works,High,Web,alice,Feature\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mapping := `{"version":1,"mode":"jira_csv_bulk_create","metadata_mode":"editmeta_degraded","jira":{"project":"MMGFX","project_id":"104804","issuetype":"Story","issuetype_id":"7"},"field_mappings":[{"csv_column":"Summary","jira_field_id":"summary","jira_field_name":"Summary","required":true,"phase":"create","transform":"string","confidence":0.98},{"csv_column":"Priority","jira_field_id":"priority","jira_field_name":"Priority","phase":"post_create_update","transform":"priority","confidence":0.98},{"csv_column":"Component","jira_field_id":"components","jira_field_name":"Components","phase":"post_create_update","transform":"components","confidence":0.98},{"csv_column":"Reporter","jira_field_id":"reporter","jira_field_name":"Reporter","phase":"post_create_update","transform":"user","confidence":0.98},{"csv_column":"Story Type","jira_field_id":"customfield_26388","jira_field_name":"Story Type","phase":"post_create_update","transform":"option","confidence":0.98}],"required_fields":[{"jira_field_id":"summary","jira_field_name":"Summary"}]}`
	if err := os.WriteFile(mappingPath, []byte(mapping), 0o600); err != nil {
		t.Fatal(err)
	}
	return csvPath, mappingPath
}

func editMetaDegradedIssueJSON() string {
	return `{"key":"MMGFX-13887","fields":{"project":{"key":"MMGFX","id":"104804"},"issuetype":{"id":"7","name":"Story"},"summary":"Template"},"names":{"summary":"Summary","description":"Description","priority":"Priority","components":"Components","reporter":"Reporter","customfield_26388":"Story Type","status":"Status","resolution":"Resolution"},"schema":{"summary":{"type":"string"},"description":{"type":"string"},"priority":{"type":"priority"},"components":{"type":"array","items":"component"},"reporter":{"type":"user","system":"reporter"},"customfield_26388":{"type":"option"}},"editmeta":{"fields":{"summary":{"name":"Summary","required":true,"schema":{"type":"string"},"operations":["set"]},"description":{"name":"Description","schema":{"type":"string"},"operations":["set"]},"priority":{"name":"Priority","schema":{"type":"priority"},"allowedValues":[{"id":"3","name":"High"}],"operations":["set"]},"components":{"name":"Components","schema":{"type":"array","items":"component"},"allowedValues":[{"id":"20","name":"Web"}],"operations":["set"]},"reporter":{"name":"Reporter","schema":{"type":"user","system":"reporter"},"operations":["set"]},"customfield_26388":{"name":"Story Type","schema":{"type":"option"},"allowedValues":[{"id":"100","value":"Feature"}],"operations":["set"]},"status":{"name":"Status","schema":{"type":"status"},"operations":["set"]},"resolution":{"name":"Resolution","schema":{"type":"resolution"},"operations":["set"]}}}}`
}

func jsonWarningsContain(warnings []interface{}, code string) bool {
	for _, item := range warnings {
		warning := item.(map[string]interface{})
		if warning["code"].(string) == code {
			return true
		}
	}
	return false
}
