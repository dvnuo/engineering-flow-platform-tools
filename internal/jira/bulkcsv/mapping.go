package bulkcsv

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func BuildMappingPlan(input MappingInput) (MappingPlan, error) {
	if input.MinConfidence == 0 {
		input.MinConfidence = 0.75
	}
	fields := collectFields(input)
	jiraInfo := inferJiraInfo(input)
	plan := MappingPlan{
		Version:          PlanVersion,
		Mode:             PlanMode,
		Jira:             jiraInfo,
		CSV:              input.CSV,
		FieldMappings:    []FieldMapping{},
		TemplateDefaults: []TemplateDefault{},
		BlockedFields:    []BlockedField{},
		AmbiguousColumns: []AmbiguousColumn{},
		UnmappedColumns:  []UnmappedColumn{},
		Warnings:         []PlanWarning{},
		RequiredFields:   []FieldRef{},
		CreateFields:     map[string]FieldRef{},
	}

	for _, id := range sortedFieldIDs(fields) {
		f := fields[id]
		if f.Creatable {
			plan.CreateFields[id] = FieldRef{JiraFieldID: id, JiraFieldName: f.Name}
			if f.Required {
				plan.RequiredFields = append(plan.RequiredFields, FieldRef{JiraFieldID: id, JiraFieldName: f.Name})
			}
		}
	}

	for _, column := range input.CSV.Columns {
		candidates := candidatesForColumn(column, sampleValues(input.Rows, column), fields)
		if len(candidates) == 0 || candidates[0].Confidence < 0.55 {
			plan.UnmappedColumns = append(plan.UnmappedColumns, UnmappedColumn{CSVColumn: column, Reason: "no confident Jira field match"})
			continue
		}
		if candidates[0].Confidence < input.MinConfidence {
			plan.AmbiguousColumns = append(plan.AmbiguousColumns, AmbiguousColumn{CSVColumn: column, Candidates: trimCandidates(candidates, 5)})
			continue
		}
		top := candidates[0]
		mapping := FieldMapping{
			CSVColumn:     column,
			JiraFieldID:   top.JiraFieldID,
			JiraFieldName: top.JiraFieldName,
			SchemaType:    top.SchemaType,
			SchemaCustom:  top.SchemaCustom,
			Required:      top.Required,
			Phase:         top.Phase,
			Transform:     transformName(fields[top.JiraFieldID]),
			Confidence:    roundConfidence(top.Confidence),
			Reason:        top.Reason,
			AllowedValues: top.AllowedValues,
		}
		if mapping.Transform == "unknown" {
			plan.RequiresConfirmation = append(plan.RequiresConfirmation, ConfirmationItem{CSVColumn: column, JiraFieldID: mapping.JiraFieldID, Reason: "unknown field schema requires an explicit raw_json transform"})
		}
		if top.Confidence < 0.90 {
			plan.RequiresConfirmation = append(plan.RequiresConfirmation, ConfirmationItem{CSVColumn: column, JiraFieldID: mapping.JiraFieldID, Reason: "mapping confidence below 0.90"})
		}
		if mapping.Phase == PhasePostCreateUpdate {
			plan.Warnings = append(plan.Warnings, PlanWarning{Code: "post_create_update_planned", Message: mapping.JiraFieldID + " is editable after create but not available in create metadata"})
		}
		plan.FieldMappings = append(plan.FieldMappings, mapping)
	}

	if input.IncludeTemplateDefaults {
		addTemplateDefaults(&plan, input, fields)
	}
	addMissingRequiredWarnings(&plan)
	return plan, nil
}

func candidatesForColumn(column string, samples []string, fields map[string]fieldInfo) []FieldCandidate {
	out := []FieldCandidate{}
	for _, id := range sortedFieldIDs(fields) {
		f := fields[id]
		if (!f.Creatable && !f.Editable) || id == "project" || id == "issuetype" {
			continue
		}
		c := scoreField(column, samples, f)
		if c.Confidence > 0 {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].JiraFieldID < out[j].JiraFieldID
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}

func scoreField(column string, samples []string, f fieldInfo) FieldCandidate {
	colNorm := normalizeName(column)
	fieldNorm := normalizeName(f.Name)
	score := 0.0
	signals := []string{}
	reasons := []string{}

	if colNorm == fieldNorm && colNorm != "" {
		score = math.Max(score, 0.93)
		signals = append(signals, "exact_display_name")
		reasons = append(reasons, "CSV header matches Jira field name")
	} else if colNorm != "" && fieldNorm != "" && (strings.Contains(fieldNorm, colNorm) || strings.Contains(colNorm, fieldNorm)) {
		score = math.Max(score, 0.78)
		signals = append(signals, "normalized_name_overlap")
		reasons = append(reasons, "CSV header overlaps Jira field name")
	}
	aliasScore, aliasSignal := builtInAliasScore(colNorm, fieldNorm, f)
	if aliasScore > 0 {
		score = math.Max(score, aliasScore)
		signals = append(signals, aliasSignal)
		reasons = append(reasons, "built-in alias matched")
	}
	if overlap := allowedValueOverlap(samples, f.AllowedValues); overlap > 0 {
		allowedScore := 0.88 + math.Min(0.10, float64(overlap)*0.04)
		score = math.Max(score, allowedScore)
		signals = append(signals, "allowed_values_overlap")
		reasons = append(reasons, "CSV sample values overlap Jira allowedValues")
	}
	if schemaCompatible(samples, f) && score > 0 {
		score = math.Min(0.99, score+0.03)
		signals = append(signals, "schema_compatible")
	}
	if !f.Creatable && f.Editable && score > 0 {
		score = math.Max(0, score-0.04)
		signals = append(signals, "editmeta_only")
	}
	if score == 0 {
		return FieldCandidate{}
	}
	phase := PhaseCreate
	if !f.Creatable && f.Editable {
		phase = PhasePostCreateUpdate
	}
	return FieldCandidate{
		JiraFieldID:   f.ID,
		JiraFieldName: f.Name,
		SchemaType:    f.SchemaType,
		SchemaCustom:  f.SchemaCustom,
		Required:      f.Required,
		AllowedValues: f.AllowedValues,
		Confidence:    roundConfidence(score),
		Signals:       signals,
		Reason:        strings.Join(uniqueStrings(reasons), "; "),
		Phase:         phase,
	}
}

func builtInAliasScore(colNorm, fieldNorm string, f fieldInfo) (float64, string) {
	switch {
	case hasAlias(colNorm, "case title", "test case title", "title", "name") && f.ID == "summary":
		return 0.98, "alias_summary"
	case hasAlias(colNorm, "description", "desc") && f.ID == "description":
		return 0.96, "alias_description"
	case hasAlias(colNorm, "precondition", "preconditions", "pre condition") && strings.Contains(fieldNorm, "precondition"):
		return 0.96, "alias_preconditions"
	case hasAlias(colNorm, "steps", "test steps", "step") && (strings.Contains(fieldNorm, "test step") || strings.Contains(fieldNorm, "step")):
		return 0.96, "alias_steps"
	case hasAlias(colNorm, "expected", "expected result", "expected results") && strings.Contains(fieldNorm, "expected"):
		return 0.96, "alias_expected"
	case hasAlias(colNorm, "actual", "actual result") && strings.Contains(fieldNorm, "actual"):
		return 0.94, "alias_actual"
	case hasAlias(colNorm, "priority") && f.ID == "priority":
		return 0.96, "alias_priority"
	case hasAlias(colNorm, "component", "components", "component area") && f.ID == "components":
		return 0.93, "alias_components"
	case hasAlias(colNorm, "label", "labels", "tags") && f.ID == "labels":
		return 0.93, "alias_labels"
	case hasAlias(colNorm, "type") && strings.Contains(fieldNorm, "test type"):
		return 0.84, "alias_test_type"
	case hasAlias(colNorm, "automation", "automated", "automation status") && strings.Contains(fieldNorm, "automation"):
		return 0.90, "alias_automation"
	default:
		return 0, ""
	}
}

func addTemplateDefaults(plan *MappingPlan, input MappingInput, fields map[string]fieldInfo) {
	mapped := map[string]bool{}
	for _, m := range plan.FieldMappings {
		mapped[m.JiraFieldID] = true
	}
	exampleFields := exampleIssueFields(input.ExampleIssue)
	for _, id := range sortedFieldIDs(fields) {
		f := fields[id]
		if !f.Creatable || mapped[id] {
			continue
		}
		if isBlockedTemplateField(id) {
			if !isEmptyValue(exampleFields[id]) {
				plan.BlockedFields = append(plan.BlockedFields, BlockedField{JiraFieldID: id, JiraFieldName: f.Name, Reason: "template field is not safe to copy"})
			}
			continue
		}
		value, ok := exampleFields[id]
		if !ok || isEmptyValue(value) {
			continue
		}
		plan.TemplateDefaults = append(plan.TemplateDefaults, TemplateDefault{JiraFieldID: id, JiraFieldName: f.Name, Required: f.Required, Value: value})
	}
}

func addMissingRequiredWarnings(plan *MappingPlan) {
	satisfied := map[string]bool{}
	for _, m := range plan.FieldMappings {
		if m.Phase == PhaseCreate {
			satisfied[m.JiraFieldID] = true
		}
	}
	for _, d := range plan.TemplateDefaults {
		satisfied[d.JiraFieldID] = true
	}
	for _, f := range plan.RequiredFields {
		if !satisfied[f.JiraFieldID] {
			plan.Warnings = append(plan.Warnings, PlanWarning{Code: "required_field_missing", Message: f.JiraFieldID + " is required by create metadata but has no CSV mapping or template default"})
		}
	}
}

func transformName(f fieldInfo) string {
	id := f.ID
	schemaType := strings.ToLower(f.SchemaType)
	schemaItems := strings.ToLower(f.SchemaItems)
	schemaCustom := strings.ToLower(f.SchemaCustom)
	switch {
	case id == "summary" || id == "description" || schemaType == "string" || schemaType == "textarea":
		return "string"
	case id == "priority":
		return "priority"
	case id == "components":
		return "components"
	case id == "versions" || id == "fixVersions" || id == "affectedVersions":
		return "versions"
	case id == "labels":
		return "labels"
	case schemaType == "date":
		return "date"
	case schemaType == "datetime":
		return "datetime"
	case schemaType == "array" && (schemaItems == "option" || strings.Contains(schemaCustom, "multiselect") || strings.Contains(schemaCustom, "multicheckboxes")):
		return "multi_option"
	case schemaType == "option" || len(f.AllowedValues) > 0 || strings.Contains(schemaCustom, "select"):
		return "option"
	case strings.Contains(schemaCustom, "userpicker"):
		return "user"
	case strings.Contains(schemaCustom, "grouppicker"):
		return "group"
	case strings.Contains(schemaCustom, "cascadingselect"):
		return "cascading_select"
	case schemaType == "array":
		return "string_array"
	case schemaType == "":
		return "unknown"
	default:
		return "string"
	}
}

func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonWord.ReplaceAllString(s, " ")
	parts := strings.Fields(s)
	for i, p := range parts {
		if len(p) > 3 && strings.HasSuffix(p, "s") {
			parts[i] = strings.TrimSuffix(p, "s")
		}
	}
	return strings.Join(parts, " ")
}

func hasAlias(value string, aliases ...string) bool {
	for _, alias := range aliases {
		if value == normalizeName(alias) {
			return true
		}
	}
	return false
}

func sampleValues(rows []CSVRow, column string) []string {
	out := []string{}
	for _, row := range rows {
		v := strings.TrimSpace(row.Values[column])
		if v != "" {
			out = append(out, v)
		}
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func allowedValueOverlap(samples []string, allowed []AllowedValue) int {
	if len(samples) == 0 || len(allowed) == 0 {
		return 0
	}
	allowedSet := map[string]bool{}
	for _, v := range allowed {
		for _, part := range []string{v.ID, v.Name, v.Value} {
			n := normalizeName(part)
			if n != "" {
				allowedSet[n] = true
			}
		}
	}
	overlap := 0
	for _, sample := range samples {
		for _, part := range splitList(sample) {
			if allowedSet[normalizeName(part)] {
				overlap++
				break
			}
		}
	}
	return overlap
}

func schemaCompatible(samples []string, f fieldInfo) bool {
	if len(samples) == 0 {
		return true
	}
	transform := transformName(f)
	for _, sample := range samples {
		if strings.TrimSpace(sample) == "" {
			continue
		}
		switch transform {
		case "multi_option", "components", "labels", "versions", "string_array":
			if strings.ContainsAny(sample, ",;") {
				return true
			}
		default:
			return true
		}
	}
	return true
}

func sortedFieldIDs(fields map[string]fieldInfo) []string {
	ids := make([]string, 0, len(fields))
	for id := range fields {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func trimCandidates(in []FieldCandidate, n int) []FieldCandidate {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func roundConfidence(v float64) float64 {
	return math.Round(v*100) / 100
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) == ""
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	default:
		return false
	}
}
