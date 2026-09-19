package appd

import (
	"encoding/json"
	"strings"
	"testing"
)

func jsonNumber(s string) json.Number { return json.Number(s) }

func TestExpandPreset(t *testing.T) {
	for _, tc := range []struct {
		preset, tier, bt, node string
		want                   string
	}{
		{"bt-response-time", "web", "/checkout", "", "Business Transaction Performance|Business Transactions|web|/checkout|Average Response Time (ms)"},
		{"BT-Calls", " web ", "/checkout", "", "Business Transaction Performance|Business Transactions|web|/checkout|Calls per Minute"},
		{"bt-errors", "web", "/checkout", "ignored", "Business Transaction Performance|Business Transactions|web|/checkout|Errors per Minute"},
		{"tier-cpu", "web", "", "", "Application Infrastructure Performance|web|Hardware Resources|CPU|%Busy"},
		{"node-heap", "web", "", "web-node-1", "Application Infrastructure Performance|web|Individual Nodes|web-node-1|JVM|Memory:Heap|Used %"},
	} {
		got, err := ExpandPreset(tc.preset, tc.tier, tc.bt, tc.node)
		if err != nil || got != tc.want {
			t.Fatalf("ExpandPreset(%q)=%q err=%v want %q", tc.preset, got, err, tc.want)
		}
	}
	for _, tc := range []struct {
		preset, tier, bt, node string
		msg                    string
	}{
		{"bt-response-time", "web", "", "", "--tier and --bt"},
		{"bt-calls", "", "/checkout", "", "--tier and --bt"},
		{"tier-cpu", "", "", "", "--tier"},
		{"node-heap", "web", "", "", "--tier and --node"},
		{"", "web", "", "", "--preset is required"},
		{"cpu", "web", "", "", "unknown preset"},
	} {
		_, err := ExpandPreset(tc.preset, tc.tier, tc.bt, tc.node)
		if err == nil || !strings.Contains(err.Error(), tc.msg) {
			t.Fatalf("ExpandPreset(%q) err=%v want substring %q", tc.preset, err, tc.msg)
		}
	}
	if len(PresetNames) != 5 {
		t.Fatalf("preset names: %v", PresetNames)
	}
}

func TestModelHelpers(t *testing.T) {
	items := []any{
		map[string]any{"id": jsonNumber("1"), "name": "ecommerce", "tierName": "web", "tierId": jsonNumber("10")},
		map[string]any{"id": 2.0, "name": "Payments", "tierName": "inventory", "tierId": 11.0},
		"not an object",
	}
	if m, ok := FindByNameOrID(items, "ECOMMERCE"); !ok || m["name"] != "ecommerce" {
		t.Fatalf("find by name: %#v %v", m, ok)
	}
	if m, ok := FindByNameOrID(items, "2"); !ok || m["name"] != "Payments" {
		t.Fatalf("find by float id: %#v %v", m, ok)
	}
	if _, ok := FindByNameOrID(items, "3"); ok {
		t.Fatal("unexpected match")
	}
	if _, ok := FindByNameOrID(items, ""); ok {
		t.Fatal("empty key must not match")
	}
	if got := FilterByTier(items, "WEB"); len(got) != 1 {
		t.Fatalf("filter by tier name: %#v", got)
	}
	if got := FilterByTier(items, "11"); len(got) != 1 {
		t.Fatalf("filter by tier id: %#v", got)
	}
	if got := FilterByTier(items, ""); len(got) != 3 {
		t.Fatalf("empty tier keeps all: %#v", got)
	}
	if v, ok := FirstItem([]any{}); ok || v != nil {
		t.Fatal("empty array should not unwrap")
	}
	if v, ok := FirstItem([]any{"a", "b"}); !ok || v != "a" {
		t.Fatal("first element expected")
	}
	if v, ok := FirstItem(map[string]any{"a": 1}); !ok || v == nil {
		t.Fatal("object passes through")
	}
	if _, ok := FirstItem(nil); ok {
		t.Fatal("nil should not unwrap")
	}
	trimmed, _ := TrimSnapshot(map[string]any{"requestGUID": "g", "callChain": "x", "exitCalls": []any{}, "errorDetails": nil}).(map[string]any)
	if _, present := trimmed["callChain"]; present || trimmed["requestGUID"] != "g" || len(trimmed) != 3 {
		t.Fatalf("trim: %#v", trimmed)
	}
	if got := TrimSnapshot("raw"); got != "raw" {
		t.Fatalf("non-object passthrough: %#v", got)
	}
	if got := NormalizeEnumCSV(" error, very_slow ,,stall"); got != "ERROR,VERY_SLOW,STALL" {
		t.Fatalf("csv normalize: %q", got)
	}
	if got := SplitCSV(" 1, ,2 "); len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("split csv: %v", got)
	}
	if NumberString(1000000.0) != "1000000" || NumberString(jsonNumber("42")) != "42" || NumberString(7) != "7" || NumberString(nil) != "" {
		t.Fatal("number string")
	}
}
