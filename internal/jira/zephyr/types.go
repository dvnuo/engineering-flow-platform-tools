package zephyr

import (
	"fmt"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/config"
)

const (
	APIFamilyZAPILegacy = "zapi_legacy"
	DefaultRESTPath     = "/rest/zapi/latest"
	DefaultVersionID    = "-1"
	PluginKey           = "com.thed.zephyr.je"
)

type EffectiveConfig struct {
	Enabled          *bool
	APIFamily        string
	RESTPath         string
	DefaultVersionID string
	StatusMap        map[string]int
	StrictStatus     bool
}

type Error struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

func NewError(code, message, hint string, status int) *Error {
	if status == 0 {
		status = 400
	}
	return &Error{Code: code, Message: message, Hint: hint, Status: status}
}

func DefaultStatusMap() map[string]int {
	return map[string]int{
		"UNEXECUTED": -1,
		"PASS":       1,
		"FAIL":       2,
		"WIP":        3,
		"BLOCKED":    4,
	}
}

func EffectiveConfigFor(inst config.InstanceConfig) (EffectiveConfig, error) {
	cfg := inst.Zephyr
	out := EffectiveConfig{
		Enabled:          cfg.Enabled,
		APIFamily:        normalizeAPIFamily(cfg.APIFamily),
		RESTPath:         strings.TrimSpace(cfg.RESTPath),
		DefaultVersionID: strings.TrimSpace(cfg.DefaultVersionID),
		StatusMap:        DefaultStatusMap(),
	}
	if out.APIFamily == "" {
		out.APIFamily = APIFamilyZAPILegacy
	}
	if out.RESTPath == "" {
		out.RESTPath = DefaultRESTPath
	}
	if !strings.HasPrefix(out.RESTPath, "/") {
		out.RESTPath = "/" + out.RESTPath
	}
	out.RESTPath = "/" + strings.Trim(out.RESTPath, "/")
	if out.DefaultVersionID == "" {
		out.DefaultVersionID = DefaultVersionID
	}
	if cfg.StrictStatus != nil {
		out.StrictStatus = *cfg.StrictStatus
	}
	for k, v := range cfg.StatusMap {
		name := strings.ToUpper(strings.TrimSpace(k))
		if name == "" {
			continue
		}
		out.StatusMap[name] = v
	}
	switch out.APIFamily {
	case APIFamilyZAPILegacy:
		return out, nil
	case "auto":
		out.APIFamily = APIFamilyZAPILegacy
		return out, nil
	default:
		return out, NewError("zephyr_api_family_unknown", fmt.Sprintf("unsupported Zephyr api_family %q", cfg.APIFamily), "P0 supports legacy ZAPI through /rest/zapi/latest.", 400)
	}
}

func normalizeAPIFamily(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "", APIFamilyZAPILegacy:
		return v
	case "legacy", "zapi", "zapi-legacy":
		return APIFamilyZAPILegacy
	default:
		return v
	}
}

func KnownStatuses(statusMap map[string]int) []string {
	out := make([]string, 0, len(statusMap))
	for k := range statusMap {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
