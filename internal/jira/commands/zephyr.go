package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/jira"
	zapi "engineering-flow-platform-tools/internal/jira/zephyr"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type zephyrRuntime struct {
	ctx    *jira.Context
	client *zapi.Client
	cfg    zapi.EffectiveConfig
}

func zephyrCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "zephyr"}
	doctor := &cobra.Command{Use: "doctor", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", mustB(cmd, "enable-probe"))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		data, err := zapi.Doctor(rt.ctx, rt.client, rt.cfg, mustS(cmd, "project"))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_not_detected"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	doctor.Flags().String("project", "", "")
	doctor.Flags().Bool("enable-probe", false, "")
	c.AddCommand(doctor)

	c.AddCommand(&cobra.Command{Use: "resolve-url <jira-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		data, err := zapi.ResolveURL(args[0])
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_url_unrecognized"))
		}
		return print(cmd, o, output.Success("", data))
	}})

	status := &cobra.Command{Use: "status"}
	status.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, zephyrStatusData(rt.cfg)))
	}})
	c.AddCommand(status)

	util := &cobra.Command{Use: "util"}
	util.AddCommand(&cobra.Command{Use: "test-issue-type", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("util/zephyrTestIssueType")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
		}
		data, err := rt.client.Get("util/zephyrTestIssueType", nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_not_detected"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}})
	c.AddCommand(util)

	c.AddCommand(zephyrTestCmd(o), zephyrCycleCmd(o), zephyrExecutionCmd(o), zephyrAPICmd(o))
	return c
}

func zephyrTestCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		jql := strings.TrimSpace(mustS(cmd, "jql"))
		project := firstNonEmpty(mustS(cmd, "project"), rt.ctx.Inst.DefaultProject)
		if jql == "" {
			if project == "" {
				return invalidArgs(cmd, o, "--project or --jql required", "Use jira zephyr test list --project <PROJECT> --json.")
			}
			jql = "project = " + project + " AND issuetype = Test ORDER BY created DESC"
		}
		body := map[string]interface{}{"jql": jql}
		if limit := mustS(cmd, "limit"); limit != "" {
			n, err := strconv.Atoi(limit)
			if err != nil {
				return invalidArgs(cmd, o, "--limit must be an integer", "")
			}
			body["maxResults"] = n
		}
		if start := mustS(cmd, "start"); start != "" {
			n, err := strconv.Atoi(start)
			if err != nil {
				return invalidArgs(cmd, o, "--start must be an integer", "")
			}
			body["startAt"] = n
		}
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", "search", nil, body)))
		}
		data, err := rt.client.DoJSON(http.MethodPost, "search", nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	list.Flags().String("project", "", "")
	list.Flags().String("jql", "", "")
	list.Flags().String("limit", "", "")
	list.Flags().String("start", "", "")
	c.AddCommand(list)

	get := &cobra.Command{Use: "get <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, args[0], false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		q := map[string]string{}
		if fields := mustS(cmd, "fields"); fields != "" {
			q["fields"] = fields
		}
		if expand := mustS(cmd, "expand"); expand != "" {
			q["expand"] = expand
		}
		path := "issue/" + jira.IssueKey(args[0])
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		data, err := rt.client.DoJSON(http.MethodGet, path, q, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	get.Flags().String("fields", "", "")
	get.Flags().String("expand", "", "")
	c.AddCommand(get)

	create := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		project := firstNonEmpty(mustS(cmd, "project"), rt.ctx.Inst.DefaultProject)
		summary := mustS(cmd, "summary")
		if project == "" || summary == "" {
			return invalidArgs(cmd, o, "--project and --summary required", "Use jira zephyr test create --project <PROJECT> --summary 'Test summary' --json.")
		}
		fields := map[string]interface{}{
			"project":   map[string]string{"key": project},
			"issuetype": map[string]string{"name": "Test"},
			"summary":   summary,
		}
		if desc := mustS(cmd, "description"); desc != "" {
			fields["description"] = desc
		}
		for k, v := range parseKeyValueFields(mustStringArray(cmd, "field")) {
			fields[k] = v
		}
		body := map[string]interface{}{"fields": fields}
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", "issue", nil, body)))
		}
		data, err := rt.client.DoJSON(http.MethodPost, "issue", nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	create.Flags().String("project", "", "")
	create.Flags().String("summary", "", "")
	create.Flags().String("description", "", "")
	create.Flags().StringArray("field", nil, "")
	c.AddCommand(create)
	return c
}

func zephyrCycleCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "cycle"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		projectID, err := zephyrProjectID(rt, mustS(cmd, "project"), mustS(cmd, "project-id"), o.DryRun)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_project_unresolved"))
		}
		q := map[string]string{"projectId": projectID, "versionId": zephyrVersionID(rt, cmd)}
		path := rt.client.ZAPI("cycle")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		data, err := rt.client.Get("cycle", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	addProjectVersionFlags(list)
	c.AddCommand(list)

	get := &cobra.Command{Use: "get <cycle-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		q := zephyrOptionalProjectVersionQuery(rt, cmd)
		path := rt.client.ZAPI("cycle/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		data, err := rt.client.DoJSON(http.MethodGet, path, q, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_cycle_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	get.Flags().String("project-id", "", "")
	get.Flags().String("version-id", "", "")
	c.AddCommand(get)

	create := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "name") == "" {
			return invalidArgs(cmd, o, "--name required", "Use jira zephyr cycle create --project <PROJECT> --name 'Regression' --json.")
		}
		projectID, err := zephyrProjectID(rt, mustS(cmd, "project"), mustS(cmd, "project-id"), o.DryRun)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_project_unresolved"))
		}
		body := zephyrCycleBody(cmd, projectID, zephyrVersionID(rt, cmd))
		path := rt.client.ZAPI("cycle")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", path, nil, body)))
		}
		data, err := rt.client.Post("cycle", body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	addProjectVersionFlags(create)
	addCycleBodyFlags(create)
	c.AddCommand(create)

	update := &cobra.Command{Use: "update <cycle-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		body := map[string]interface{}{"id": args[0]}
		addOptionalString(body, "name", mustS(cmd, "name"))
		addOptionalString(body, "description", mustS(cmd, "description"))
		addOptionalString(body, "build", mustS(cmd, "build"))
		addOptionalString(body, "environment", mustS(cmd, "environment"))
		if len(body) == 1 {
			return invalidArgs(cmd, o, "--name, --description, --build, or --environment required", "Use jira zephyr cycle update <cycle-id> --name 'Regression RC2' --json.")
		}
		path := rt.client.ZAPI("cycle")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("PUT", path, nil, body)))
		}
		data, err := rt.client.Put("cycle", body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_cycle_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	addCycleBodyFlags(update)
	c.AddCommand(update)
	return c
}

func zephyrExecutionCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "execution"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "cycle-id") == "" || mustS(cmd, "project-id") == "" {
			return invalidArgs(cmd, o, "--cycle-id and --project-id required", "Use jira zephyr execution list --cycle-id <ID> --project-id <ID> --json.")
		}
		q := map[string]string{"action": "expand", "cycleId": mustS(cmd, "cycle-id"), "projectId": mustS(cmd, "project-id"), "versionId": zephyrVersionID(rt, cmd)}
		path := rt.client.ZAPI("execution")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		data, err := rt.client.Get("execution", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	list.Flags().String("cycle-id", "", "")
	list.Flags().String("project-id", "", "")
	list.Flags().String("version-id", "", "")
	c.AddCommand(list)

	get := &cobra.Command{Use: "get <execution-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("execution/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
		}
		data, err := rt.client.DoJSON(http.MethodGet, path, nil, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	c.AddCommand(get)

	create := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "issue-id") == "" || mustS(cmd, "cycle-id") == "" || mustS(cmd, "project-id") == "" {
			return invalidArgs(cmd, o, "--issue-id, --cycle-id, and --project-id required", "Use jira zephyr execution create --issue-id <ID> --cycle-id <ID> --project-id <ID> --json.")
		}
		body := map[string]interface{}{"issueId": mustS(cmd, "issue-id"), "cycleId": mustS(cmd, "cycle-id"), "projectId": mustS(cmd, "project-id"), "versionId": zephyrVersionID(rt, cmd)}
		path := rt.client.ZAPI("execution")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", path, nil, body)))
		}
		data, err := rt.client.Post("execution", body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	create.Flags().String("issue-id", "", "")
	create.Flags().String("cycle-id", "", "")
	create.Flags().String("project-id", "", "")
	create.Flags().String("version-id", "", "")
	c.AddCommand(create)

	updateStatus := &cobra.Command{Use: "update-status <execution-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		status := mustS(cmd, "status")
		if status == "" {
			return invalidArgs(cmd, o, "--status required", "Use jira zephyr execution update-status <execution-id> --status PASS --json.")
		}
		mapped, err := zapi.MapStatus(status, rt.cfg.StatusMap)
		if err != nil {
			env := zephyrFailure(err, "invalid_zephyr_status")
			if env.Error != nil {
				env.Error.Hint = "Known statuses: " + strings.Join(zapi.KnownStatuses(rt.cfg.StatusMap), ", ")
			}
			return print(cmd, o, env)
		}
		body := map[string]interface{}{"status": strconv.Itoa(mapped.ID)}
		if comment := mustS(cmd, "comment"); comment != "" {
			body["comment"] = comment
		}
		path := rt.client.ZAPI("execution/" + zapi.PathEscape(args[0]) + "/execute")
		if o.DryRun {
			data := jira.DryRunData("PUT", path, nil, body)
			data["status"] = mapped
			return print(cmd, o, output.Success(rt.ctx.Instance, data))
		}
		data, err := rt.client.DoJSON(http.MethodPut, path, nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	updateStatus.Flags().String("status", "", "")
	updateStatus.Flags().String("comment", "", "")
	c.AddCommand(updateStatus)

	addTests := &cobra.Command{Use: "add-tests-to-cycle", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		issues := splitCSV(mustS(cmd, "issues"))
		if mustS(cmd, "cycle-id") == "" || mustS(cmd, "project-id") == "" || len(issues) == 0 {
			return invalidArgs(cmd, o, "--cycle-id, --project-id, and --issues required", "Use jira zephyr execution add-tests-to-cycle --cycle-id <ID> --project-id <ID> --issues EFP-T1,EFP-T2 --json.")
		}
		body := map[string]interface{}{"cycleId": mustS(cmd, "cycle-id"), "projectId": mustS(cmd, "project-id"), "versionId": zephyrVersionID(rt, cmd), "issues": issues}
		path := rt.client.ZAPI("execution/addTestsToCycle")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", path, nil, body)))
		}
		data, err := rt.client.Post("execution/addTestsToCycle", body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	addTests.Flags().String("cycle-id", "", "")
	addTests.Flags().String("project-id", "", "")
	addTests.Flags().String("version-id", "", "")
	addTests.Flags().String("issues", "", "")
	c.AddCommand(addTests)
	return c
}

func zephyrAPICmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	for _, item := range []struct {
		use    string
		method string
		body   bool
	}{
		{"get <path>", http.MethodGet, false},
		{"post <path>", http.MethodPost, true},
		{"put <path>", http.MethodPut, true},
	} {
		method := item.method
		acceptsBody := item.body
		cc := &cobra.Command{Use: item.use, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newZephyrRuntime(o, "", false)
			if err != nil {
				return print(cmd, o, zephyrFailure(err, "server_error"))
			}
			path, err := rt.client.RawPath(args[0])
			if err != nil {
				return print(cmd, o, zephyrFailure(err, "zephyr_raw_path_blocked"))
			}
			q := queryFromFlags(cmd)
			var body interface{}
			if acceptsBody {
				body, err = readOptionalJiraJSONBody(cmd)
				if err != nil {
					return invalidArgs(cmd, o, err.Error(), "")
				}
			}
			if o.DryRun {
				return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData(method, path, q, body)))
			}
			data, err := rt.client.DoJSON(method, path, q, body)
			if err != nil {
				return print(cmd, o, zephyrFailure(err, "server_error"))
			}
			return print(cmd, o, output.Success(rt.ctx.Instance, data))
		}}
		cc.Flags().StringArray("query", nil, "")
		if acceptsBody {
			cc.Flags().String("body", "", "")
			cc.Flags().String("body-file", "", "")
			cc.Flags().Bool("body-stdin", false, "")
		}
		c.AddCommand(cc)
	}
	return c
}

func newZephyrRuntime(o *Opts, entity string, allowDisabled bool) (*zephyrRuntime, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return nil, zapi.NewError("config_missing", err.Error(), "", 404)
	}
	ctx, err := jira.NewContext(cfg, o.Instance, entity, o.DryRun)
	if err != nil {
		return nil, zapi.NewError(err.Error(), err.Error(), "", 400)
	}
	eff, err := zapi.EffectiveConfigFor(ctx.Inst)
	if err != nil {
		return nil, err
	}
	if eff.Enabled != nil && !*eff.Enabled && !allowDisabled {
		return nil, zapi.NewError("zephyr_not_enabled", "Zephyr is disabled for the selected Jira instance.", "Set jira.instances[].zephyr.enabled=true or run jira zephyr doctor --enable-probe for a probe.", 400)
	}
	return &zephyrRuntime{ctx: ctx, client: zapi.NewClient(ctx, eff), cfg: eff}, nil
}

func zephyrFailure(err error, fallbackCode string) output.Envelope {
	var zerr *zapi.Error
	if errors.As(err, &zerr) {
		return output.Failure(zerr.Code, zerr.Message, zerr.Hint, zerr.Status)
	}
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return output.Failure("zephyr_permission_denied", httpErr.Message, "Verify Jira and Zephyr permissions for the selected account.", httpErr.Status)
		case http.StatusTooManyRequests:
			return output.Failure("zephyr_rate_limited", httpErr.Message, "Retry later or reduce request frequency.", httpErr.Status)
		case http.StatusNotFound:
			if fallbackCode == "zephyr_project_unresolved" || fallbackCode == "zephyr_cycle_not_found" || fallbackCode == "zephyr_execution_not_found" || fallbackCode == "zephyr_not_detected" {
				return output.Failure(fallbackCode, httpErr.Message, "", httpErr.Status)
			}
		}
		return output.Failure(httpErr.Code, httpErr.Message, httpErr.Hint, httpErr.Status)
	}
	if fallbackCode == "" {
		fallbackCode = "server_error"
	}
	return output.Failure(fallbackCode, err.Error(), "", 500)
}

func zephyrStatusData(cfg zapi.EffectiveConfig) map[string]interface{} {
	statuses := make([]map[string]interface{}, 0, len(cfg.StatusMap))
	for _, name := range zapi.KnownStatuses(cfg.StatusMap) {
		statuses = append(statuses, map[string]interface{}{"name": name, "id": cfg.StatusMap[name]})
	}
	return map[string]interface{}{
		"api_family":         cfg.APIFamily,
		"base_path":          cfg.RESTPath,
		"default_version_id": cfg.DefaultVersionID,
		"status_map":         cfg.StatusMap,
		"statuses":           statuses,
	}
}

func zephyrProjectID(rt *zephyrRuntime, project, projectID string, dryRun bool) (string, error) {
	if projectID != "" {
		return projectID, nil
	}
	project = firstNonEmpty(project, rt.ctx.Inst.DefaultProject)
	if project == "" {
		return "", zapi.NewError("invalid_args", "--project or --project-id required", "Use --project <KEY> or --project-id <ID>.", 400)
	}
	if dryRun {
		return "{projectId:" + project + "}", nil
	}
	data, err := rt.client.DoJSON(http.MethodGet, "/rest/api/2/project/"+zapi.PathEscape(project), nil, nil)
	if err != nil {
		return "", err
	}
	m, _ := data.(map[string]interface{})
	id := fmt.Sprint(m["id"])
	if id == "" || id == "<nil>" {
		return "", zapi.NewError("zephyr_project_unresolved", "Jira project did not include an id", "Run jira project get "+project+" --json to inspect the project.", 404)
	}
	return id, nil
}

func zephyrVersionID(rt *zephyrRuntime, cmd *cobra.Command) string {
	return firstNonEmpty(mustS(cmd, "version-id"), rt.cfg.DefaultVersionID)
}

func zephyrOptionalProjectVersionQuery(rt *zephyrRuntime, cmd *cobra.Command) map[string]string {
	q := map[string]string{}
	if projectID := mustS(cmd, "project-id"); projectID != "" {
		q["projectId"] = projectID
		q["versionId"] = zephyrVersionID(rt, cmd)
	} else if versionID := mustS(cmd, "version-id"); versionID != "" {
		q["versionId"] = versionID
	}
	return q
}

func addProjectVersionFlags(cmd *cobra.Command) {
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("project-id", "", "")
	cmd.Flags().String("version-id", "", "")
}

func addCycleBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("build", "", "")
	cmd.Flags().String("environment", "", "")
}

func zephyrCycleBody(cmd *cobra.Command, projectID, versionID string) map[string]interface{} {
	body := map[string]interface{}{"name": mustS(cmd, "name"), "projectId": projectID, "versionId": versionID}
	addOptionalString(body, "description", mustS(cmd, "description"))
	addOptionalString(body, "build", mustS(cmd, "build"))
	addOptionalString(body, "environment", mustS(cmd, "environment"))
	return body
}

func addOptionalString(m map[string]interface{}, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func queryFromFlags(cmd *cobra.Command) map[string]string {
	out := map[string]string{}
	for _, item := range mustStringArray(cmd, "query") {
		kv := strings.SplitN(item, "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func readOptionalJiraJSONBody(cmd *cobra.Command) (interface{}, error) {
	body, err := readJiraBody(cmd)
	if err != nil {
		return nil, err
	}
	if body == "" {
		return nil, nil
	}
	var out interface{}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return map[string]string{"body": body}, nil
	}
	return out, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
