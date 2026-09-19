package commands

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	for _, key := range []string{"HOME", "USERPROFILE"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "."
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}

// awsCredentialsFilePath honours AWS_SHARED_CREDENTIALS_FILE like the AWS CLI,
// saml2aws, and the opencode runtime (which points it into its state dir).
func awsCredentialsFilePath() string {
	if p := strings.TrimSpace(os.Getenv("AWS_SHARED_CREDENTIALS_FILE")); p != "" {
		return p
	}
	return filepath.Join(homeDir(), ".aws", "credentials")
}

func awsConfigFilePath() string {
	if p := strings.TrimSpace(os.Getenv("AWS_CONFIG_FILE")); p != "" {
		return p
	}
	return filepath.Join(homeDir(), ".aws", "config")
}

// iniSection is one [section] of an AWS credentials/config file with key
// order preserved so rewrites stay diff-friendly.
type iniSection struct {
	Name   string
	Keys   []string
	Values map[string]string
}

func parseINI(text string) []iniSection {
	var sections []iniSection
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, iniSection{Name: strings.TrimSpace(line[1 : len(line)-1]), Values: map[string]string{}})
			continue
		}
		if len(sections) == 0 {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		current := &sections[len(sections)-1]
		key = strings.TrimSpace(key)
		if _, exists := current.Values[key]; !exists {
			current.Keys = append(current.Keys, key)
		}
		current.Values[key] = strings.TrimSpace(value)
	}
	return sections
}

func readINI(path string) ([]iniSection, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseINI(string(b)), nil
}

func renderINI(sections []iniSection) string {
	var sb strings.Builder
	for i, section := range sections {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[" + section.Name + "]\n")
		for _, key := range section.Keys {
			sb.WriteString(key + " = " + section.Values[key] + "\n")
		}
	}
	return sb.String()
}

func writeINI(path string, sections []iniSection) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(renderINI(sections)), 0o600)
}

func upsertINISection(sections []iniSection, name string, keys []string, values map[string]string) []iniSection {
	for i := range sections {
		if sections[i].Name != name {
			continue
		}
		if sections[i].Values == nil {
			sections[i].Values = map[string]string{}
		}
		for _, key := range keys {
			if _, exists := sections[i].Values[key]; !exists {
				sections[i].Keys = append(sections[i].Keys, key)
			}
			sections[i].Values[key] = values[key]
		}
		return sections
	}
	section := iniSection{Name: name, Values: map[string]string{}}
	for _, key := range keys {
		section.Keys = append(section.Keys, key)
		section.Values[key] = values[key]
	}
	return append(sections, section)
}

func findINISection(sections []iniSection, name string) (iniSection, bool) {
	for _, section := range sections {
		if section.Name == name {
			return section, true
		}
	}
	return iniSection{}, false
}

// awsConfigSectionName maps a profile name onto the AWS config file's naming
// convention: [default] for the default profile, [profile <name>] otherwise.
func awsConfigSectionName(profile string) string {
	if profile == "default" {
		return "default"
	}
	return "profile " + profile
}

func upsertAWSConfigProfile(path, profile string, keys []string, values map[string]string) error {
	sections, err := readINI(path)
	if err != nil {
		return err
	}
	return writeINI(path, upsertINISection(sections, awsConfigSectionName(profile), keys, values))
}

// expiryKeys are the keys ADFS tools write next to temporary credentials:
// saml2aws writes x_security_token_expires; other tools use aws_expiration or
// expiration.
var expiryKeys = []string{"x_security_token_expires", "aws_expiration", "aws_session_expiration", "expiration", "expires", "x_expiration"}

var expiryLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05 -0700 MST",
	"2006-01-02 15:04:05",
}

func parseExpiry(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range expiryLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	if isDigits(raw) {
		if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
			if seconds > 1e12 {
				seconds /= 1000
			}
			return time.Unix(seconds, 0), true
		}
	}
	return time.Time{}, false
}

func sectionExpiry(section iniSection) (time.Time, bool) {
	for _, key := range expiryKeys {
		if value, ok := section.Values[key]; ok {
			if t, ok := parseExpiry(value); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func profileExpiryFromFile(path, profile string) (time.Time, bool) {
	sections, err := readINI(path)
	if err != nil {
		return time.Time{}, false
	}
	section, ok := findINISection(sections, profile)
	if !ok {
		return time.Time{}, false
	}
	return sectionExpiry(section)
}
