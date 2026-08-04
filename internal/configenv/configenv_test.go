package configenv

import (
	"errors"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveExactReferences(t *testing.T) {
	value := struct {
		Portable string            `yaml:"portable"`
		Windows  string            `yaml:"windows"`
		Literal  string            `yaml:"literal"`
		Values   map[string]string `yaml:"values"`
	}{
		Portable: "${PORTABLE_VALUE}",
		Windows:  "%WINDOWS_VALUE%",
		Literal:  "prefix-${PORTABLE_VALUE}",
		Values:   map[string]string{"nested": "${NESTED_VALUE}"},
	}
	lookup := func(name string) (string, bool) {
		values := map[string]string{
			"PORTABLE_VALUE": "portable",
			"WINDOWS_VALUE":  "windows",
			"NESTED_VALUE":   "nested",
		}
		resolved, ok := values[name]
		return resolved, ok
	}
	resolution, err := Resolve(&value, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if resolution == nil || value.Portable != "portable" || value.Windows != "windows" || value.Values["nested"] != "nested" {
		t.Fatalf("references were not resolved: %#v", value)
	}
	if value.Literal != "prefix-${PORTABLE_VALUE}" {
		t.Fatalf("partial reference was changed: %q", value.Literal)
	}
}

func TestResolveRejectsMissingReference(t *testing.T) {
	value := "${MISSING_VALUE}"
	_, err := Resolve(&value, func(string) (string, bool) { return "", false })
	var missing *MissingEnvReferenceError
	if !errors.As(err, &missing) || missing.Name != "MISSING_VALUE" {
		t.Fatalf("expected missing reference error, got %v", err)
	}
}

func TestMergeTopLevelMatchesNamedSequenceItems(t *testing.T) {
	type item struct {
		Name   string `yaml:"name"`
		URL    string `yaml:"url"`
		Secret string `yaml:"secret"`
	}
	type tool struct {
		Items []item `yaml:"items"`
	}
	type root struct {
		Tool tool `yaml:"tool"`
	}

	raw := []byte(`tool:
  items:
    - name: a
      url: https://a.example.test
      secret: "${SECRET_A}"
    - name: b
      url: https://b.example.test
      secret: "%SECRET_B%"
`)
	var effective root
	if err := yaml.Unmarshal(raw, &effective); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"SECRET_A": "secret-a", "SECRET_B": "secret-b"}
	resolution, err := Resolve(&effective, func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Capture(effective, resolution)
	if err != nil {
		t.Fatal(err)
	}
	effective.Tool.Items[1].URL = "https://new-b.example.test"
	effective.Tool.Items[0], effective.Tool.Items[1] = effective.Tool.Items[1], effective.Tool.Items[0]

	oldRoot := mustDocumentRoot(t, raw)
	newBytes, err := yaml.Marshal(effective)
	if err != nil {
		t.Fatal(err)
	}
	newRoot := mustDocumentRoot(t, newBytes)
	merged := MergeTopLevel(mappingValue(oldRoot, "tool"), mappingValue(newRoot, "tool"), snapshot, "tool")
	var saved tool
	if err := merged.Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Items) != 2 || saved.Items[0].Name != "b" || saved.Items[0].Secret != "%SECRET_B%" || saved.Items[0].URL != "https://new-b.example.test" {
		t.Fatalf("item b was not merged by name: %#v", saved.Items)
	}
	if saved.Items[1].Name != "a" || saved.Items[1].Secret != "${SECRET_A}" {
		t.Fatalf("item a was not merged by name: %#v", saved.Items)
	}
}

func mustDocumentRoot(t *testing.T, data []byte) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) == 0 {
		t.Fatal("empty YAML document")
	}
	return doc.Content[0]
}
