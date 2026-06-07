package authoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/visual/manifest"
)

const (
	AgentGuideFile   = "agent-guide.md"
	PanelGrammarFile = "panel-grammar.md"
	QualityRulesFile = "quality.rules.json"
)

type Guide struct {
	TemplateID string            `json:"template_id"`
	GuidePath  string            `json:"guide_path"`
	Available  bool              `json:"available"`
	Sections   map[string]string `json:"sections,omitempty"`
	Summary    []string          `json:"summary,omitempty"`
	Raw        string            `json:"raw_markdown,omitempty"`
}

type QualityRules struct {
	Schema             string           `json:"schema"`
	TemplateID         string           `json:"template_id"`
	Label              LabelRules       `json:"label"`
	RequiredForQuality []RuleDefinition `json:"required_for_quality"`
	SemanticRules      []RuleDefinition `json:"semantic_rules"`
	TemplateSpecific   map[string]any   `json:"template_specific"`
}

type LabelRules struct {
	MaxOverviewLabelChars      int  `json:"max_overview_label_chars"`
	MaxDetailLabelChars        int  `json:"max_detail_label_chars"`
	RequireSummaryForLongLabel bool `json:"require_summary_for_long_labels"`
}

type RuleDefinition struct {
	Path        string         `json:"path,omitempty"`
	Severity    string         `json:"severity,omitempty"`
	Code        string         `json:"code"`
	Description string         `json:"description,omitempty"`
	Suggestion  string         `json:"suggestion,omitempty"`
	AutoFixHint map[string]any `json:"auto_fix_hint,omitempty"`
}

func TemplateBaseDir(templateDir string, entry manifest.RegistryEntry) string {
	return filepath.Dir(filepath.Join(templateDir, filepath.Clean(entry.Path)))
}

func GuideRelPath(entry manifest.RegistryEntry) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Clean(entry.Path)), AgentGuideFile))
}

func GuidePath(templateDir string, entry manifest.RegistryEntry) string {
	return filepath.Join(TemplateBaseDir(templateDir, entry), AgentGuideFile)
}

func PanelGrammarRelPath(entry manifest.RegistryEntry) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Clean(entry.Path)), PanelGrammarFile))
}

func PanelGrammarPath(templateDir string, entry manifest.RegistryEntry) string {
	return filepath.Join(TemplateBaseDir(templateDir, entry), PanelGrammarFile)
}

func QualityRulesRelPath(entry manifest.RegistryEntry) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Clean(entry.Path)), QualityRulesFile))
}

func QualityRulesPath(templateDir string, entry manifest.RegistryEntry) string {
	return filepath.Join(TemplateBaseDir(templateDir, entry), QualityRulesFile)
}

func GuideAvailable(templateDir string, entry manifest.RegistryEntry) bool {
	info, err := os.Stat(GuidePath(templateDir, entry))
	return err == nil && !info.IsDir() && info.Size() > 0
}

func PanelGrammarAvailable(templateDir string, entry manifest.RegistryEntry) bool {
	info, err := os.Stat(PanelGrammarPath(templateDir, entry))
	return err == nil && !info.IsDir() && info.Size() > 0
}

func QualityRulesAvailable(templateDir string, entry manifest.RegistryEntry) bool {
	info, err := os.Stat(QualityRulesPath(templateDir, entry))
	return err == nil && !info.IsDir() && info.Size() > 0
}

func LoadGuide(templateDir string, entry manifest.RegistryEntry, includeRaw bool) (Guide, error) {
	path := GuidePath(templateDir, entry)
	guide := Guide{TemplateID: entry.ID, GuidePath: GuideRelPath(entry), Available: false, Sections: map[string]string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return guide, nil
		}
		return guide, err
	}
	text := strings.TrimSpace(string(raw))
	guide.Available = text != ""
	guide.Sections = ParseGuideSections(text)
	guide.Summary = GuideSummary(guide.Sections)
	if includeRaw {
		guide.Raw = text
	}
	return guide, nil
}

func LoadPanelGrammar(templateDir string, entry manifest.RegistryEntry, includeRaw bool) (Guide, error) {
	path := PanelGrammarPath(templateDir, entry)
	guide := Guide{TemplateID: entry.ID, GuidePath: PanelGrammarRelPath(entry), Available: false, Sections: map[string]string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return guide, nil
		}
		return guide, err
	}
	text := strings.TrimSpace(string(raw))
	guide.Available = text != ""
	guide.Sections = ParseGuideSections(text)
	guide.Summary = GuideSummary(guide.Sections)
	if includeRaw {
		guide.Raw = text
	}
	return guide, nil
}

func LoadQualityRules(templateDir string, entry manifest.RegistryEntry) (QualityRules, bool, string, error) {
	path := QualityRulesPath(templateDir, entry)
	var rules QualityRules
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultQualityRules(entry.ID), false, QualityRulesRelPath(entry), nil
		}
		return rules, false, QualityRulesRelPath(entry), err
	}
	if err := json.Unmarshal(raw, &rules); err != nil {
		return rules, true, QualityRulesRelPath(entry), err
	}
	if rules.TemplateID == "" {
		rules.TemplateID = entry.ID
	}
	if rules.Label.MaxOverviewLabelChars <= 0 {
		rules.Label.MaxOverviewLabelChars = 32
	}
	if rules.Label.MaxDetailLabelChars <= 0 {
		rules.Label.MaxDetailLabelChars = 64
	}
	if rules.TemplateSpecific == nil {
		rules.TemplateSpecific = map[string]any{}
	}
	return rules, true, QualityRulesRelPath(entry), nil
}

func defaultQualityRules(templateID string) QualityRules {
	return QualityRules{
		Schema:     "efp.visual.template_quality_rules.v1",
		TemplateID: templateID,
		Label: LabelRules{
			MaxOverviewLabelChars:      32,
			MaxDetailLabelChars:        64,
			RequireSummaryForLongLabel: true,
		},
		TemplateSpecific: map[string]any{},
	}
}

func ParseGuideSections(markdown string) map[string]string {
	sections := map[string]string{}
	current := "overview"
	var b strings.Builder
	flush := func() {
		value := strings.TrimSpace(b.String())
		if value != "" {
			sections[current] = value
		}
		b.Reset()
	}
	heading := regexp.MustCompile(`^##\s+(.+?)\s*$`)
	for _, line := range strings.Split(markdown, "\n") {
		if m := heading.FindStringSubmatch(line); len(m) == 2 {
			flush()
			current = sectionKey(m[1])
			continue
		}
		if strings.HasPrefix(line, "# ") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	flush()
	return sections
}

func GuideSummary(sections map[string]string) []string {
	keys := []string{"when_to_use_this_template", "semantic_model", "required_construction_rules", "recommended_fields", "visual_encoding_rules", "common_mistakes_to_avoid", "quality_checklist_before_render"}
	var out []string
	for _, key := range keys {
		value := strings.TrimSpace(sections[key])
		if value == "" {
			continue
		}
		line := firstContentLine(value)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func RequiredGuideSections() []string {
	return []string{
		"when_to_use_this_template",
		"semantic_model",
		"required_construction_rules",
		"recommended_fields",
		"visual_encoding_rules",
		"common_mistakes_to_avoid",
		"quality_checklist_before_render",
		"minimal_good_example",
	}
}

func MissingGuideSections(sections map[string]string) []string {
	var missing []string
	for _, key := range RequiredGuideSections() {
		if strings.TrimSpace(sections[key]) == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

func sectionKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func firstContentLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-0123456789. ")
		if line != "" {
			if len(line) > 180 {
				return line[:180]
			}
			return line
		}
	}
	return ""
}
