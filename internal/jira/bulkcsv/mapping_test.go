package bulkcsv

import "testing"

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

func assertMapping(t *testing.T, plan MappingPlan, column, fieldID string) {
	t.Helper()
	for _, m := range plan.FieldMappings {
		if m.CSVColumn == column {
			if m.JiraFieldID != fieldID {
				t.Fatalf("%s mapped to %s, want %s", column, m.JiraFieldID, fieldID)
			}
			return
		}
	}
	t.Fatalf("mapping for %s not found", column)
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
