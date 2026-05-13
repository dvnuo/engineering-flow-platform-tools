package instance

import (
	"testing"

	"engineering-flow-platform-tools/internal/config"
)

func TestResolveJiraBrowse(t *testing.T) {
	p := config.ProductConfig{Instances: []config.InstanceConfig{{Name: "jira-main", BaseURL: "https://a.example.com"}}}
	r, err := Resolve(p, "", "https://a.example.com/browse/EFP-123", "jira")
	if err != nil || r.Instance.Name != "jira-main" || r.Entity.Attrs["key"] != "EFP-123" {
		t.Fatal("failed")
	}
}
func TestLongestPrefix(t *testing.T) {
	p := config.ProductConfig{Instances: []config.InstanceConfig{{Name: "main", BaseURL: "https://a.example.com"}, {Name: "ctx", BaseURL: "https://a.example.com/jira"}}}
	r, err := Resolve(p, "", "https://a.example.com/jira/browse/EFP-1", "jira")
	if err != nil || r.Instance.Name != "ctx" {
		t.Fatal("prefix")
	}
}
func TestConfluencePageID(t *testing.T) {
	p := config.ProductConfig{Instances: []config.InstanceConfig{{Name: "confluence-main", BaseURL: "https://c.example.com"}}}
	r, err := Resolve(p, "", "https://c.example.com/pages/viewpage.action?pageId=123456", "confluence")
	if err != nil || r.Entity.Attrs["id"] != "123456" {
		t.Fatal("confluence")
	}
}
func TestAmbiguous(t *testing.T) {
	p := config.ProductConfig{Instances: []config.InstanceConfig{{Name: "a", BaseURL: "https://x.example.com/a"}, {Name: "b", BaseURL: "https://x.example.com/a"}}}
	_, err := Resolve(p, "", "https://x.example.com/a/browse/EFP-1", "jira")
	if err == nil || err.Error() != "ambiguous_instance" {
		t.Fatal("expected ambiguous")
	}
}
