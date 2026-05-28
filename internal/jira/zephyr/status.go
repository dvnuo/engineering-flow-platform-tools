package zephyr

import "strings"

type MappedStatus struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
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
	name, err := NormalizeStatus(input)
	if err != nil {
		return MappedStatus{}, err
	}
	id, ok := statusMap[name]
	if !ok {
		return MappedStatus{}, NewError("invalid_zephyr_status", "Zephyr status is not configured: "+name, "Run jira zephyr status list --json to inspect known statuses.", 400)
	}
	return MappedStatus{Name: name, ID: id}, nil
}
