package llm

func Help(product string) map[string]interface{} {
	return map[string]interface{}{"product": product, "tips": []string{"Use --json", "Prefer --instance", "Check error.code/error.hint"}}
}
