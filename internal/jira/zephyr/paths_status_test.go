package zephyr

import "testing"

func TestZAPIAndRawPath(t *testing.T) {
	if got := ZAPI("/rest/zapi/latest", "cycle"); got != "/rest/zapi/latest/cycle" {
		t.Fatalf("ZAPI cycle = %q", got)
	}
	if got, err := RawPath("/rest/zapi/latest", "execution/123"); err != nil || got != "/rest/zapi/latest/execution/123" {
		t.Fatalf("RawPath relative = %q, %v", got, err)
	}
	if got, err := RawPath("/rest/zapi/latest", "/rest/zapi/latest/cycle"); err != nil || got != "/rest/zapi/latest/cycle" {
		t.Fatalf("RawPath absolute = %q, %v", got, err)
	}
	if _, err := RawPath("/rest/zapi/latest", "https://evil.example/rest/zapi/latest/cycle"); err == nil {
		t.Fatal("expected external raw URL to be blocked")
	}
	if _, err := RawPath("/rest/zapi/latest", "../rest/api/2/myself"); err == nil {
		t.Fatal("expected parent path segment to be blocked")
	}
}

func TestStatusNormalizeAndMap(t *testing.T) {
	aliases := map[string]string{
		"pass":       "PASS",
		"passed":     "PASS",
		"FAIL":       "FAIL",
		"failed":     "FAIL",
		"wip":        "WIP",
		"blocked":    "BLOCKED",
		"unexecuted": "UNEXECUTED",
	}
	for in, want := range aliases {
		got, err := NormalizeStatus(in)
		if err != nil || got != want {
			t.Fatalf("NormalizeStatus(%q) = %q, %v want %q", in, got, err, want)
		}
	}
	mapped, err := MapStatus("passed", DefaultStatusMap())
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Name != "PASS" || mapped.ID != 1 {
		t.Fatalf("bad mapping: %#v", mapped)
	}
	if _, err := MapStatus("skipped", DefaultStatusMap()); err == nil {
		t.Fatal("expected unknown status error")
	}
}
