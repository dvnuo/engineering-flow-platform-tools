package zephyr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/httpclient"
)

type MappedStatus struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type StatusDefinition struct {
	Name        string      `json:"name"`
	ID          int         `json:"id"`
	Description string      `json:"description,omitempty"`
	Color       string      `json:"color,omitempty"`
	Type        interface{} `json:"type,omitempty"`
}

type StatusCatalog struct {
	ExecutionStatuses []StatusDefinition `json:"execution_statuses"`
	StepStatuses      []StatusDefinition `json:"step_statuses"`
	Aliases           map[string]string  `json:"aliases"`
	Source            string             `json:"source"`
}

func NormalizeStatus(input string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(input))
	switch s {
	case "PASS", "PASSED":
		return "PASS", nil
	case "FAIL", "FAILED":
		return "FAIL", nil
	case "WIP":
		return "WIP", nil
	case "BLOCKED":
		return "BLOCKED", nil
	case "UNEXECUTED":
		return "UNEXECUTED", nil
	default:
		return "", NewError("invalid_zephyr_status", "unknown Zephyr execution status", "Run jira zephyr status list --json to inspect known statuses.", 400)
	}
}

func MapStatus(input string, statusMap map[string]int) (MappedStatus, error) {
	return mapStatusInDefinitions(input, statusDefinitionsFromMap(statusMap))
}

func MapStatusWithCatalog(input string, catalog StatusCatalog) (MappedStatus, error) {
	if mapped, err := mapStatusInDefinitions(input, catalog.ExecutionStatuses); err == nil {
		return mapped, nil
	}
	if mapped, err := mapStatusInDefinitions(input, catalog.StepStatuses); err == nil {
		return mapped, nil
	}
	return MappedStatus{}, invalidStatusError(input)
}

func ConfigStatusCatalog(cfg EffectiveConfig) StatusCatalog {
	defs := statusDefinitionsFromMap(cfg.StatusMap)
	return StatusCatalog{
		ExecutionStatuses: defs,
		StepStatuses:      defs,
		Aliases:           DefaultStatusAliases(),
		Source:            "config",
	}
}

func DiscoverStatusCatalog(client *Client, cfg EffectiveConfig, allowServer bool, verifyZephyr bool) (StatusCatalog, error) {
	fallback := ConfigStatusCatalog(cfg)
	if !allowServer {
		return fallback, nil
	}
	execRaw, execErr := client.Get("util/testExecutionStatus", nil)
	stepRaw, stepErr := client.Get("util/teststepExecutionStatus", nil)

	execStatuses := ParseStatusDefinitions(execRaw)
	stepStatuses := ParseStatusDefinitions(stepRaw)
	serverRead := execErr == nil && len(execStatuses) > 0 || stepErr == nil && len(stepStatuses) > 0
	if serverRead {
		if len(execStatuses) == 0 {
			execStatuses = fallback.ExecutionStatuses
		}
		if len(stepStatuses) == 0 {
			stepStatuses = fallback.StepStatuses
		}
		return StatusCatalog{
			ExecutionStatuses: execStatuses,
			StepStatuses:      stepStatuses,
			Aliases:           aliasesForStatuses(execStatuses, stepStatuses),
			Source:            "server",
		}, nil
	}

	for _, err := range []error{execErr, stepErr} {
		if err != nil && !isStatusEndpointUnavailable(err) {
			return StatusCatalog{}, err
		}
	}
	if verifyZephyr {
		if err := detectZephyrModule(client); err != nil {
			return StatusCatalog{}, err
		}
	}
	return fallback, nil
}

func ParseStatusDefinitions(raw interface{}) []StatusDefinition {
	defs := parseStatusDefinitions(raw)
	sort.SliceStable(defs, func(i, j int) bool {
		return strings.ToUpper(defs[i].Name) < strings.ToUpper(defs[j].Name)
	})
	return defs
}

func KnownStatusNames(defs []StatusDefinition) []string {
	out := make([]string, 0, len(defs))
	seen := map[string]bool{}
	for _, def := range defs {
		name := strings.ToUpper(strings.TrimSpace(def.Name))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func DefaultStatusAliases() map[string]string {
	return map[string]string{
		"PASSED": "PASS",
		"FAILED": "FAIL",
	}
}

func aliasesForStatuses(groups ...[]StatusDefinition) map[string]string {
	aliases := DefaultStatusAliases()
	for _, defs := range groups {
		for _, def := range defs {
			name := strings.ToUpper(strings.TrimSpace(def.Name))
			if name != "" {
				aliases[name] = name
			}
		}
	}
	return aliases
}

func statusDefinitionsFromMap(statusMap map[string]int) []StatusDefinition {
	names := KnownStatuses(statusMap)
	out := make([]StatusDefinition, 0, len(names))
	for _, name := range names {
		out = append(out, StatusDefinition{Name: name, ID: statusMap[name]})
	}
	return out
}

func mapStatusInDefinitions(input string, defs []StatusDefinition) (MappedStatus, error) {
	inputKey := strings.ToUpper(strings.TrimSpace(input))
	if inputKey == "" {
		return MappedStatus{}, invalidStatusError(input)
	}
	statuses := map[string]StatusDefinition{}
	for _, def := range defs {
		key := strings.ToUpper(strings.TrimSpace(def.Name))
		if key != "" {
			statuses[key] = def
		}
	}
	if alias, ok := DefaultStatusAliases()[inputKey]; ok {
		if def, ok := statuses[alias]; ok {
			return MappedStatus{Name: def.Name, ID: def.ID}, nil
		}
	}
	if def, ok := statuses[inputKey]; ok {
		return MappedStatus{Name: def.Name, ID: def.ID}, nil
	}
	if normalized, err := NormalizeStatus(input); err == nil {
		if def, ok := statuses[normalized]; ok {
			return MappedStatus{Name: def.Name, ID: def.ID}, nil
		}
	}
	return MappedStatus{}, invalidStatusError(input)
}

func invalidStatusError(input string) error {
	name := strings.ToUpper(strings.TrimSpace(input))
	if name == "" {
		name = input
	}
	return NewError("invalid_zephyr_status", "unknown Zephyr execution status: "+name, "Run jira zephyr status list --json to inspect known statuses.", 400)
}

func parseStatusDefinitions(raw interface{}) []StatusDefinition {
	switch v := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		out := make([]StatusDefinition, 0, len(v))
		for _, item := range v {
			out = append(out, parseStatusDefinitions(item)...)
		}
		return dedupeStatusDefinitions(out)
	case []map[string]interface{}:
		out := make([]StatusDefinition, 0, len(v))
		for _, item := range v {
			out = append(out, parseStatusDefinitions(item)...)
		}
		return dedupeStatusDefinitions(out)
	case map[string]interface{}:
		if def, ok := statusDefinitionFromMap(v); ok {
			return []StatusDefinition{def}
		}
		for _, key := range []string{"executionStatuses", "testExecutionStatuses", "stepStatuses", "teststepExecutionStatuses", "statuses", "status"} {
			if child, ok := v[key]; ok {
				if defs := parseStatusDefinitions(child); len(defs) > 0 {
					return defs
				}
			}
		}
		out := []StatusDefinition{}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if child, ok := v[key].(map[string]interface{}); ok {
				if def, ok := statusDefinitionFromMap(child); ok {
					if def.Name == "" {
						def.Name = key
					}
					out = append(out, def)
				}
			}
		}
		return dedupeStatusDefinitions(out)
	default:
		return nil
	}
}

func statusDefinitionFromMap(m map[string]interface{}) (StatusDefinition, bool) {
	name := firstString(m, "name", "statusName", "label")
	id, ok := firstInt(m, "id", "status", "value")
	if name == "" || !ok {
		return StatusDefinition{}, false
	}
	def := StatusDefinition{
		Name:        strings.TrimSpace(name),
		ID:          id,
		Description: firstString(m, "description", "desc"),
		Color:       firstString(m, "color", "colour"),
	}
	if t, ok := m["type"]; ok {
		def.Type = normalizeJSONScalar(t)
	}
	return def, true
}

func dedupeStatusDefinitions(defs []StatusDefinition) []StatusDefinition {
	out := []StatusDefinition{}
	seen := map[string]bool{}
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			continue
		}
		key := strings.ToUpper(name) + ":" + strconv.Itoa(def.ID)
		if seen[key] {
			continue
		}
		seen[key] = true
		def.Name = name
		out = append(out, def)
	}
	return out
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		s := strings.TrimSpace(fmt.Sprint(v))
		if s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func firstInt(m map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		if n, ok := interfaceInt(m[key]); ok {
			return n, true
		}
	}
	return 0, false
}

func interfaceInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		return int(n), err == nil
	case string:
		if strings.TrimSpace(x) == "" {
			return 0, false
		}
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	default:
		return 0, false
	}
}

func normalizeJSONScalar(v interface{}) interface{} {
	switch x := v.(type) {
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
		return x.String()
	default:
		return x
	}
}

func isStatusEndpointUnavailable(err error) bool {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
			return true
		}
	}
	return false
}

func detectZephyrModule(client *Client) error {
	if _, err := client.Get("moduleInfo", nil); err != nil {
		var httpErr *httpclient.HTTPError
		if errors.As(err, &httpErr) {
			switch httpErr.Status {
			case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
				return NewError("zephyr_not_detected", "Zephyr legacy ZAPI endpoint was not detected for this Jira instance.", "Confirm the app is installed and try jira zephyr doctor --project <PROJECT> --json.", 404)
			}
		}
		return err
	}
	return nil
}
