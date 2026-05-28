package zephyr

import (
	"errors"
	"fmt"
	"net/http"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/jira"
)

type DoctorResult struct {
	Product          string                 `json:"product"`
	Detected         bool                   `json:"detected"`
	APIFamily        string                 `json:"api_family"`
	PluginKey        string                 `json:"plugin_key"`
	BasePath         string                 `json:"base_path"`
	ProjectKey       string                 `json:"project_key"`
	ProjectID        string                 `json:"project_id"`
	DefaultVersionID string                 `json:"default_version_id"`
	TestIssueType    interface{}            `json:"test_issue_type"`
	StatusMap        map[string]int         `json:"status_map"`
	ServerInfo       map[string]interface{} `json:"server_info,omitempty"`
	CycleProbe       interface{}            `json:"cycle_probe,omitempty"`
}

func Doctor(ctx *jira.Context, client *Client, cfg EffectiveConfig, projectKey string) (DoctorResult, error) {
	if projectKey == "" {
		projectKey = ctx.Inst.DefaultProject
	}
	if projectKey == "" {
		return DoctorResult{}, NewError("invalid_args", "--project required", "Use jira zephyr doctor --project <PROJECT> --json.", 400)
	}
	serverInfo, err := getMap(ctx, "/rest/api/2/serverInfo", nil)
	if err != nil {
		return DoctorResult{}, err
	}
	project, err := getMap(ctx, "/rest/api/2/project/"+PathEscape(projectKey), nil)
	if err != nil {
		return DoctorResult{}, mapProjectError(projectKey, err)
	}
	projectID := fmt.Sprint(project["id"])
	if projectID == "" || projectID == "<nil>" {
		return DoctorResult{}, NewError("zephyr_project_unresolved", "Jira project did not include an id", "Run jira project get "+projectKey+" --json and verify the project exists.", 404)
	}
	testIssueType, err := client.Get("util/zephyrTestIssueType", nil)
	if err != nil {
		return DoctorResult{}, mapDetectError(err)
	}
	cycleProbe, err := client.Get("cycle", map[string]string{"projectId": projectID, "versionId": cfg.DefaultVersionID})
	if err != nil {
		return DoctorResult{}, mapDetectError(err)
	}
	return DoctorResult{
		Product:          "zephyr",
		Detected:         true,
		APIFamily:        cfg.APIFamily,
		PluginKey:        PluginKey,
		BasePath:         cfg.RESTPath,
		ProjectKey:       projectKey,
		ProjectID:        projectID,
		DefaultVersionID: cfg.DefaultVersionID,
		TestIssueType:    testIssueType,
		StatusMap:        cfg.StatusMap,
		ServerInfo:       serverInfo,
		CycleProbe:       cycleProbe,
	}, nil
}

func getMap(ctx *jira.Context, p string, q map[string]string) (map[string]interface{}, error) {
	resp, err := ctx.Client.Do(httpclient.Request{Method: http.MethodGet, Path: p, Query: q})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	v, _ := ReadJSONValue(resp.Body)
	m, _ := v.(map[string]interface{})
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

func mapProjectError(projectKey string, err error) error {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Status == http.StatusUnauthorized || httpErr.Status == http.StatusForbidden {
			return NewError("zephyr_permission_denied", "Jira project cannot be read with the selected credentials", "Verify Jira project permissions before probing Zephyr.", httpErr.Status)
		}
		if httpErr.Status == http.StatusNotFound {
			return NewError("zephyr_project_unresolved", "Jira project could not be resolved: "+projectKey, "Run jira project get "+projectKey+" --json to verify the project key.", 404)
		}
		if httpErr.Status == http.StatusTooManyRequests {
			return NewError("zephyr_rate_limited", "Jira rate limited the Zephyr doctor probe", "Retry later or reduce probe frequency.", 429)
		}
	}
	return err
}

func mapDetectError(err error) error {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return NewError("zephyr_permission_denied", "Zephyr API cannot be read with the selected credentials", "Verify Zephyr project permissions for the selected Jira account.", httpErr.Status)
		case http.StatusTooManyRequests:
			return NewError("zephyr_rate_limited", "Zephyr API rate limited the request", "Retry later or reduce request frequency.", http.StatusTooManyRequests)
		case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
			return NewError("zephyr_not_detected", "Zephyr legacy ZAPI endpoint was not detected for this Jira instance.", "Confirm the app is installed and try jira zephyr api get /rest/zapi/latest/util/zephyrTestIssueType --json.", 404)
		}
	}
	return err
}
