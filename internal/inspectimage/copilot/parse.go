package copilot

import (
	"encoding/json"
	"strings"
)

type Parsed struct {
	Result   any
	Warnings []string
}

func ParseResponse(raw map[string]any) (Parsed, error) {
	if isRefusal(raw) {
		return Parsed{}, &APIError{Code: "safety_refusal", Message: "The model refused to inspect the image.", Hint: "Try a narrower prompt that asks for visible, non-sensitive details only.", Status: 400}
	}
	text := outputText(raw)
	if strings.TrimSpace(text) == "" {
		return Parsed{Result: map[string]any{"raw_text": ""}, Warnings: []string{"response_parse_failed"}}, nil
	}
	if obj, ok := parseJSONObject(text); ok {
		return Parsed{Result: obj}, nil
	}
	return Parsed{Result: map[string]any{"raw_text": text}, Warnings: []string{"response_parse_failed"}}, nil
}

func outputText(raw map[string]any) string {
	if s, ok := raw["output_text"].(string); ok {
		return s
	}
	var parts []string
	output, _ := raw["output"].([]any)
	for _, item := range output {
		m, _ := item.(map[string]any)
		content, _ := m["content"].([]any)
		for _, c := range content {
			cm, _ := c.(map[string]any)
			if s, ok := cm["text"].(string); ok {
				parts = append(parts, s)
			}
			if s, ok := cm["output_text"].(string); ok {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func parseJSONObject(text string) (map[string]any, bool) {
	s := strings.TrimSpace(text)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j >= i {
			s = s[i : j+1]
		}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil, false
	}
	return obj, true
}

func isRefusal(raw map[string]any) bool {
	if v, ok := raw["refusal"].(string); ok && v != "" {
		return true
	}
	text := strings.ToLower(outputText(raw))
	return strings.Contains(text, "i can't help") || strings.Contains(text, "i cannot help")
}
