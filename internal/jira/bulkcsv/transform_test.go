package bulkcsv

import "testing"

func TestTransformOptionUsesAllowedValueID(t *testing.T) {
	m := FieldMapping{CSVColumn: "Type", JiraFieldID: "customfield_10555", Transform: "option", AllowedValues: []AllowedValue{{ID: "1", Value: "Regression"}}}
	got, rowErr := TransformValue("Regression", m, 2)
	if rowErr != nil {
		t.Fatal(rowErr)
	}
	obj := got.(map[string]string)
	if obj["id"] != "1" {
		t.Fatalf("option payload = %#v", got)
	}
}

func TestTransformMultiOptionSplitsSemicolon(t *testing.T) {
	m := FieldMapping{
		CSVColumn: "Tags", JiraFieldID: "customfield_10010", Transform: "multi_option",
		AllowedValues: []AllowedValue{{ID: "1", Value: "Regression"}, {ID: "2", Value: "Smoke"}},
	}
	got, rowErr := TransformValue("Regression; Smoke", m, 2)
	if rowErr != nil {
		t.Fatal(rowErr)
	}
	items := got.([]interface{})
	if len(items) != 2 || items[0].(map[string]string)["id"] != "1" || items[1].(map[string]string)["id"] != "2" {
		t.Fatalf("multi option payload = %#v", got)
	}
}

func TestTransformComponentsAndLabels(t *testing.T) {
	components, rowErr := TransformValue("Web; API", FieldMapping{CSVColumn: "Component", JiraFieldID: "components", Transform: "components"}, 2)
	if rowErr != nil {
		t.Fatal(rowErr)
	}
	componentItems := components.([]interface{})
	if len(componentItems) != 2 || componentItems[0].(map[string]string)["name"] != "Web" {
		t.Fatalf("components payload = %#v", components)
	}

	labels, rowErr := TransformValue("smoke, checkout", FieldMapping{CSVColumn: "Labels", JiraFieldID: "labels", Transform: "labels"}, 2)
	if rowErr != nil {
		t.Fatal(rowErr)
	}
	labelItems := labels.([]string)
	if len(labelItems) != 2 || labelItems[1] != "checkout" {
		t.Fatalf("labels payload = %#v", labels)
	}
}

func TestTransformInvalidOptionReturnsRowError(t *testing.T) {
	m := FieldMapping{CSVColumn: "Type", JiraFieldID: "customfield_10555", Transform: "option", AllowedValues: []AllowedValue{{ID: "1", Value: "Regression"}}}
	_, rowErr := TransformValue("Exploratory", m, 7)
	if rowErr == nil || rowErr.RowNumber != 7 || rowErr.Code != "invalid_option" {
		t.Fatalf("row error = %#v", rowErr)
	}
}

func TestDryRunRowsReturnsPreviewPayloadsAndValidationErrors(t *testing.T) {
	plan := MappingPlan{
		Version: PlanVersion,
		Mode:    PlanMode,
		Jira:    JiraInfo{ProjectKey: "QA", IssueTypeName: "Test"},
		FieldMappings: []FieldMapping{
			{CSVColumn: "Title", JiraFieldID: "summary", Phase: PhaseCreate, Transform: "string", Required: true},
			{CSVColumn: "Type", JiraFieldID: "customfield_10555", Phase: PhaseCreate, Transform: "option", AllowedValues: []AllowedValue{{ID: "1", Value: "Regression"}}},
		},
		RequiredFields: []FieldRef{{JiraFieldID: "summary", JiraFieldName: "Summary"}},
	}
	rows := []CSVRow{
		{RowNumber: 2, Values: map[string]string{"Title": "Login", "Type": "Regression"}},
		{RowNumber: 3, Values: map[string]string{"Title": "Bad", "Type": "Smoke"}},
	}
	result := DryRunRows(rows, plan, 3)
	if result.ValidRows != 1 || result.InvalidRows != 1 || len(result.PreviewPayloads) != 1 {
		t.Fatalf("dry run result = %#v", result)
	}
	if result.Errors[0].Code != "invalid_option" || result.Errors[0].RowNumber != 3 {
		t.Fatalf("dry run errors = %#v", result.Errors)
	}
}

func TestBuildPostCreateUpdatePayload(t *testing.T) {
	plan := MappingPlan{FieldMappings: []FieldMapping{
		{CSVColumn: "Title", JiraFieldID: "summary", Phase: PhaseCreate, Transform: "string"},
		{CSVColumn: "Reviewer", JiraFieldID: "customfield_20000", Phase: PhasePostCreateUpdate, Transform: "user"},
	}}
	fields, errs := BuildPostCreateUpdatePayload(CSVRow{RowNumber: 2, Values: map[string]string{"Title": "Login", "Reviewer": "alice"}}, plan)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
	if len(fields) != 1 {
		t.Fatalf("post fields = %#v", fields)
	}
	user := fields["customfield_20000"].(map[string]string)
	if user["name"] != "alice" {
		t.Fatalf("post update user = %#v", fields)
	}
}
