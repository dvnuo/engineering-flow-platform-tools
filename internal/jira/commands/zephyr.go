package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
		catalog, err := zapi.DiscoverStatusCatalog(rt.client, rt.cfg, !o.DryRun, true)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_not_detected"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, zephyrStatusData(rt.cfg, catalog)))
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

	c.AddCommand(zephyrTestCmd(o), zephyrCycleCmd(o), zephyrExecutionCmd(o), zephyrSummaryCmd(o), zephyrZQLCmd(o), zephyrStepResultCmd(o), zephyrAttachmentCmd(o), zephyrFolderCmd(o), zephyrTeststepCmd(o), zephyrDefectCmd(o), zephyrReportCmd(o), zephyrAPICmd(o))
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

	resolve := &cobra.Command{Use: "resolve", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		data, err := zephyrResolveCycle(rt, mustS(cmd, "name"), mustS(cmd, "project"), mustS(cmd, "project-id"), zephyrVersionID(rt, cmd))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_cycle_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, data))
	}}
	resolve.Flags().String("name", "", "")
	addProjectVersionFlags(resolve)
	c.AddCommand(resolve)

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

	del := &cobra.Command{Use: "delete <cycle-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return invalidArgs(cmd, o, "--yes required", "Add --yes after confirming the Zephyr cycle deletion.")
		}
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("cycle/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("DELETE", path, nil, nil)))
		}
		if err := zephyrDelete(rt, path, nil); err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_cycle_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"deleted": true}))
	}}
	c.AddCommand(del)
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
		if status := mustS(cmd, "status"); status != "" {
			mapped, err := zephyrMapStatus(rt, status)
			if err != nil {
				return print(cmd, o, zephyrStatusFailure(err, rt.cfg))
			}
			q["status"] = strconv.Itoa(mapped.ID)
		}
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
	list.Flags().String("status", "", "")
	c.AddCommand(list)

	resolve := &cobra.Command{Use: "resolve", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, mustS(cmd, "issue"), false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		resolved, err := zephyrResolveExecution(rt, zephyrExecutionResolveOptions{
			CycleID:   mustS(cmd, "cycle-id"),
			Issue:     mustS(cmd, "issue"),
			Project:   mustS(cmd, "project"),
			ProjectID: mustS(cmd, "project-id"),
			VersionID: zephyrVersionID(rt, cmd),
			FolderID:  mustS(cmd, "folder-id"),
		})
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, zephyrResolvedExecutionData(resolved)))
	}}
	addExecutionResolverFlags(resolve)
	c.AddCommand(resolve)

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

	updateStatus := &cobra.Command{Use: "update-status [execution-id]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && (mustS(cmd, "cycle-id") != "" || mustS(cmd, "issue") != "") {
			return invalidArgs(cmd, o, "<execution-id> cannot be combined with --cycle-id or --issue", "Use direct mode with only <execution-id>, or semantic mode with --cycle-id and --issue.")
		}
		rt, err := newZephyrRuntime(o, mustS(cmd, "issue"), false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		status := mustS(cmd, "status")
		if status == "" {
			return invalidArgs(cmd, o, "--status required", "Use jira zephyr execution update-status <execution-id> --status PASS --json.")
		}
		mapped, err := zephyrMapStatus(rt, status)
		if err != nil {
			return print(cmd, o, zephyrStatusFailure(err, rt.cfg))
		}
		body := zephyrStatusBody(mapped, mustS(cmd, "comment"))
		executionID := ""
		var resolved zephyrExecutionCandidate
		if len(args) == 1 {
			executionID = args[0]
		} else {
			resolved, err = zephyrResolveExecution(rt, zephyrExecutionResolveOptions{
				CycleID:   mustS(cmd, "cycle-id"),
				Issue:     mustS(cmd, "issue"),
				Project:   mustS(cmd, "project"),
				ProjectID: mustS(cmd, "project-id"),
				VersionID: zephyrVersionID(rt, cmd),
				FolderID:  mustS(cmd, "folder-id"),
			})
			if err != nil {
				return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
			}
			executionID = resolved.ExecutionID
		}
		path := rt.client.ZAPI("execution/" + zapi.PathEscape(executionID) + "/execute")
		if o.DryRun {
			data := jira.DryRunData("PUT", path, nil, body)
			data["status"] = mapped
			data["target_status"] = mapped.Name
			if len(args) == 0 {
				for k, v := range zephyrResolvedExecutionData(resolved) {
					if k != "raw" {
						data[k] = v
					}
				}
			} else {
				data["execution_id"] = executionID
			}
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
	addExecutionResolverFlags(updateStatus)
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

	count := &cobra.Command{Use: "count", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		projectID := mustS(cmd, "project-id")
		if projectID == "" {
			return invalidArgs(cmd, o, "--project-id required", "Use jira zephyr execution count --project-id <ID> --json.")
		}
		group := firstNonEmpty(mustS(cmd, "group"), "cycle")
		if group != "cycle" {
			return invalidArgs(cmd, o, "--group must be cycle", "Only cycle grouping is currently supported.")
		}
		versionID := zephyrVersionID(rt, cmd)
		q := map[string]string{"projectId": projectID, "versionId": versionID}
		path := rt.client.ZAPI("cycle")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("cycle", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, zephyrExecutionCountData(projectID, versionID, group, raw)))
	}}
	count.Flags().String("project-id", "", "")
	count.Flags().String("version-id", "", "")
	count.Flags().String("group", "cycle", "")
	c.AddCommand(count)

	del := &cobra.Command{Use: "delete <execution-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return invalidArgs(cmd, o, "--yes required", "Add --yes after confirming the Zephyr execution deletion.")
		}
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("execution/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("DELETE", path, nil, nil)))
		}
		if err := zephyrDelete(rt, path, nil); err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"deleted": true}))
	}}
	c.AddCommand(del)

	bulkUpdate := &cobra.Command{Use: "bulk-update-status", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		ids := splitCSV(mustS(cmd, "execution-ids"))
		if len(ids) == 0 || mustS(cmd, "status") == "" {
			return invalidArgs(cmd, o, "--execution-ids and --status required", "Use jira zephyr execution bulk-update-status --execution-ids 1,2,3 --status PASS --json.")
		}
		mapped, err := zephyrMapStatus(rt, mustS(cmd, "status"))
		if err != nil {
			return print(cmd, o, zephyrStatusFailure(err, rt.cfg))
		}
		updates := make([]map[string]interface{}, 0, len(ids))
		for _, id := range ids {
			body := zephyrStatusBody(mapped, mustS(cmd, "comment"))
			path := rt.client.ZAPI("execution/" + zapi.PathEscape(id) + "/execute")
			item := jira.DryRunData("PUT", path, nil, body)
			item["status"] = mapped
			updates = append(updates, item)
		}
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"dry_run": true, "updates": updates}))
		}
		results := make([]interface{}, 0, len(updates))
		for i, id := range ids {
			item := updates[i]
			data, err := rt.client.DoJSON(http.MethodPut, item["path"].(string), nil, item["body"])
			if err != nil {
				return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
			}
			results = append(results, map[string]interface{}{"execution_id": id, "result": data})
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"updated": len(results), "results": results}))
	}}
	bulkUpdate.Flags().String("execution-ids", "", "")
	bulkUpdate.Flags().String("status", "", "")
	bulkUpdate.Flags().String("comment", "", "")
	c.AddCommand(bulkUpdate)

	exportCmd := &cobra.Command{Use: "export", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		zql := mustS(cmd, "zql")
		if zql == "" {
			return invalidArgs(cmd, o, "--zql required", "Use jira zephyr execution export --zql \"executionStatus != UNEXECUTED\" --json.")
		}
		exportType := firstNonEmpty(mustS(cmd, "type"), "xls")
		if !isAllowedZephyrExportType(exportType) {
			return invalidArgs(cmd, o, "--type must be xls, xlsx, or csv", "")
		}
		q := map[string]string{"zqlQuery": zql}
		path := rt.client.ZAPI("zql/executeSearch")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("zql/executeSearch", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{
			"export_type": exportType,
			"zql":         zql,
			"results":     raw,
			"note":        "This command returns exportable Zephyr query results as JSON and does not write a file.",
		}))
	}}
	exportCmd.Flags().String("zql", "", "")
	exportCmd.Flags().String("type", "xls", "")
	c.AddCommand(exportCmd)
	return c
}

func zephyrSummaryCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "summary", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		project := firstNonEmpty(mustS(cmd, "project"), rt.ctx.Inst.DefaultProject)
		projectID, err := zephyrProjectID(rt, project, "", o.DryRun)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_project_unresolved"))
		}
		versionID := zephyrVersionID(rt, cmd)
		q := map[string]string{"projectId": projectID, "versionId": versionID}
		path := rt.client.ZAPI("cycle")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("cycle", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, zephyrCycleSummaryData(project, projectID, versionID, raw)))
	}}
	c.Flags().String("project", "", "")
	c.Flags().String("version-id", "", "")
	return c
}

func zephyrZQLCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "zql"}
	search := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		zqlQuery := mustS(cmd, "query")
		if zqlQuery == "" {
			return invalidArgs(cmd, o, "--query required", "Use jira zephyr zql search --query \"executionStatus = FAIL\" --json.")
		}
		q := map[string]string{"zqlQuery": zqlQuery}
		if limit := mustS(cmd, "limit"); limit != "" {
			if _, err := strconv.Atoi(limit); err != nil {
				return invalidArgs(cmd, o, "--limit must be an integer", "")
			}
			q["maxRecords"] = limit
		}
		if start := mustS(cmd, "start"); start != "" {
			if _, err := strconv.Atoi(start); err != nil {
				return invalidArgs(cmd, o, "--start must be an integer", "")
			}
			q["offset"] = start
		}
		path := rt.client.ZAPI("zql/executeSearch")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("zql/executeSearch", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	search.Flags().String("query", "", "")
	search.Flags().String("limit", "", "")
	search.Flags().String("start", "", "")
	c.AddCommand(search)

	clauses := &cobra.Command{Use: "clauses", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("zql/clauses")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
		}
		raw, err := rt.client.Get("zql/clauses", nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	c.AddCommand(clauses)

	autocompleteJSON := &cobra.Command{Use: "autocomplete-json", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("zql/autocompleteZQLJson")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
		}
		raw, err := rt.client.Get("zql/autocompleteZQLJson", nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	c.AddCommand(autocompleteJSON)

	autocomplete := &cobra.Command{Use: "autocomplete", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "field-name") == "" {
			return invalidArgs(cmd, o, "--field-name required", "Use jira zephyr zql autocomplete --field-name executionStatus --field-value PA --json.")
		}
		q := map[string]string{"fieldName": mustS(cmd, "field-name")}
		if fieldValue := mustS(cmd, "field-value"); fieldValue != "" {
			q["fieldValue"] = fieldValue
		}
		path := rt.client.ZAPI("zql/autocomplete")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("zql/autocomplete", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	autocomplete.Flags().String("field-name", "", "")
	autocomplete.Flags().String("field-value", "", "")
	c.AddCommand(autocomplete)
	return c
}

func zephyrStepResultCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "step-result"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		executionID := mustS(cmd, "execution-id")
		if executionID == "" {
			return invalidArgs(cmd, o, "--execution-id required", "Use jira zephyr step-result list --execution-id <ID> --json.")
		}
		q := map[string]string{"executionId": executionID}
		path := rt.client.ZAPI("stepResult")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("stepResult", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	list.Flags().String("execution-id", "", "")
	c.AddCommand(list)

	updateStatus := &cobra.Command{Use: "update-status <step-result-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "status") == "" {
			return invalidArgs(cmd, o, "--status required", "Use jira zephyr step-result update-status <step-result-id> --status PASS --json.")
		}
		mapped, err := zephyrMapStatus(rt, mustS(cmd, "status"))
		if err != nil {
			return print(cmd, o, zephyrStatusFailure(err, rt.cfg))
		}
		body := zephyrStatusBody(mapped, mustS(cmd, "comment"))
		path := rt.client.ZAPI("stepResult/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			data := jira.DryRunData("PUT", path, nil, body)
			data["status"] = mapped
			return print(cmd, o, output.Success(rt.ctx.Instance, data))
		}
		raw, err := rt.client.DoJSON(http.MethodPut, path, nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	updateStatus.Flags().String("status", "", "")
	updateStatus.Flags().String("comment", "", "")
	c.AddCommand(updateStatus)
	return c
}

func zephyrAttachmentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "attachment"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		q, ok := zephyrEntityQuery(cmd)
		if !ok {
			return invalidArgs(cmd, o, "--entity-type and --entity-id required", "Use jira zephyr attachment list --entity-type execution --entity-id <ID> --json.")
		}
		path := rt.client.ZAPI("attachment/attachmentsByEntity")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("attachment/attachmentsByEntity", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addZephyrEntityFlags(list)
	c.AddCommand(list)

	get := &cobra.Command{Use: "get <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("attachment/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
		}
		raw, err := rt.client.DoJSON(http.MethodGet, path, nil, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	c.AddCommand(get)

	upload := &cobra.Command{Use: "upload", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		q, ok := zephyrEntityQuery(cmd)
		if !ok {
			return invalidArgs(cmd, o, "--entity-type and --entity-id required", "Use jira zephyr attachment upload --entity-type execution --entity-id <ID> --file ./report.png --json.")
		}
		file := mustS(cmd, "file")
		if file == "" {
			return invalidArgs(cmd, o, "--file required", "Use jira zephyr attachment upload --entity-type execution --entity-id <ID> --file ./report.png --json.")
		}
		path := rt.client.ZAPI("attachment")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", path, q, map[string]interface{}{"file": file})))
		}
		f, err := os.Open(file)
		if err != nil {
			return invalidArgs(cmd, o, err.Error(), "")
		}
		defer f.Close()
		resp, err := rt.client.JiraClient.Do(httpclient.Request{
			Method:         http.MethodPost,
			Path:           path,
			Query:          q,
			MultipartField: "file",
			MultipartName:  filepath.Base(file),
			Multipart:      f,
			Headers:        map[string]string{"X-Atlassian-Token": "no-check"},
		})
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		defer resp.Body.Close()
		raw, err := zapi.ReadJSONValue(resp.Body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addZephyrEntityFlags(upload)
	upload.Flags().String("file", "", "")
	c.AddCommand(upload)

	del := &cobra.Command{Use: "delete <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return invalidArgs(cmd, o, "--yes required", "Add --yes after confirming the Zephyr attachment deletion.")
		}
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("attachment/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("DELETE", path, nil, nil)))
		}
		if err := zephyrDelete(rt, path, nil); err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"deleted": true}))
	}}
	c.AddCommand(del)
	return c
}

func zephyrFolderCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "folder"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "cycle-id") == "" || mustS(cmd, "project-id") == "" || mustS(cmd, "version-id") == "" {
			return invalidArgs(cmd, o, "--cycle-id, --project-id, and --version-id required", "Use jira zephyr folder list --cycle-id <ID> --project-id <ID> --version-id -1 --json.")
		}
		q := map[string]string{"projectId": mustS(cmd, "project-id"), "versionId": mustS(cmd, "version-id")}
		if limit := mustS(cmd, "limit"); limit != "" {
			q["limit"] = limit
		}
		if offset := mustS(cmd, "offset"); offset != "" {
			q["offset"] = offset
		}
		path := rt.client.ZAPI("cycle/" + zapi.PathEscape(mustS(cmd, "cycle-id")) + "/folders")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.DoJSON(http.MethodGet, path, q, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addFolderScopeFlags(list)
	list.Flags().String("limit", "", "")
	list.Flags().String("offset", "", "")
	c.AddCommand(list)

	create := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "cycle-id") == "" || mustS(cmd, "project-id") == "" || mustS(cmd, "version-id") == "" || mustS(cmd, "name") == "" {
			return invalidArgs(cmd, o, "--cycle-id, --project-id, --version-id, and --name required", "Use jira zephyr folder create --cycle-id <ID> --project-id <ID> --version-id -1 --name Smoke --json.")
		}
		body := map[string]interface{}{
			"cycleId":   mustS(cmd, "cycle-id"),
			"projectId": mustS(cmd, "project-id"),
			"versionId": mustS(cmd, "version-id"),
			"name":      mustS(cmd, "name"),
		}
		addOptionalString(body, "description", mustS(cmd, "description"))
		path := rt.client.ZAPI("folder/create")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", path, nil, body)))
		}
		raw, err := rt.client.Post("folder/create", body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addFolderScopeFlags(create)
	create.Flags().String("name", "", "")
	create.Flags().String("description", "", "")
	c.AddCommand(create)

	update := &cobra.Command{Use: "update <folder-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		body := map[string]interface{}{"folderId": args[0]}
		addOptionalString(body, "name", mustS(cmd, "name"))
		addOptionalString(body, "description", mustS(cmd, "description"))
		if len(body) == 1 {
			return invalidArgs(cmd, o, "--name or --description required", "Use jira zephyr folder update <folder-id> --name 'Smoke RC2' --json.")
		}
		path := rt.client.ZAPI("folder/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("PUT", path, nil, body)))
		}
		raw, err := rt.client.DoJSON(http.MethodPut, path, nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	update.Flags().String("name", "", "")
	update.Flags().String("description", "", "")
	c.AddCommand(update)

	del := &cobra.Command{Use: "delete <folder-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return invalidArgs(cmd, o, "--yes required", "Add --yes after confirming the Zephyr folder deletion.")
		}
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "cycle-id") == "" || mustS(cmd, "project-id") == "" || mustS(cmd, "version-id") == "" {
			return invalidArgs(cmd, o, "--cycle-id, --project-id, and --version-id required", "Use jira zephyr folder delete <folder-id> --cycle-id <ID> --project-id <ID> --version-id -1 --yes --json.")
		}
		q := map[string]string{"cycleId": mustS(cmd, "cycle-id"), "projectId": mustS(cmd, "project-id"), "versionId": mustS(cmd, "version-id")}
		path := rt.client.ZAPI("folder/" + zapi.PathEscape(args[0]))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("DELETE", path, q, nil)))
		}
		if err := zephyrDelete(rt, path, q); err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"deleted": true}))
	}}
	addFolderScopeFlags(del)
	c.AddCommand(del)
	return c
}

func zephyrTeststepCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "teststep"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, mustS(cmd, "issue"), false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		issue, err := zephyrResolveIssue(rt, mustS(cmd, "issue"))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if issue.ID == "" {
			return print(cmd, o, zephyrFailure(zapi.NewError("invalid_args", "Jira issue id could not be resolved", "Run jira issue get "+mustS(cmd, "issue")+" --json to inspect the issue.", 400), "invalid_args"))
		}
		q := map[string]string{"offset": mustS(cmd, "offset"), "limit": mustS(cmd, "limit")}
		path := rt.client.ZAPI("teststep/" + zapi.PathEscape(issue.ID))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.DoJSON(http.MethodGet, path, q, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addTeststepIssueFlags(list)
	list.Flags().String("offset", "0", "")
	list.Flags().String("limit", "50", "")
	c.AddCommand(list)

	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, mustS(cmd, "issue"), false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "step-id") == "" {
			return invalidArgs(cmd, o, "--step-id required", "Use jira zephyr teststep get --issue EFP-123 --step-id 10 --json.")
		}
		issue, err := zephyrResolveIssue(rt, mustS(cmd, "issue"))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("teststep/" + zapi.PathEscape(issue.ID) + "/" + zapi.PathEscape(mustS(cmd, "step-id")))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
		}
		raw, err := rt.client.DoJSON(http.MethodGet, path, nil, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addTeststepIssueFlags(get)
	get.Flags().String("step-id", "", "")
	c.AddCommand(get)

	create := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, mustS(cmd, "issue"), false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "step") == "" {
			return invalidArgs(cmd, o, "--step required", "Use jira zephyr teststep create --issue EFP-123 --step 'Open login page' --json.")
		}
		issue, err := zephyrResolveIssue(rt, mustS(cmd, "issue"))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		body := zephyrTeststepBody(cmd, false)
		path := rt.client.ZAPI("teststep/" + zapi.PathEscape(issue.ID))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", path, nil, body)))
		}
		raw, err := rt.client.DoJSON(http.MethodPost, path, nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addTeststepIssueFlags(create)
	addTeststepBodyFlags(create)
	c.AddCommand(create)

	update := &cobra.Command{Use: "update", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, mustS(cmd, "issue"), false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "step-id") == "" {
			return invalidArgs(cmd, o, "--step-id required", "Use jira zephyr teststep update --issue EFP-123 --step-id 10 --step 'Open login page' --json.")
		}
		body := zephyrTeststepBody(cmd, true)
		if len(body) == 0 {
			return invalidArgs(cmd, o, "--step, --data, or --result required", "Use jira zephyr teststep update --issue EFP-123 --step-id 10 --step 'Open login page' --json.")
		}
		issue, err := zephyrResolveIssue(rt, mustS(cmd, "issue"))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("teststep/" + zapi.PathEscape(issue.ID) + "/" + zapi.PathEscape(mustS(cmd, "step-id")))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("PUT", path, nil, body)))
		}
		raw, err := rt.client.DoJSON(http.MethodPut, path, nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	addTeststepIssueFlags(update)
	update.Flags().String("step-id", "", "")
	addTeststepBodyFlags(update)
	c.AddCommand(update)

	del := &cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return invalidArgs(cmd, o, "--yes required", "Add --yes after confirming the Zephyr test step deletion.")
		}
		rt, err := newZephyrRuntime(o, mustS(cmd, "issue"), false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		if mustS(cmd, "step-id") == "" {
			return invalidArgs(cmd, o, "--step-id required", "Use jira zephyr teststep delete --issue EFP-123 --step-id 10 --yes --json.")
		}
		issue, err := zephyrResolveIssue(rt, mustS(cmd, "issue"))
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		path := rt.client.ZAPI("teststep/" + zapi.PathEscape(issue.ID) + "/" + zapi.PathEscape(mustS(cmd, "step-id")))
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("DELETE", path, nil, nil)))
		}
		if err := zephyrDelete(rt, path, nil); err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"deleted": true}))
	}}
	addTeststepIssueFlags(del)
	del.Flags().String("step-id", "", "")
	c.AddCommand(del)
	return c
}

func zephyrDefectCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "defect"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		executionID := mustS(cmd, "execution-id")
		if executionID == "" {
			return invalidArgs(cmd, o, "--execution-id required", "Use jira zephyr defect list --execution-id <ID> --json.")
		}
		path := rt.client.ZAPI("execution/" + zapi.PathEscape(executionID) + "/defects")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
		}
		raw, err := rt.client.DoJSON(http.MethodGet, path, nil, nil)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	list.Flags().String("execution-id", "", "")
	c.AddCommand(list)

	add := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		executionID := mustS(cmd, "execution-id")
		issue := mustS(cmd, "issue")
		if executionID == "" || issue == "" {
			return invalidArgs(cmd, o, "--execution-id and --issue required", "Use jira zephyr defect add --execution-id <ID> --issue EFP-999 --json.")
		}
		body := map[string]interface{}{"issueKey": issue}
		path := rt.client.ZAPI("execution/" + zapi.PathEscape(executionID) + "/defect")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("POST", path, nil, body)))
		}
		raw, err := rt.client.DoJSON(http.MethodPost, path, nil, body)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_execution_not_found"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, raw))
	}}
	add.Flags().String("execution-id", "", "")
	add.Flags().String("issue", "", "")
	c.AddCommand(add)
	return c
}

func zephyrReportCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "report"}
	coverage := &cobra.Command{Use: "coverage", RunE: func(cmd *cobra.Command, args []string) error {
		rt, err := newZephyrRuntime(o, "", false)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		project := firstNonEmpty(mustS(cmd, "project"), rt.ctx.Inst.DefaultProject)
		projectID, err := zephyrProjectID(rt, project, mustS(cmd, "project-id"), o.DryRun)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_project_unresolved"))
		}
		versionID := zephyrVersionID(rt, cmd)
		q := map[string]string{"projectId": projectID, "versionId": versionID}
		path := rt.client.ZAPI("cycle")
		if o.DryRun {
			return print(cmd, o, output.Success(rt.ctx.Instance, jira.DryRunData("GET", path, q, nil)))
		}
		raw, err := rt.client.Get("cycle", q)
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "server_error"))
		}
		return print(cmd, o, output.Success(rt.ctx.Instance, zephyrCoverageData(project, projectID, versionID, raw)))
	}}
	addProjectVersionFlags(coverage)
	c.AddCommand(coverage)
	return c
}

func zephyrAPICmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	c.AddCommand(&cobra.Command{Use: "catalog", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", zapi.CatalogEnvelope()))
	}})
	c.AddCommand(&cobra.Command{Use: "describe <endpoint-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, err := zapi.DescribeEndpoint(args[0])
		if err != nil {
			return print(cmd, o, zephyrFailure(err, "zephyr_endpoint_not_found"))
		}
		return print(cmd, o, output.Success("", endpoint))
	}})
	for _, item := range []struct {
		use    string
		method string
		body   bool
	}{
		{"get <path>", http.MethodGet, false},
		{"post <path>", http.MethodPost, true},
		{"put <path>", http.MethodPut, true},
		{"delete <path>", http.MethodDelete, false},
	} {
		method := item.method
		acceptsBody := item.body
		cc := &cobra.Command{Use: item.use, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if method == http.MethodDelete && !o.Yes {
				return invalidArgs(cmd, o, "--yes required", "Add --yes after confirming the Zephyr raw API DELETE.")
			}
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
			if method == http.MethodDelete {
				if err := zephyrDelete(rt, path, q); err != nil {
					return print(cmd, o, zephyrFailure(err, "server_error"))
				}
				return print(cmd, o, output.Success(rt.ctx.Instance, map[string]interface{}{"deleted": true}))
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

func zephyrStatusData(cfg zapi.EffectiveConfig, catalog zapi.StatusCatalog) map[string]interface{} {
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
		"execution_statuses": catalog.ExecutionStatuses,
		"step_statuses":      catalog.StepStatuses,
		"aliases":            catalog.Aliases,
		"source":             catalog.Source,
	}
}

func zephyrMapStatus(rt *zephyrRuntime, status string) (zapi.MappedStatus, error) {
	catalog, err := zapi.DiscoverStatusCatalog(rt.client, rt.cfg, !rt.ctx.DryRun, false)
	if err != nil {
		return zapi.MappedStatus{}, err
	}
	return zapi.MapStatusWithCatalog(status, catalog)
}

func zephyrStatusFailure(err error, cfg zapi.EffectiveConfig) output.Envelope {
	env := zephyrFailure(err, "invalid_zephyr_status")
	if env.Error != nil {
		env.Error.Hint = "Known statuses: " + strings.Join(zapi.KnownStatuses(cfg.StatusMap), ", ")
	}
	return env
}

func zephyrStatusBody(mapped zapi.MappedStatus, comment string) map[string]interface{} {
	body := map[string]interface{}{"status": strconv.Itoa(mapped.ID)}
	if comment != "" {
		body["comment"] = comment
	}
	return body
}

func zephyrDelete(rt *zephyrRuntime, path string, q map[string]string) error {
	resp, err := rt.client.JiraClient.Do(httpclient.Request{Method: http.MethodDelete, Path: path, Query: q})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
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

func addExecutionResolverFlags(cmd *cobra.Command) {
	cmd.Flags().String("cycle-id", "", "")
	cmd.Flags().String("issue", "", "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("project-id", "", "")
	cmd.Flags().String("version-id", "", "")
	cmd.Flags().String("folder-id", "", "")
}

func addZephyrEntityFlags(cmd *cobra.Command) {
	cmd.Flags().String("entity-type", "", "")
	cmd.Flags().String("entity-id", "", "")
}

func addFolderScopeFlags(cmd *cobra.Command) {
	cmd.Flags().String("cycle-id", "", "")
	cmd.Flags().String("project-id", "", "")
	cmd.Flags().String("version-id", "", "")
}

func zephyrEntityQuery(cmd *cobra.Command) (map[string]string, bool) {
	entityType := mustS(cmd, "entity-type")
	entityID := mustS(cmd, "entity-id")
	if entityType == "" || entityID == "" {
		return nil, false
	}
	return map[string]string{"entityId": entityID, "entityType": entityType}, true
}

func addCycleBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("description", "", "")
	cmd.Flags().String("build", "", "")
	cmd.Flags().String("environment", "", "")
}

func addTeststepIssueFlags(cmd *cobra.Command) {
	cmd.Flags().String("issue", "", "")
}

func addTeststepBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("step", "", "")
	cmd.Flags().String("data", "", "")
	cmd.Flags().String("result", "", "")
}

func zephyrTeststepBody(cmd *cobra.Command, partial bool) map[string]interface{} {
	body := map[string]interface{}{}
	for _, key := range []string{"step", "data", "result"} {
		value := mustS(cmd, key)
		if partial {
			if value != "" {
				body[key] = value
			}
			continue
		}
		body[key] = value
	}
	return body
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

func zephyrCycleSummaryData(project, projectID, versionID string, raw interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"project":    project,
		"project_id": projectID,
		"version_id": versionID,
	}
	if cycles, ok := zephyrExtractCycles(raw); ok {
		out["cycles"] = cycles
		out["cycle_count"] = len(cycles)
	} else {
		out["raw"] = raw
	}
	return out
}

func zephyrExecutionCountData(projectID, versionID, group string, raw interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"project_id": projectID,
		"version_id": versionID,
		"group":      group,
		"groups":     []map[string]interface{}{},
		"raw":        raw,
	}
	cycles, ok := zephyrExtractCycles(raw)
	if !ok {
		return out
	}
	groups := make([]map[string]interface{}, 0, len(cycles))
	for _, cycle := range cycles {
		count, _ := zephyrCycleExecutionCount(cycle)
		groups = append(groups, map[string]interface{}{
			"cycle_id": zephyrStringField(cycle, "id"),
			"name":     zephyrStringField(cycle, "name"),
			"count":    count,
		})
	}
	out["groups"] = groups
	return out
}

func zephyrCoverageData(project, projectID, versionID string, raw interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"project":    project,
		"project_id": projectID,
		"version_id": versionID,
		"source":     "cycle",
	}
	cycles, ok := zephyrExtractCycles(raw)
	if !ok {
		out["raw"] = raw
		return out
	}
	out["cycles"] = cycles
	out["cycle_count"] = len(cycles)
	totalExecutions := 0
	hasExecutionCount := false
	statusCounts := map[string]int{}
	for _, cycle := range cycles {
		if count, ok := zephyrCycleExecutionCount(cycle); ok {
			totalExecutions += count
			hasExecutionCount = true
		}
		if counts, ok := zephyrCycleStatusCounts(cycle); ok {
			for k, v := range counts {
				statusCounts[k] += v
			}
		}
	}
	if hasExecutionCount {
		out["execution_count"] = totalExecutions
	}
	if len(statusCounts) > 0 {
		out["status_counts"] = statusCounts
	}
	return out
}

func zephyrExtractCycles(raw interface{}) ([]map[string]interface{}, bool) {
	switch v := raw.(type) {
	case []interface{}:
		return zephyrCycleMapsFromList(v)
	case []map[string]interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out, true
	case map[string]interface{}:
		for _, key := range []string{"cycles", "cycleSummaries", "cycle_summaries"} {
			if child, ok := v[key]; ok {
				if cycles, ok := zephyrExtractCycles(child); ok {
					return cycles, true
				}
			}
		}
		return zephyrCycleMapsFromMap(v)
	default:
		return nil, false
	}
}

func zephyrCycleMapsFromList(items []interface{}) ([]map[string]interface{}, bool) {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out, true
}

func zephyrCycleMapsFromMap(m map[string]interface{}) ([]map[string]interface{}, bool) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		child, ok := m[key].(map[string]interface{})
		if !ok || !zephyrLooksLikeCycle(key, child) {
			continue
		}
		if _, exists := child["id"]; !exists && key != "" {
			copyChild := map[string]interface{}{}
			for k, v := range child {
				copyChild[k] = v
			}
			copyChild["id"] = key
			child = copyChild
		}
		out = append(out, child)
	}
	return out, len(out) > 0
}

func zephyrLooksLikeCycle(key string, m map[string]interface{}) bool {
	if key == "recordsCount" || key == "offset" || key == "maxRecords" {
		return false
	}
	for _, field := range []string{"id", "name", "totalExecutions", "executionSummaries", "projectId", "versionId", "environment", "build"} {
		if _, ok := m[field]; ok {
			return true
		}
	}
	return false
}

func zephyrCycleExecutionCount(cycle map[string]interface{}) (int, bool) {
	for _, key := range []string{"executionCount", "execution_count", "totalExecutions", "totalExecution", "totalExecuted", "total"} {
		if count, ok := zephyrInt(cycle[key]); ok {
			return count, true
		}
	}
	if items, ok := cycle["executions"].([]interface{}); ok {
		return len(items), true
	}
	return 0, false
}

func zephyrCycleStatusCounts(cycle map[string]interface{}) (map[string]int, bool) {
	for _, key := range []string{"executionSummaries", "statusCounts", "status_counts", "summary"} {
		if counts, ok := zephyrStatusCounts(cycle[key]); ok {
			return counts, true
		}
	}
	return nil, false
}

func zephyrStatusCounts(v interface{}) (map[string]int, bool) {
	switch x := v.(type) {
	case map[string]interface{}:
		out := map[string]int{}
		for k, v := range x {
			if normalized, err := zapi.NormalizeStatus(k); err == nil {
				if count, ok := zephyrInt(v); ok {
					out[normalized] += count
				}
			}
		}
		if len(out) > 0 {
			return out, true
		}
		for _, key := range []string{"executionSummary", "statuses", "statusCounts", "status_counts"} {
			if counts, ok := zephyrStatusCounts(x[key]); ok {
				return counts, true
			}
		}
	case []interface{}:
		out := map[string]int{}
		for _, item := range x {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name := firstNonEmpty(zephyrStringField(m, "statusName"), zephyrStringField(m, "name"), zephyrStringField(m, "status"))
			normalized, err := zapi.NormalizeStatus(name)
			if err != nil {
				continue
			}
			for _, countKey := range []string{"count", "executionCount", "total", "value"} {
				if count, ok := zephyrInt(m[countKey]); ok {
					out[normalized] += count
					break
				}
			}
		}
		if len(out) > 0 {
			return out, true
		}
	}
	return nil, false
}

func zephyrInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		return int(n), err == nil
	case string:
		if x == "" {
			return 0, false
		}
		n, err := strconv.Atoi(x)
		return n, err == nil
	default:
		return 0, false
	}
}

func zephyrStringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func isAllowedZephyrExportType(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "xls", "xlsx", "csv":
		return true
	default:
		return false
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
