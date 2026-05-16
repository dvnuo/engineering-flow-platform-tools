package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
)

type Context struct {
	Cfg      config.RootConfig
	Inst     config.InstanceConfig
	Client   *httpclient.Client
	DryRun   bool
	Instance string
}

func NewContext(cfg config.RootConfig, explicit, entity string, dry bool) (*Context, error) {
	res, err := instance.Resolve(cfg.Jira, explicit, entity, "jira")
	if err != nil {
		return nil, err
	}
	cl, err := httpclient.New(res.Instance)
	if err != nil {
		return nil, err
	}
	return &Context{Cfg: cfg, Inst: res.Instance, Client: cl, DryRun: dry, Instance: res.Instance.Name}, nil
}

func ReadJSON(resp io.Reader) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := json.NewDecoder(resp).Decode(&out)
	if err != nil {
		return map[string]interface{}{"raw": ""}, nil
	}
	return out, nil
}

func ReadJSONValue(resp io.Reader) (interface{}, error) {
	var out interface{}
	err := json.NewDecoder(resp).Decode(&out)
	if err != nil {
		return map[string]interface{}{"raw": ""}, nil
	}
	return out, nil
}

func IssueKey(input string) string {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil {
			return input
		}
		for _, pattern := range []string{`/browse/([A-Z][A-Z0-9]+-\d+)`, `/rest/api/\d+/issue/([A-Z][A-Z0-9]+-\d+)`} {
			re := regexp.MustCompile(pattern)
			if m := re.FindStringSubmatch(u.Path); len(m) == 2 {
				return m[1]
			}
		}
		parts := strings.Split(strings.TrimRight(u.Path, "/"), "/")
		return parts[len(parts)-1]
	}
	return input
}

func DryRunData(method, path string, q map[string]string, body interface{}) map[string]interface{} {
	return map[string]interface{}{"dry_run": true, "method": method, "path": path, "query": q, "body": redactBody(body)}
}

func redactBody(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, v := range x {
			if isSecretKey(k) {
				out[k] = "***REDACTED***"
				continue
			}
			out[k] = redactBody(v)
		}
		return out
	case map[string]string:
		out := map[string]string{}
		for k, v := range x {
			if isSecretKey(k) {
				out[k] = "***REDACTED***"
				continue
			}
			out[k] = v
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, v := range x {
			out[i] = redactBody(v)
		}
		return out
	default:
		return v
	}
}

func isSecretKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "password") || strings.Contains(k, "api_key") || strings.Contains(k, "apikey") || strings.Contains(k, "token") || k == "authorization"
}

func RequireYes(yes bool) error {
	if !yes {
		return fmt.Errorf("invalid_args")
	}
	return nil
}
