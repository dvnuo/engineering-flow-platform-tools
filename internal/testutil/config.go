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

// NexusConfig is a single-instance nexus config with basic password auth and a
// secret canary; rest_path is left empty to exercise the /service/rest/v1 default.
func NexusConfig(base string) string {
	return fmt.Sprintf("nexus:\n  default_instance: local\n  instances:\n    - name: local\n      base_url: %s\n      auth:\n        type: basic_password\n        username: ci-reader\n        password: secret-password-should-not-appear\n", base)
}

// NexusAnonymousConfig is a single-instance nexus config without an auth block.
func NexusAnonymousConfig(base string) string {
	return fmt.Sprintf("nexus:\n  default_instance: public\n  instances:\n    - name: public\n      base_url: %s\n", base)
}
