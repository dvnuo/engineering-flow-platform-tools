package manifest

import "strings"

type Registry struct {
	Version   int             `json:"version"`
	Templates []RegistryEntry `json:"templates"`
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
