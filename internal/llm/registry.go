package llm

type CommandMeta struct {
	Name        string   `json:"name"`
	Usage       string   `json:"usage"`
	Product     string   `json:"product"`
	Risk        string   `json:"risk"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Flags       []string `json:"flags"`
	Required    []string `json:"required"`
}

type Registry struct{ items map[string]CommandMeta }

func NewRegistry() *Registry               { return &Registry{items: map[string]CommandMeta{}} }
func (r *Registry) Register(m CommandMeta) { r.items[m.Name] = m }
func (r *Registry) List(product string) []CommandMeta {
	out := []CommandMeta{}
	for _, v := range r.items {
		if v.Product == product {
			out = append(out, v)
		}
	}
	return out
}
func (r *Registry) Get(name string) (CommandMeta, bool) { v, ok := r.items[name]; return v, ok }
