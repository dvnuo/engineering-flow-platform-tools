package zephyr

import "testing"

func TestResolveURL(t *testing.T) {
	cases := []string{
		"https://jira.company.local/projects/EFP?selectedItem=com.thed.zephyr.je%3Azephyr-tests-page#test-summary-tab",
		"https://jira.company.local/projects/EFP?selectedItem=com.thed.zephyr.je:zephyr-tests-page#test-summary-tab",
	}
	for _, raw := range cases {
		got, err := ResolveURL(raw)
		if err != nil {
			t.Fatalf("ResolveURL(%q): %v", raw, err)
		}
		if got.Product != "jira" || got.Plugin != "zephyr" || got.ProjectKey != "EFP" || got.PluginKey != PluginKey || got.Page != "zephyr-tests-page" || got.Tab != "test-summary-tab" {
			t.Fatalf("unexpected parse: %#v", got)
		}
	}
	if _, err := ResolveURL("https://jira.company.local/projects/EFP?selectedItem=com.example:page"); err == nil {
		t.Fatal("expected non-Zephyr URL to fail")
	}
}
