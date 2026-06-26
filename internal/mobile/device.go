package mobile

import (
	"sort"
	"strconv"
	"strings"
)

type DeviceInfo struct {
	OS         string `json:"os"`
	OSVersion  string `json:"os_version"`
	Name       string `json:"device"`
	RealMobile bool   `json:"real_mobile"`
	DeviceTier string `json:"device_tier,omitempty"`
	GroupUsage int    `json:"group_usage,omitempty"`
}

type DeviceQuery struct {
	Platform     string
	OSVersion    string
	MinOSVersion string
	Manufacturer string
	Name         string
	RealOnly     bool
	Tier         string
	Strategy     string
}

type DeviceResolveResult struct {
	Recommended  DeviceSelection   `json:"recommended"`
	Alternatives []DeviceSelection `json:"alternatives"`
	Reasoning    []string          `json:"reasoning"`
	Restrictions []string          `json:"restrictions,omitempty"`
}

func ResolveDevice(devices []DeviceInfo, q DeviceQuery) (DeviceResolveResult, error) {
	if q.Strategy == "" {
		q.Strategy = "latest-compatible"
	}
	var filtered []DeviceInfo
	for _, d := range devices {
		if q.Platform != "" && !strings.EqualFold(d.OS, q.Platform) {
			continue
		}
		if q.Name != "" && !strings.EqualFold(d.Name, q.Name) {
			continue
		}
		if q.Manufacturer != "" && !strings.Contains(strings.ToLower(d.Name), strings.ToLower(q.Manufacturer)) {
			continue
		}
		if q.OSVersion != "" && !versionEqual(d.OSVersion, q.OSVersion) {
			continue
		}
		if q.MinOSVersion != "" && compareVersion(d.OSVersion, q.MinOSVersion) < 0 {
			continue
		}
		if q.RealOnly && !d.RealMobile {
			continue
		}
		if q.Tier != "" && !strings.EqualFold(d.DeviceTier, q.Tier) {
			continue
		}
		filtered = append(filtered, d)
	}
	if len(filtered) == 0 {
		return DeviceResolveResult{}, NewError("device_not_supported", "no BrowserStack device matched the requested filters", "Relax device filters or run mobile-auto device list --json.", 404)
	}
	sortDevices(filtered, q.Strategy)
	choices := make([]DeviceSelection, 0, len(filtered))
	for _, d := range filtered {
		choices = append(choices, DeviceSelection{Name: d.Name, OS: d.OS, OSVersion: d.OSVersion, Tier: d.DeviceTier, Reason: q.Strategy})
	}
	reasons := []string{"filtered BrowserStack devices deterministically"}
	if q.Strategy == "exact" {
		reasons = append(reasons, "exact strategy preserved exact name/version matches only")
	}
	if strings.Contains(q.Strategy, "latest") {
		reasons = append(reasons, "latest-compatible sorts by OS version descending, then device name")
	}
	return DeviceResolveResult{Recommended: choices[0], Alternatives: choices[1:], Reasoning: reasons}, nil
}

func sortDevices(devices []DeviceInfo, strategy string) {
	sort.SliceStable(devices, func(i, j int) bool {
		a, b := devices[i], devices[j]
		switch strategy {
		case "representative":
			if a.OS != b.OS {
				return a.OS < b.OS
			}
			if a.DeviceTier != b.DeviceTier {
				return a.DeviceTier < b.DeviceTier
			}
			return a.Name < b.Name
		case "least-used":
			if a.GroupUsage != b.GroupUsage {
				return a.GroupUsage < b.GroupUsage
			}
		case "fallback":
			if a.RealMobile != b.RealMobile {
				return a.RealMobile
			}
		default:
			cmp := compareVersion(a.OSVersion, b.OSVersion)
			if cmp != 0 {
				return cmp > 0
			}
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.OSVersion < b.OSVersion
	})
}

func versionEqual(a, b string) bool {
	return compareVersion(a, b) == 0
}

func compareVersion(a, b string) int {
	aa := splitVersion(a)
	bb := splitVersion(b)
	n := len(aa)
	if len(bb) > n {
		n = len(bb)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func splitVersion(v string) []int {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' || r == '_' || r == ' ' })
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}
