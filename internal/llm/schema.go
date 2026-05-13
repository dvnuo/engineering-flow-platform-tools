package llm

func SchemaFor(m CommandMeta) map[string]interface{} {
	return map[string]interface{}{"command": m.Name, "version": 1, "input": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}, "output": map[string]interface{}{"type": "object", "envelope": true}}
}
