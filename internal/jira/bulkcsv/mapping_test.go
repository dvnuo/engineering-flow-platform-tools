package bulkcsv

import (
	"strings"
	"testing"
)

func TestBuildMappingPlanMapsCommonTestCaseColumns(t *testing.T) {
	input := MappingInput{
		CSV: CSVSummary{
			Path:    "testcases.csv",
			Columns: []string{"Case Title", "Precondition", "Steps", "Expected Result", "Type", "Automation", "Priority", "Component"},
		},
		Rows: []CSVRow{{
			RowNumber: 2,
			Values: map[string]string{
				"Case Title":      "Login works",
				"Precondition":    "User exists",
				"Steps":           "Open login",
				"Expected Result": "Dashboard opens",
				"Type":            "Regression",
				"Automation":      "Manual",
				"Priority":        "High",
				"Component":       "Web",
			},
		}, {
			RowNumber: 3,
			Values: map[string]string{
				"Case Title": "API smoke",
				"Type":       "Smoke",
				"Automation": "Automated",
				"Priority":   "Low",
				"Component":  "API",
			},
		}},
		CreateMeta:              mappingCreateMetaFixture(),
		Jira:                    JiraInfo{ProjectKey: "QA", IssueTypeName: "Test"},
		MinConfidence:           0.75,
		IncludeTemplateDefaults: true,
	}
	plan, err := BuildMappingPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	assertMapping(t, plan, "Case Title", "summary")
	assertMapping(t, plan, "Precondition", "customfield_10401")
	assertMapping(t, plan, "Steps", "customfield_10402")
	assertMapping(t, plan, "Expected Result", "customfield_10403")
	assertMapping(t, plan, "Type", "customfield_10555")
	assertMapping(t, plan, "Automation", "customfield_10666")
	assertMapping(t, plan, "Priority", "priority")
	assertMapping(t, plan, "Component", "components")
	if len(plan.AmbiguousColumns) != 0 {
		t.Fatalf("unexpected ambiguous columns: %#v", plan.AmbiguousColumns)
	}
}

func TestBuildMappingPlanDefersReporterSystemUserUpdate(t *testing.T) {
	input := MappingInput{
		CSV: CSVSummary{Path: "testcases.csv", Columns: []string{"Title", "Reporter"}},
		Rows: []CSVRow{{
			RowNumber: 2,
			Values: map[string]string{
				"Title":    "Login works",
				"Reporter": "XXXXX",
			},
		}},
		FieldCatalog: []interface{}{
			map[string]interface{}{"id": "summary", "name": "Summary", "schema": map[string]interface{}{"type": "string"}},
			map[string]interface{}{"id": "reporter", "name": "Reporter", "schema": map[string]interface{}{"type": "user", "system": "reporter"}},
		},
		CreateMeta: map[string]interface{}{
			"project":   map[string]interface{}{"key": "QA", "id": "10000"},
			"issuetype": map[string]interface{}{"name": "Test", "id": "10001"},
			"fields": map[string]interface{}{
				"summary":  map[string]interface{}{"name": "Summary", "required": true, "schema": map[string]interface{}{"type": "string"}},
				"reporter": map[string]interface{}{"name": "Reporter", "schema": map[string]interface{}{"type": "user", "system": "reporter"}},
			},
		},
		EditMeta: map[string]interface{}{
			"fields": map[string]interface{}{
				"reporter": map[string]interface{}{"name": "Reporter", "schema": map[string]interface{}{"type": "user", "system": "reporter"}},
			},
		},
		MinConfidence: 0.75,
	}
	plan, err := BuildMappingPlan(input)
	if err != nil {
		t.Fatal(err)
	}

	m := mappingFor(t, plan, "Reporter")
	if m.JiraFieldID != "reporter" || m.Transform != "user" || m.Phase != PhasePostCreateUpdate {
		t.Fatalf("reporter mapping = %#v", m)
	}
	if !strings.Contains(m.Reason, "Reporter is a system user field") {
		t.Fatalf("missing reporter deferred reason: %#v", m)
	}

	fields := collectFields(input)
	c := scoreField("Reporter", sampleValues(input.Rows, "Reporter"), fields["reporter"])
	if !containsString(c.Signals, "system_reporter_deferred") {
		t.Fatalf("missing reporter deferred signal: %#v", c.Signals)
	}
}

func TestBuildMappingPlanTransformsSystemUserAndCustomUserPicker(t *testing.T) {
	input := MappingInput{
		CSV: CSVSummary{Path: "testcases.csv", Columns: []string{"Assignee", "Reviewer"}},
		Rows: []CSVRow{{
			RowNumber: 2,
			Values: map[string]string{
				"Assignee": "bob",
				"Reviewer": "alice",
			},
		}},
		CreateMeta: map[string]interface{}{
			"project":   map[string]interface{}{"key": "QA", "id": "10000"},
			"issuetype": map[string]interface{}{"name": "Test", "id": "10001"},
			"fields": map[string]interface{}{
				"assignee": map[string]interface{}{"name": "Assignee", "schema": map[string]interface{}{"type": "user", "system": "assignee"}},
				"customfield_20000": map[string]interface{}{
					"name":   "Reviewer",
					"schema": map[string]interface{}{"type": "any", "custom": "com.atlassian.jira.plugin.system.customfieldtypes:userpicker"},
				},
			},
		},
		MinConfidence: 0.75,
	}
	plan, err := BuildMappingPlan(input)
	if err != nil {
		t.Fatal(err)
	}

	assignee := mappingFor(t, plan, "Assignee")
	if assignee.JiraFieldID != "assignee" || assignee.Transform != "user" {
		t.Fatalf("assignee mapping = %#v", assignee)
	}
	got, rowErr := TransformValue("bob", assignee, 2)
	if rowErr != nil {
		t.Fatal(rowErr)
	}
	if got.(map[string]string)["name"] != "bob" {
		t.Fatalf("assignee transform = %#v", got)
	}

	reviewer := mappingFor(t, plan, "Reviewer")
	if reviewer.JiraFieldID != "customfield_20000" || reviewer.Transform != "user" {
		t.Fatalf("custom userpicker mapping = %#v", reviewer)
	}
}

func TestBuildMappingPlanRejectsEmptyCreateMetaFields(t *testing.T) {
	input := MappingInput{
		CSV: CSVSummary{Path: "testcases.csv", Columns: []string{"Title"}},
		Rows: []CSVRow{{
			RowNumber: 2,
			Values:    map[string]string{"Title": "Login works"},
		}},
		FieldCatalog: []interface{}{
			map[string]interface{}{"id": "summary", "name": "Summary", "schema": map[string]interface{}{"type": "string"}},
		},
		CreateMeta: map[string]interface{}{
			"project":   map[string]interface{}{"key": "QA", "id": "10000"},
			"issuetype": map[string]interface{}{"name": "Test", "id": "10001"},
			"fields":    map[string]interface{}{},
		},
		EditMeta: map[string]interface{}{
			"fields": map[string]interface{}{
				"summary": map[string]interface{}{"name": "Summary", "schema": map[string]interface{}{"type": "string"}},
			},
		},
		MinConfidence: 0.75,
	}

	plan, err := BuildMappingPlan(input)
	if err == nil {
		t.Fatalf("expected empty createmeta error, got plan: %#v", plan)
	}
	bulkErr, ok := err.(*Error)
	if !ok || bulkErr.Code != "createmeta_fields_empty" {
		t.Fatalf("wrong error: %#v", err)
	}
}

func TestBuildMappingPlanDoesNotMapSummaryAsPostCreateUpdate(t *testing.T) {
	input := MappingInput{
		CSV: CSVSummary{Path: "testcases.csv", Columns: []string{"Title"}},
		Rows: []CSVRow{{
			RowNumber: 2,
			Values:    map[string]string{"Title": "Login works"},
		}},
		FieldCatalog: []interface{}{
			map[string]interface{}{"id": "summary", "name": "Summary", "schema": map[string]interface{}{"type": "string"}},
			map[string]interface{}{"id": "priority", "name": "Priority", "schema": map[string]interface{}{"type": "priority"}},
		},
		CreateMeta: map[string]interface{}{
			"project":   map[string]interface{}{"key": "QA", "id": "10000"},
			"issuetype": map[string]interface{}{"name": "Test", "id": "10001"},
			"fields": map[string]interface{}{
				"priority": map[string]interface{}{"name": "Priority", "schema": map[string]interface{}{"type": "priority"}},
			},
		},
		EditMeta: map[string]interface{}{
			"fields": map[string]interface{}{
				"summary": map[string]interface{}{"name": "Summary", "schema": map[string]interface{}{"type": "string"}},
			},
		},
		MinConfidence: 0.75,
	}

	plan, err := BuildMappingPlan(input)
	if err == nil {
		for _, mapping := range plan.FieldMappings {
			if mapping.JiraFieldID == "summary" && mapping.Phase == PhasePostCreateUpdate {
				t.Fatalf("summary mapped as post-create update: %#v", plan)
			}
		}
		t.Fatalf("expected summary_not_creatable error, got plan: %#v", plan)
	}
	bulkErr, ok := err.(*Error)
	if !ok || bulkErr.Code != "summary_not_creatable" {
		t.Fatalf("wrong error: %#v", err)
	}
}

func assertMapping(t *testing.T, plan MappingPlan, column, fieldID string) {
	t.Helper()
	m := mappingFor(t, plan, column)
	if m.JiraFieldID != fieldID {
		t.Fatalf("%s mapped to %s, want %s", column, m.JiraFieldID, fieldID)
	}
}

func mappingFor(t *testing.T, plan MappingPlan, column string) FieldMapping {
	t.Helper()
	for _, m := range plan.FieldMappings {
		if m.CSVColumn == column {
			return m
		}
	}
	t.Fatalf("mapping for %s not found", column)
	return FieldMapping{}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mappingCreateMetaFixture() map[string]interface{} {
	return map[string]interface{}{
		"project":   map[string]interface{}{"key": "QA", "id": "10000"},
		"issuetype": map[string]interface{}{"name": "Test", "id": "10001"},
		"fields": map[string]interface{}{
			"summary":           map[string]interface{}{"name": "Summary", "required": true, "schema": map[string]interface{}{"type": "string"}},
			"customfield_10401": map[string]interface{}{"name": "Preconditions", "schema": map[string]interface{}{"type": "string"}},
			"customfield_10402": map[string]interface{}{"name": "Test Steps", "schema": map[string]interface{}{"type": "string"}},
			"customfield_10403": map[string]interface{}{"name": "Expected Result", "schema": map[string]interface{}{"type": "string"}},
			"customfield_10555": map[string]interface{}{
				"name":          "Test Type",
				"schema":        map[string]interface{}{"type": "option"},
				"allowedValues": []interface{}{map[string]interface{}{"id": "1", "value": "Regression"}, map[string]interface{}{"id": "2", "value": "Smoke"}},
			},
			"customfield_10666": map[string]interface{}{
				"name":          "Automation Status",
				"schema":        map[string]interface{}{"type": "option"},
				"allowedValues": []interface{}{map[string]interface{}{"id": "10", "value": "Manual"}, map[string]interface{}{"id": "11", "value": "Automated"}},
			},
			"priority": map[string]interface{}{
				"name":          "Priority",
				"schema":        map[string]interface{}{"type": "priority"},
				"allowedValues": []interface{}{map[string]interface{}{"id": "3", "name": "High"}, map[string]interface{}{"id": "4", "name": "Low"}},
			},
			"components": map[string]interface{}{
				"name":          "Components",
				"schema":        map[string]interface{}{"type": "array", "items": "component"},
				"allowedValues": []interface{}{map[string]interface{}{"id": "20", "name": "Web"}, map[string]interface{}{"id": "21", "name": "API"}},
			},
		},
	}
}
