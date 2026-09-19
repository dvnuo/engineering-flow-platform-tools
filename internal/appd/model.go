package appd

import (
	"encoding/json"
	"strconv"
	"strings"
)

// SnapshotSummaryFields are the documented per-snapshot fields appd snapshot
// list keeps so list output stays bounded; snapshot get returns everything.
var SnapshotSummaryFields = []string{
	"requestGUID", "summary", "userExperience", "timeTakenInMilliSecs",
	"businessTransactionId", "applicationComponentId", "applicationComponentNodeId",
	"serverStartTime", "exitCalls", "errorDetails", "URL", "snapshotExitSequence",
}

// ListAny returns v as a JSON array, or nil when it is not one.
func ListAny(v any) []any {
	items, _ := v.([]any)
	return items
}

// FirstItem unwraps the single-element arrays the Controller returns for
// /tiers/{tier} and /nodes/{node}; objects are returned as-is.
func FirstItem(v any) (any, bool) {
	switch x := v.(type) {
	case []any:
		if len(x) == 0 {
			return nil, false
		}
		return x[0], true
	case nil:
		return nil, false
	default:
		return x, true
	}
}

// NumberString renders a JSON number, string, or integer as its canonical
// decimal text so ids can be compared regardless of the decoder used.
func NumberString(v any) string {
	switch x := v.(type) {
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case string:
		return strings.TrimSpace(x)
	default:
		return ""
	}
}

// FindByNameOrID returns the first object whose name matches key
// case-insensitively or whose id equals key.
func FindByNameOrID(items []any, key string) (map[string]any, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false
	}
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := m["name"].(string); strings.EqualFold(strings.TrimSpace(name), key) {
			return m, true
		}
		if NumberString(m["id"]) == key {
			return m, true
		}
	}
	return nil, false
}

// FilterByTier keeps the objects whose tierName matches tier
// case-insensitively or whose tierId equals tier; an empty tier keeps all.
func FilterByTier(items []any, tier string) []any {
	tier = strings.TrimSpace(tier)
	if tier == "" {
		return items
	}
	out := make([]any, 0, len(items))
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["tierName"].(string)
		if strings.EqualFold(strings.TrimSpace(name), tier) || NumberString(m["tierId"]) == tier {
			out = append(out, m)
		}
	}
	return out
}

// TrimSnapshot keeps only SnapshotSummaryFields of one snapshot object.
func TrimSnapshot(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(SnapshotSummaryFields))
	for _, field := range SnapshotSummaryFields {
		if value, present := m[field]; present {
			out[field] = value
		}
	}
	return out
}

// TrimSnapshots applies TrimSnapshot to every element.
func TrimSnapshots(items []any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, TrimSnapshot(item))
	}
	return out
}

// SplitCSV splits a comma-separated flag value, trimming blanks and dropping
// empty entries.
func SplitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// NormalizeEnumCSV upper-cases a comma-separated enum list (event types,
// severities, user experiences) and re-joins it without blanks.
func NormalizeEnumCSV(s string) string {
	parts := SplitCSV(s)
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i])
	}
	return strings.Join(parts, ",")
}
