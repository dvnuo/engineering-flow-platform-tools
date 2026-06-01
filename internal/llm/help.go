package llm

func Help(product string) map[string]interface{} {
	return map[string]interface{}{"product": product, "tips": []string{"For agents, default every command and subcommand to --json", "Prefer --instance when multiple instances are configured", "Check error.code/error.hint"}}
}
