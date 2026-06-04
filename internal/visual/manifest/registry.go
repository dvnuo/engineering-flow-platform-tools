package manifest

import (
	"os"
	"path"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/visual/metadata"
)

type Registry struct {
	Version   int              `json:"version"`
	Expected  RegistryExpected `json:"expected,omitempty"`
	Templates []RegistryEntry  `json:"templates"`
}

type RegistryExpected struct {
	CanonicalCount int            `json:"canonical_count"`
	Categories     map[string]int `json:"categories,omitempty"`
}

type RegistryEntry struct {
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	Category        string   `json:"category"`
	Path            string   `json:"path"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	InputSchema     string   `json:"input_schema,omitempty"`
	InputSchemaKind string   `json:"input_schema_kind"`
	Renderer        string   `json:"renderer"`
	LayoutPreset    string   `json:"layout_preset"`
	Tags            []string `json:"tags"`
	Aliases         []string `json:"aliases,omitempty"`
}

func (r Registry) Find(id string) (RegistryEntry, bool) {
	entry, _, ok := r.Resolve(id)
	return entry, ok
}

func (r Registry) Resolve(id string) (RegistryEntry, string, bool) {
	id = strings.TrimSpace(id)
	for _, entry := range r.Templates {
		if entry.ID == id {
			return entry, entry.ID, true
		}
		for _, alias := range entry.Aliases {
			if strings.TrimSpace(alias) == id {
				return entry, alias, true
			}
		}
	}
	return RegistryEntry{}, "", false
}

func (r Registry) CanonicalCount() int {
	return len(r.Templates)
}

func (r Registry) AliasCount() int {
	count := 0
	for _, entry := range r.Templates {
		count += len(entry.Aliases)
	}
	return count
}

func (r Registry) TotalCount() int {
	return r.CanonicalCount() + r.AliasCount()
}

func (r Registry) CategoryCounts() map[string]int {
	counts := map[string]int{}
	for _, entry := range r.Templates {
		counts[entry.Category]++
	}
	return counts
}

func (r Registry) CanonicalTemplateDirs() []string {
	dirs := map[string]bool{}
	for _, entry := range r.Templates {
		clean := path.Clean(strings.ReplaceAll(entry.Path, "\\", "/"))
		dir := path.Dir(clean)
		if dir != "." && dir != "/" {
			dirs[dir] = true
		}
	}
	out := make([]string, 0, len(dirs))
	for dir := range dirs {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}

func (r Registry) OrphanTemplateDirs(templateDir string) ([]string, error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, metadata.NewError(
			"template_registry_invalid",
			"visual template directory could not be read: "+err.Error(),
			"Pass --template-dir pointing to a readable templates/visual directory.",
			400,
		)
	}
	canonical := map[string]bool{}
	for _, dir := range r.CanonicalTemplateDirs() {
		canonical[dir] = true
	}
	orphan := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "_shared" {
			continue
		}
		if !canonical[name] {
			orphan = append(orphan, name)
		}
	}
	sort.Strings(orphan)
	return orphan, nil
}

func (r Registry) EffectiveExpected() (RegistryExpected, []string) {
	expected := RegistryExpected{
		CanonicalCount: r.Expected.CanonicalCount,
		Categories:     copyCategoryCounts(r.Expected.Categories),
	}
	var warnings []string
	if expected.CanonicalCount == 0 && len(expected.Categories) == 0 {
		warnings = append(warnings, "registry.expected is missing; using default visual template counts.")
		expected = DefaultRegistryExpected()
		return expected, warnings
	}
	if expected.CanonicalCount == 0 {
		warnings = append(warnings, "registry.expected.canonical_count is missing; using default canonical count.")
		expected.CanonicalCount = DefaultExpectedCanonicalCount
	}
	if len(expected.Categories) == 0 {
		warnings = append(warnings, "registry.expected.categories is missing; using default category counts.")
		expected.Categories = copyCategoryCounts(ExpectedCategoryCounts)
	}
	return expected, warnings
}

func DefaultRegistryExpected() RegistryExpected {
	return RegistryExpected{
		CanonicalCount: DefaultExpectedCanonicalCount,
		Categories:     copyCategoryCounts(ExpectedCategoryCounts),
	}
}

func copyCategoryCounts(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
