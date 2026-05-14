package jira

import (
	"encoding/json"
	"fmt"
	"io"
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

func IssueKey(input string) string {
	if strings.HasPrefix(input, "http") {
		parts := strings.Split(strings.TrimRight(input, "/"), "/")
		return parts[len(parts)-1]
	}
	return input
}

func DryRunData(method, path string, q map[string]string, body interface{}) map[string]interface{} {
	return map[string]interface{}{"dry_run": true, "method": method, "path": path, "query": q, "body": body}
}

func RequireYes(yes bool) error { if !yes { return fmt.Errorf("invalid_args") }; return nil }
