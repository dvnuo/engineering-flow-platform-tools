package bulkcsv

import (
	"fmt"
	"strings"
)

type fieldInfo struct {
	ID            string
	Name          string
	SchemaType    string
	SchemaItems   string
	SchemaCustom  string
	Required      bool
	AllowedValues []AllowedValue
	Creatable     bool
	Editable      bool
	Raw           map[string]interface{}
}

func collectFields(input MappingInput) map[string]fieldInfo {
	fields := map[string]fieldInfo{}
	for _, f := range fieldCatalogItems(input.FieldCatalog) {
		id := stringAt(f, "id")
		if id == "" {
			id = stringAt(f, "key")
		}
		if id == "" {
			continue
		}
		info := fields[id]
		info.ID = id
		info.Name = firstString(info.Name, stringAt(f, "name"))
		applySchema(&info, asMap(f["schema"]))
		info.Raw = mergeRaw(info.Raw, f)
		fields[id] = info
	}
	for id, raw := range fieldsMap(input.CreateMeta) {
		info := fields[id]
		info.ID = id
		info.Name = firstString(stringAt(raw, "name"), info.Name, id)
		info.Required = boolAt(raw, "required")
		info.AllowedValues = allowedValues(raw["allowedValues"])
		info.Creatable = true
		applySchema(&info, asMap(raw["schema"]))
		info.Raw = mergeRaw(info.Raw, raw)
		fields[id] = info
	}
	for id, raw := range fieldsMap(input.EditMeta) {
		info := fields[id]
		info.ID = id
		info.Name = firstString(info.Name, stringAt(raw, "name"), id)
		if len(info.AllowedValues) == 0 {
			info.AllowedValues = allowedValues(raw["allowedValues"])
		}
		info.Editable = true
		applySchema(&info, asMap(raw["schema"]))
		info.Raw = mergeRaw(info.Raw, raw)
		fields[id] = info
	}
	for id, raw := range exampleNames(input.ExampleIssue) {
		info := fields[id]
		info.ID = id
		info.Name = firstString(info.Name, raw)
		fields[id] = info
	}
	return fields
}

func fieldCatalogItems(v interface{}) []map[string]interface{} {
	switch x := v.(type) {
	case []interface{}:
		return mapsFromArray(x)
	case []map[string]interface{}:
		return x
	case map[string]interface{}:
		for _, key := range []string{"fields", "data", "values"} {
			if arr := asArray(x[key]); len(arr) > 0 {
				return mapsFromArray(arr)
			}
		}
	}
	return nil
}

func fieldsMap(v map[string]interface{}) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	if v == nil {
		return out
	}
	fields := asMap(v["fields"])
	if len(fields) == 0 {
		return out
	}
	for id, raw := range fields {
		if m := asMap(raw); len(m) > 0 {
			out[id] = m
		}
	}
	return out
}

func exampleNames(issue map[string]interface{}) map[string]string {
	out := map[string]string{}
	names := asMap(issue["names"])
	for k, v := range names {
		out[k] = fmt.Sprint(v)
	}
	return out
}

func exampleIssueFields(issue map[string]interface{}) map[string]interface{} {
	return asMap(issue["fields"])
}

func inferJiraInfo(input MappingInput) JiraInfo {
	info := input.Jira
	if info.ProjectKey == "" || info.ProjectID == "" || info.IssueTypeName == "" || info.IssueTypeID == "" {
		fields := exampleIssueFields(input.ExampleIssue)
		project := asMap(fields["project"])
		issueType := asMap(fields["issuetype"])
		if info.ProjectKey == "" {
			info.ProjectKey = stringAt(project, "key")
		}
		if info.ProjectID == "" {
			info.ProjectID = stringAt(project, "id")
		}
		if info.IssueTypeName == "" {
			info.IssueTypeName = stringAt(issueType, "name")
		}
		if info.IssueTypeID == "" {
			info.IssueTypeID = stringAt(issueType, "id")
		}
	}
	project := asMap(input.CreateMeta["project"])
	issueType := asMap(input.CreateMeta["issuetype"])
	if info.ProjectKey == "" {
		info.ProjectKey = stringAt(project, "key")
	}
	if info.ProjectID == "" {
		info.ProjectID = stringAt(project, "id")
	}
	if info.IssueTypeName == "" {
		info.IssueTypeName = stringAt(issueType, "name")
	}
	if info.IssueTypeID == "" {
		info.IssueTypeID = stringAt(issueType, "id")
	}
	return info
}

func applySchema(info *fieldInfo, schema map[string]interface{}) {
	if len(schema) == 0 {
		return
	}
	info.SchemaType = firstString(info.SchemaType, stringAt(schema, "type"))
	info.SchemaItems = firstString(info.SchemaItems, stringAt(schema, "items"))
	info.SchemaCustom = firstString(info.SchemaCustom, stringAt(schema, "custom"))
}

func allowedValues(v interface{}) []AllowedValue {
	out := []AllowedValue{}
	for _, item := range asArray(v) {
		m := asMap(item)
		if len(m) == 0 {
			continue
		}
		out = append(out, AllowedValue{
			ID:    stringAt(m, "id"),
			Name:  stringAt(m, "name"),
			Value: stringAt(m, "value"),
			Raw:   m,
		})
	}
	return out
}

func mergeRaw(existing, next map[string]interface{}) map[string]interface{} {
	if existing == nil {
		existing = map[string]interface{}{}
	}
	for k, v := range next {
		existing[k] = v
	}
	return existing
}

func asMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func asArray(v interface{}) []interface{} {
	if a, ok := v.([]interface{}); ok {
		return a
	}
	return nil
}

func mapsFromArray(items []interface{}) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, item := range items {
		if m := asMap(item); len(m) > 0 {
			out = append(out, m)
		}
	}
	return out
}

func stringAt(m map[string]interface{}, key string) string {
	if m == nil || m[key] == nil {
		return ""
	}
	return fmt.Sprint(m[key])
}

func boolAt(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func firstString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isBlockedTemplateField(id string) bool {
	switch id {
	case "key", "id", "self", "status", "resolution", "created", "updated", "creator", "reporter",
		"comment", "comments", "worklog", "watchers", "votes", "attachment", "attachments", "issuelinks",
		"subtasks", "aggregatetimespent", "aggregatetimeoriginalestimate", "aggregatetimeestimate",
		"timespent", "timeoriginalestimate", "timeestimate", "project", "issuetype":
		return true
	default:
		return strings.HasPrefix(id, "aggregate")
	}
}
