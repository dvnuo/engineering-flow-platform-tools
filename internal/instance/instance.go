package instance

import "engineering-flow-platform-tools/internal/config"

type ResolvedEntity struct {
	Type  string            `json:"type"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

type Result struct {
	Instance   config.InstanceConfig `json:"instance"`
	Entity     ResolvedEntity        `json:"entity"`
	Candidates []string              `json:"candidates,omitempty"`
}
