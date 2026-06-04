package manifest

import "strings"

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
