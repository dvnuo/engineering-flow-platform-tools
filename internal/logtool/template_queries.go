package logtool

import (
	"sort"
	"strings"
)

func TemplateGet(runDir, templateID string) (Template, error) {
	if strings.TrimSpace(templateID) == "" {
		return Template{}, NewError("invalid_args", "--template is required.", "Pass --template <template-id>.", 400)
	}
	templates, err := ReadTemplates(runDir)
	if err != nil {
		return Template{}, err
	}
	for _, tpl := range templates {
		if tpl.TemplateID == templateID {
			return tpl, nil
		}
	}
	return Template{}, NewError("not_found", "Template was not found in this run.", "Run log template list <run> --json to list template ids.", 404)
}

func TemplateEntries(runDir, templateID string, opts EntryListOptions) (EntryListResult, error) {
	if strings.TrimSpace(templateID) == "" {
		return EntryListResult{}, NewError("invalid_args", "--template is required.", "Pass --template <template-id>.", 400)
	}
	opts.TemplateID = templateID
	return Entries(runDir, opts)
}

func TemplateVariables(runDir, templateID string, limit int) (TemplateVariablesResult, error) {
	tpl, err := TemplateGet(runDir, templateID)
	if err != nil {
		return TemplateVariablesResult{}, err
	}
	n, err := normalizeLimit(limit, defaultListLimit)
	if err != nil {
		return TemplateVariablesResult{}, err
	}
	positions := map[int]map[string]int{}
	err = ReadEntries(runDir, func(entry Entry) error {
		if entry.TemplateID != templateID {
			return nil
		}
		for i, value := range entry.Variables {
			value = Redact(strings.TrimSpace(value))
			if value == "" {
				continue
			}
			pos := i + 1
			if positions[pos] == nil {
				positions[pos] = map[string]int{}
			}
			positions[pos][value]++
		}
		return nil
	})
	if err != nil {
		return TemplateVariablesResult{}, err
	}
	keys := make([]int, 0, len(positions))
	for pos := range positions {
		keys = append(keys, pos)
	}
	sort.Ints(keys)
	result := TemplateVariablesResult{TemplateID: tpl.TemplateID, Template: tpl.Template}
	for _, pos := range keys {
		samples := variableSamples(positions[pos], n)
		result.Variables = append(result.Variables, TemplateVariable{
			Position:  pos,
			TypeGuess: guessVariableType(samples),
			TopValues: samples,
		})
	}
	return result, nil
}

func variableSamples(counts map[string]int, limit int) []VariableSample {
	samples := make([]VariableSample, 0, len(counts))
	for value, count := range counts {
		samples = append(samples, VariableSample{Value: value, Count: count})
	}
	sort.SliceStable(samples, func(i, j int) bool {
		if samples[i].Count == samples[j].Count {
			return samples[i].Value < samples[j].Value
		}
		return samples[i].Count > samples[j].Count
	})
	if len(samples) > limit {
		samples = samples[:limit]
	}
	return samples
}

func guessVariableType(samples []VariableSample) string {
	if len(samples) == 0 {
		return ""
	}
	value := samples[0].Value
	switch {
	case strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">"):
		return strings.Trim(value, "<>")
	case strings.HasSuffix(value, "ms") || strings.HasSuffix(value, "s") || strings.HasSuffix(value, "m") || strings.HasSuffix(value, "h"):
		return "duration"
	default:
		return "value"
	}
}
