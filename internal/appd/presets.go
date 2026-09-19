package appd

import (
	"fmt"
	"strings"
)

// Preset names accepted by appd metric preset, in display order.
var PresetNames = []string{"bt-response-time", "bt-calls", "bt-errors", "tier-cpu", "node-heap"}

// ExpandPreset turns a preset plus its entity flags into the documented
// Controller metric path:
//
//	bt-response-time  Business Transaction Performance|Business Transactions|<tier>|<bt>|Average Response Time (ms)
//	bt-calls          Business Transaction Performance|Business Transactions|<tier>|<bt>|Calls per Minute
//	bt-errors         Business Transaction Performance|Business Transactions|<tier>|<bt>|Errors per Minute
//	tier-cpu          Application Infrastructure Performance|<tier>|Hardware Resources|CPU|%Busy
//	node-heap         Application Infrastructure Performance|<tier>|Individual Nodes|<node>|JVM|Memory:Heap|Used %
func ExpandPreset(preset, tier, bt, node string) (string, error) {
	preset = strings.ToLower(strings.TrimSpace(preset))
	tier, bt, node = strings.TrimSpace(tier), strings.TrimSpace(bt), strings.TrimSpace(node)
	switch preset {
	case "bt-response-time":
		return btMetricPath(preset, tier, bt, "Average Response Time (ms)")
	case "bt-calls":
		return btMetricPath(preset, tier, bt, "Calls per Minute")
	case "bt-errors":
		return btMetricPath(preset, tier, bt, "Errors per Minute")
	case "tier-cpu":
		if tier == "" {
			return "", fmt.Errorf("preset %s requires --tier", preset)
		}
		return "Application Infrastructure Performance|" + tier + "|Hardware Resources|CPU|%Busy", nil
	case "node-heap":
		if tier == "" || node == "" {
			return "", fmt.Errorf("preset %s requires --tier and --node", preset)
		}
		return "Application Infrastructure Performance|" + tier + "|Individual Nodes|" + node + "|JVM|Memory:Heap|Used %", nil
	case "":
		return "", fmt.Errorf("--preset is required; use one of %s", strings.Join(PresetNames, ", "))
	default:
		return "", fmt.Errorf("unknown preset %q; use one of %s", preset, strings.Join(PresetNames, ", "))
	}
}

func btMetricPath(preset, tier, bt, metric string) (string, error) {
	if tier == "" || bt == "" {
		return "", fmt.Errorf("preset %s requires --tier and --bt", preset)
	}
	return "Business Transaction Performance|Business Transactions|" + tier + "|" + bt + "|" + metric, nil
}
