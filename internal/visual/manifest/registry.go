package manifest

import "strings"

type Registry struct {
	Version   int             `json:"version"`
	Templates []RegistryEntry `json:"templates"`
}

type RegistryEntry struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description"`
	InputSchema string `json:"input_schema"`
	Renderer    string `json:"renderer"`
}

func (r Registry) Find(id string) (RegistryEntry, bool) {
	id = strings.TrimSpace(id)
	for _, entry := range r.Templates {
		if entry.ID == id {
			return entry, true
		}
	}
	return RegistryEntry{}, false
}
