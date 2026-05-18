package bulkcsv

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func BuildCreatePayload(row CSVRow, plan MappingPlan) (map[string]interface{}, []RowError, *PostCreateUpdate) {
	metadataMode := EffectiveMetadataMode(plan.MetadataMode)
	fields := map[string]interface{}{}
	if plan.Jira.ProjectID != "" {
		fields["project"] = map[string]string{"id": plan.Jira.ProjectID}
	} else if plan.Jira.ProjectKey != "" {
		fields["project"] = map[string]string{"key": plan.Jira.ProjectKey}
	}
	if plan.Jira.IssueTypeID != "" {
		fields["issuetype"] = map[string]string{"id": plan.Jira.IssueTypeID}
	} else if plan.Jira.IssueTypeName != "" {
		fields["issuetype"] = map[string]string{"name": plan.Jira.IssueTypeName}
	}
	if metadataMode != MetadataModeEditMetaDegraded {
		for _, d := range plan.TemplateDefaults {
			fields[d.JiraFieldID] = cloneJSONValue(d.Value)
		}
	}

	errors := []RowError{}
	postFields := map[string]interface{}{}
	for _, m := range plan.FieldMappings {
		raw := row.Values[m.CSVColumn]
		if strings.TrimSpace(raw) == "" {
			continue
		}
		value, rowErr := TransformValue(raw, m, row.RowNumber)
		if rowErr != nil {
			errors = append(errors, *rowErr)
			continue
		}
		if createPayloadField(metadataMode, m) {
			fields[m.JiraFieldID] = value
		} else {
			postFields[m.JiraFieldID] = value
		}
	}
	for _, required := range plan.RequiredFields {
		if required.JiraFieldID == "summary" {
			continue
		}
		if isEmptyValue(fields[required.JiraFieldID]) {
			errors = append(errors, RowError{
				RowNumber:   row.RowNumber,
				JiraFieldID: required.JiraFieldID,
				Code:        "required_field_missing",
				Message:     required.JiraFieldID + " is required by create metadata",
			})
		}
	}
	errors = append(errors, validateCoreCreateFields(fields, row.RowNumber)...)
	payload := map[string]interface{}{"fields": fields}
	var post *PostCreateUpdate
	if len(postFields) > 0 {
		post = &PostCreateUpdate{RowNumber: row.RowNumber, Fields: postFields, Payload: map[string]interface{}{"fields": postFields}}
	}
	return payload, errors, post
}

func createPayloadField(metadataMode string, mapping FieldMapping) bool {
	if metadataMode == MetadataModeEditMetaDegraded {
		return mapping.JiraFieldID == "summary"
	}
	return mapping.Phase != PhasePostCreateUpdate
}

func validateCoreCreateFields(fields map[string]interface{}, rowNumber int) []RowError {
	errors := []RowError{}
	if isEmptyValue(fields["project"]) {
		errors = append(errors, RowError{
			RowNumber:   rowNumber,
			JiraFieldID: "project",
			Code:        "project_required_missing",
			Message:     "project is required for Jira issue creation; mapping plan did not include Jira project metadata.",
		})
	}
	if isEmptyValue(fields["issuetype"]) {
		errors = append(errors, RowError{
			RowNumber:   rowNumber,
			JiraFieldID: "issuetype",
			Code:        "issuetype_required_missing",
			Message:     "issuetype is required for Jira issue creation; mapping plan did not include Jira issue type metadata.",
		})
	}
	if isEmptyValue(fields["summary"]) {
		errors = append(errors, RowError{
			RowNumber:   rowNumber,
			JiraFieldID: "summary",
			Code:        "summary_required_missing",
			Message:     "summary is required for Jira issue creation; mapping placed it outside the create payload or did not map it.",
		})
	}
	return errors
}

func BuildPostCreateUpdatePayload(row CSVRow, plan MappingPlan) (map[string]interface{}, []RowError) {
	fields := map[string]interface{}{}
	errors := []RowError{}
	for _, m := range plan.FieldMappings {
		if m.Phase != PhasePostCreateUpdate {
			continue
		}
		raw := row.Values[m.CSVColumn]
		if strings.TrimSpace(raw) == "" {
			continue
		}
		value, rowErr := TransformValue(raw, m, row.RowNumber)
		if rowErr != nil {
			errors = append(errors, *rowErr)
			continue
		}
		fields[m.JiraFieldID] = value
	}
	return fields, errors
}

func DryRunRows(rows []CSVRow, plan MappingPlan, previewLimit int) DryRunResult {
	result := DryRunResult{DryRun: true, MetadataMode: EffectiveMetadataMode(plan.MetadataMode), Rows: len(rows), PreviewPayloads: []PayloadPreview{}, Errors: []RowError{}, Warnings: plan.Warnings}
	for _, row := range rows {
		payload, errs, post := BuildCreatePayload(row, plan)
		if len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
			result.InvalidRows++
			continue
		}
		result.ValidRows++
		if previewLimit < 0 || len(result.PreviewPayloads) < previewLimit {
			result.PreviewPayloads = append(result.PreviewPayloads, PayloadPreview{RowNumber: row.RowNumber, Payload: payload, CreatePreview: payload})
		}
		if post != nil {
			result.PlannedPostCreateUpdates = append(result.PlannedPostCreateUpdates, *post)
		}
	}
	if len(result.PlannedPostCreateUpdates) > 0 {
		result.Warnings = append(result.Warnings, PlanWarning{Code: "post_create_updates_planned_not_applied", Message: "post-create update fields are planned but not sent in create payloads"})
	}
	return result
}

func TransformValue(raw string, mapping FieldMapping, rowNumber int) (interface{}, *RowError) {
	value := strings.TrimSpace(raw)
	switch mapping.Transform {
	case "", "string":
		return raw, nil
	case "raw_json":
		var out interface{}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, transformError(rowNumber, mapping, "invalid_json", "raw_json transform value is not valid JSON")
		}
		return out, nil
	case "option":
		allowed, ok := matchAllowed(value, mapping.AllowedValues)
		if len(mapping.AllowedValues) > 0 && !ok {
			return nil, transformError(rowNumber, mapping, "invalid_option", "value is not allowed for "+mapping.JiraFieldID)
		}
		return optionPayload(value, allowed), nil
	case "multi_option":
		out := []interface{}{}
		for _, part := range splitList(value) {
			allowed, ok := matchAllowed(part, mapping.AllowedValues)
			if len(mapping.AllowedValues) > 0 && !ok {
				return nil, transformError(rowNumber, mapping, "invalid_option", "value is not allowed for "+mapping.JiraFieldID)
			}
			out = append(out, optionPayload(part, allowed))
		}
		return out, nil
	case "priority":
		allowed, ok := matchAllowed(value, mapping.AllowedValues)
		if len(mapping.AllowedValues) > 0 && !ok {
			return nil, transformError(rowNumber, mapping, "invalid_option", "priority value is not allowed")
		}
		if ok && allowed.ID != "" {
			return map[string]string{"id": allowed.ID}, nil
		}
		return map[string]string{"name": value}, nil
	case "components", "versions":
		out := []interface{}{}
		for _, part := range splitList(value) {
			allowed, ok := matchAllowed(part, mapping.AllowedValues)
			if len(mapping.AllowedValues) > 0 && !ok {
				return nil, transformError(rowNumber, mapping, "invalid_option", "value is not allowed for "+mapping.JiraFieldID)
			}
			name := part
			if ok {
				name = firstString(allowed.Name, allowed.Value, part)
			}
			out = append(out, map[string]string{"name": name})
		}
		return out, nil
	case "labels", "string_array":
		out := []string{}
		for _, part := range splitList(value) {
			out = append(out, part)
		}
		return out, nil
	case "date":
		d, err := parseDate(value)
		if err != nil {
			return nil, transformError(rowNumber, mapping, "invalid_date", "date must be parseable as YYYY-MM-DD")
		}
		return d, nil
	case "datetime":
		d, err := parseDateTime(value)
		if err != nil {
			return nil, transformError(rowNumber, mapping, "invalid_datetime", "datetime must be parseable")
		}
		return d, nil
	case "user":
		return map[string]string{"name": value}, nil
	case "group":
		return map[string]string{"name": value}, nil
	case "cascading_select":
		parts := strings.SplitN(value, ">", 2)
		parent := strings.TrimSpace(parts[0])
		if len(parts) == 1 {
			return map[string]string{"value": parent}, nil
		}
		child := strings.TrimSpace(parts[1])
		return map[string]interface{}{"value": parent, "child": map[string]string{"value": child}}, nil
	case "unknown":
		return nil, transformError(rowNumber, mapping, "requires_confirmation", "unknown schema requires raw_json transform")
	default:
		return raw, nil
	}
}

func transformError(rowNumber int, mapping FieldMapping, code, message string) *RowError {
	return &RowError{RowNumber: rowNumber, CSVColumn: mapping.CSVColumn, JiraFieldID: mapping.JiraFieldID, Code: code, Message: message}
}

func optionPayload(input string, allowed AllowedValue) interface{} {
	if allowed.ID != "" {
		return map[string]string{"id": allowed.ID}
	}
	return map[string]string{"value": firstString(allowed.Value, allowed.Name, input)}
}

func matchAllowed(input string, allowed []AllowedValue) (AllowedValue, bool) {
	n := normalizeName(input)
	for _, v := range allowed {
		for _, part := range []string{v.ID, v.Name, v.Value} {
			if normalizeName(part) == n {
				return v, true
			}
		}
	}
	return AllowedValue{}, false
}

func splitList(value string) []string {
	f := func(r rune) bool { return r == ',' || r == ';' }
	parts := strings.FieldsFunc(value, f)
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 && strings.TrimSpace(value) != "" {
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

func parseDate(value string) (string, error) {
	for _, layout := range []string{"2006-01-02", "2006/01/02", "01/02/2006", "1/2/2006"} {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("invalid date")
}

func parseDateTime(value string) (string, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006/01/02 15:04:05"} {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t.Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("invalid datetime")
}

func cloneJSONValue(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
