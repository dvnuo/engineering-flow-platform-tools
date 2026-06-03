package testutil

import (
	"fmt"
	"os"
)

func WriteConfig(content string) (string, error) {
	f, err := os.CreateTemp("", "efpt-config-*.yaml")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func JiraConfig(base string) string {
	return fmt.Sprintf("jira:\n  default_instance: local\n  instances:\n    - name: local\n      base_url: %s\n      rest_path: /rest/api/2\n      api_version: \"2\"\n      auth:\n        type: pat\n        token: secret-token-should-not-appear\nconfluence:\n  default_instance: local\n  instances:\n    - name: local\n      base_url: %s\n      rest_path: /rest/api\n      api_version: \"\"\n      auth:\n        type: pat\n        token: secret-token-should-not-appear\n", base, base)
}

func JenkinsConfig(base string) string {
	return fmt.Sprintf("jenkins:\n  default_instance: local\n  instances:\n    - name: local\n      base_url: %s\n      crumb_mode: auto\n      auth:\n        type: pat\n        token: secret-token-should-not-appear\n", base)
}
