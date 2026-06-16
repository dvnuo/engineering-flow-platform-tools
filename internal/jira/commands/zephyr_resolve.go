package commands

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/jira"
	zapi "engineering-flow-platform-tools/internal/jira/zephyr"
)

type zephyrIssueRef struct {
	Key       string
	ID        string
	ProjectID string
}

type zephyrExecutionResolveOptions struct {
	CycleID   string
	Issue     string
	Project   string
	ProjectID string
	VersionID string
	FolderID  string
}

type zephyrExecutionCandidate struct {
	ExecutionID string
	IssueKey    string
	IssueID     string
	CycleID     string
	ProjectID   string
	VersionID   string
	FolderID    string
	Raw         map[string]interface{}
}

func zephyrResolveIssue(rt *zephyrRuntime, issue string) (zephyrIssueRef, error) {
	keyOrID := jira.IssueKey(strings.TrimSpace(issue))
	if keyOrID == "" {
		return zephyrIssueRef{}, zapi.NewError("invalid_args", "--issue required", "Use --issue EFP-123 or a Jira issue URL.", 400)
	}
	path := "/rest/api/2/issue/" + zapi.PathEscape(keyOrID)
	raw, err := rt.client.DoJSON(http.MethodGet, path, map[string]string{"fields": "project"}, nil)
	if err != nil {
		return zephyrIssueRef{}, err
	}
	m, _ := raw.(map[string]interface{})
	ref := zephyrIssueRef{
		Key:       firstNonEmpty(zephyrLookupString(m, "key"), issueKeyFallback(keyOrID)),
		ID:        firstNonEmpty(zephyrLookupString(m, "id"), issueIDFallback(keyOrID)),
		ProjectID: zephyrLookupString(m, "fields.project.id"),
	}
	return ref, nil
}

func issueKeyFallback(v string) string {
	if strings.Contains(v, "-") {
		return strings.ToUpper(v)
	}
	return ""
}

func issueIDFallback(v string) string {
	for _, r := range v {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return v
}

func zephyrResolveExecution(rt *zephyrRuntime, opts zephyrExecutionResolveOptions) (zephyrExecutionCandidate, error) {
	if strings.TrimSpace(opts.CycleID) == "" || strings.TrimSpace(opts.Issue) == "" {
		return zephyrExecutionCandidate{}, zapi.NewError("invalid_args", "--cycle-id and --issue required", "Use jira zephyr execution resolve --cycle-id <ID> --issue <KEY> --json.", 400)
	}
	issue, err := zephyrResolveIssue(rt, opts.Issue)
	if err != nil {
		return zephyrExecutionCandidate{}, err
	}
	projectID := strings.TrimSpace(opts.ProjectID)
	if projectID == "" {
		projectID = issue.ProjectID
	}
	if projectID == "" {
		projectID, err = zephyrProjectID(rt, opts.Project, "", false)
		if err != nil {
			return zephyrExecutionCandidate{}, err
		}
	}
	versionID := firstNonEmpty(strings.TrimSpace(opts.VersionID), rt.cfg.DefaultVersionID)
	q := map[string]string{
		"action":    "expand",
		"cycleId":   opts.CycleID,
		"projectId": projectID,
		"versionId": versionID,
	}
	if opts.FolderID != "" {
		q["folderId"] = opts.FolderID
	}
	raw, err := rt.client.Get("execution", q)
	if err != nil {
		return zephyrExecutionCandidate{}, err
	}
	candidates := zephyrFilterExecutionCandidates(zephyrExtractExecutionCandidates(raw), issue)
	if len(candidates) == 0 {
		return zephyrExecutionCandidate{}, zapi.NewError(
			"zephyr_execution_not_found",
			"Zephyr execution was not found for issue "+firstNonEmpty(issue.Key, issue.ID)+" in cycle "+opts.CycleID,
			"The test may need to be added to the cycle first with jira zephyr execution add-tests-to-cycle.",
			404,
		)
	}
	if len(candidates) > 1 {
		return zephyrExecutionCandidate{}, zapi.NewError(
			"ambiguous_zephyr_execution",
			"Multiple Zephyr executions matched issue "+firstNonEmpty(issue.Key, issue.ID)+" in cycle "+opts.CycleID,
			"Candidates: "+zephyrCandidateHint(candidates),
			409,
		)
	}
	resolved := candidates[0]
	resolved.IssueKey = firstNonEmpty(resolved.IssueKey, issue.Key)
	resolved.IssueID = firstNonEmpty(resolved.IssueID, issue.ID)
	resolved.CycleID = firstNonEmpty(resolved.CycleID, opts.CycleID)
	resolved.ProjectID = firstNonEmpty(resolved.ProjectID, projectID)
	resolved.VersionID = firstNonEmpty(resolved.VersionID, versionID)
	resolved.FolderID = firstNonEmpty(resolved.FolderID, opts.FolderID)
	return resolved, nil
}

func zephyrResolveExecutionsForIssues(rt *zephyrRuntime, opts zephyrExecutionResolveOptions, issues []string) ([]zephyrExecutionCandidate, error) {
	if strings.TrimSpace(opts.CycleID) == "" || len(issues) == 0 {
		return nil, zapi.NewError("invalid_args", "--cycle-id and --issues required", "Use jira zephyr execution add-tests-to-cycle --cycle-id <ID> --issues EFP-123 --folder-id <ID> --json.", 400)
	}
	projectID := strings.TrimSpace(opts.ProjectID)
	refs := make([]zephyrIssueRef, 0, len(issues))
	for _, issueValue := range issues {
		issue, err := zephyrResolveIssue(rt, issueValue)
		if err != nil {
			return nil, err
		}
		if projectID == "" {
			projectID = issue.ProjectID
		}
		refs = append(refs, issue)
	}
	if projectID == "" {
		var err error
		projectID, err = zephyrProjectID(rt, opts.Project, "", false)
		if err != nil {
			return nil, err
		}
	}
	versionID := firstNonEmpty(strings.TrimSpace(opts.VersionID), rt.cfg.DefaultVersionID)
	q := map[string]string{
		"action":    "expand",
		"cycleId":   opts.CycleID,
		"projectId": projectID,
		"versionId": versionID,
	}
	if opts.FolderID != "" {
		q["folderId"] = opts.FolderID
	}
	raw, err := rt.client.Get("execution", q)
	if err != nil {
		return nil, err
	}
	candidates := zephyrExtractExecutionCandidates(raw)
	resolved := make([]zephyrExecutionCandidate, 0, len(refs))
	for _, issue := range refs {
		matches := zephyrFilterExecutionCandidates(candidates, issue)
		label := firstNonEmpty(issue.Key, issue.ID)
		if len(matches) == 0 {
			return nil, zapi.NewError(
				"zephyr_execution_not_found",
				"Zephyr execution was not found for issue "+label+" in cycle "+opts.CycleID,
				"The test may need to be added to the cycle first with jira zephyr execution add-tests-to-cycle.",
				404,
			)
		}
		if len(matches) > 1 {
			return nil, zapi.NewError(
				"ambiguous_zephyr_execution",
				"Multiple Zephyr executions matched issue "+label+" in cycle "+opts.CycleID,
				"Candidates: "+zephyrCandidateHint(matches),
				409,
			)
		}
		item := matches[0]
		item.IssueKey = firstNonEmpty(item.IssueKey, issue.Key)
		item.IssueID = firstNonEmpty(item.IssueID, issue.ID)
		item.CycleID = firstNonEmpty(item.CycleID, opts.CycleID)
		item.ProjectID = firstNonEmpty(item.ProjectID, projectID)
		item.VersionID = firstNonEmpty(item.VersionID, versionID)
		item.FolderID = firstNonEmpty(item.FolderID, opts.FolderID)
		if strings.TrimSpace(item.ExecutionID) == "" {
			return nil, zapi.NewError(
				"zephyr_execution_id_missing",
				"Zephyr execution for issue "+label+" did not include an execution id",
				"Inspect jira zephyr execution list --cycle-id "+opts.CycleID+" --project-id "+projectID+" --json before moving executions to a folder.",
				500,
			)
		}
		resolved = append(resolved, item)
	}
	return dedupeExecutionCandidates(resolved), nil
}

func zephyrResolvedExecutionData(resolved zephyrExecutionCandidate) map[string]interface{} {
	out := map[string]interface{}{"raw": resolved.Raw}
	addStringField(out, "execution_id", resolved.ExecutionID)
	addStringField(out, "issue_key", resolved.IssueKey)
	addStringField(out, "issue_id", resolved.IssueID)
	addStringField(out, "cycle_id", resolved.CycleID)
	addStringField(out, "project_id", resolved.ProjectID)
	addStringField(out, "version_id", resolved.VersionID)
	addStringField(out, "folder_id", resolved.FolderID)
	return out
}

func zephyrExtractExecutionCandidates(raw interface{}) []zephyrExecutionCandidate {
	switch v := raw.(type) {
	case []interface{}:
		out := make([]zephyrExecutionCandidate, 0, len(v))
		for _, item := range v {
			out = append(out, zephyrExtractExecutionCandidates(item)...)
		}
		return dedupeExecutionCandidates(out)
	case []map[string]interface{}:
		out := make([]zephyrExecutionCandidate, 0, len(v))
		for _, item := range v {
			out = append(out, zephyrExecutionCandidateFromMap("", item))
		}
		return dedupeExecutionCandidates(out)
	case map[string]interface{}:
		if child, ok := v["executions"]; ok {
			return zephyrExtractExecutionCandidates(child)
		}
		if zephyrLooksLikeExecution("", v) {
			return []zephyrExecutionCandidate{zephyrExecutionCandidateFromMap("", v)}
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := []zephyrExecutionCandidate{}
		for _, key := range keys {
			child, ok := v[key].(map[string]interface{})
			if !ok || !zephyrLooksLikeExecution(key, child) {
				continue
			}
			out = append(out, zephyrExecutionCandidateFromMap(key, child))
		}
		return dedupeExecutionCandidates(out)
	default:
		return nil
	}
}

func zephyrExecutionCandidateFromMap(mapKey string, m map[string]interface{}) zephyrExecutionCandidate {
	executionID := firstNonEmpty(
		zephyrLookupString(m, "id"),
		zephyrLookupString(m, "executionId"),
		zephyrLookupString(m, "executionID"),
		zephyrLookupString(m, "execution.id"),
	)
	if executionID == "" && issueIDFallback(mapKey) != "" {
		executionID = mapKey
	}
	return zephyrExecutionCandidate{
		ExecutionID: executionID,
		IssueKey: firstNonEmpty(
			zephyrLookupString(m, "issueKey"),
			zephyrLookupString(m, "issue.key"),
			zephyrLookupString(m, "key"),
		),
		IssueID: firstNonEmpty(
			zephyrLookupString(m, "issueId"),
			zephyrLookupString(m, "issueID"),
			zephyrLookupString(m, "issue.id"),
		),
		CycleID:   firstNonEmpty(zephyrLookupString(m, "cycleId"), zephyrLookupString(m, "cycle.id")),
		ProjectID: firstNonEmpty(zephyrLookupString(m, "projectId"), zephyrLookupString(m, "project.id")),
		VersionID: firstNonEmpty(zephyrLookupString(m, "versionId"), zephyrLookupString(m, "version.id")),
		FolderID:  firstNonEmpty(zephyrLookupString(m, "folderId"), zephyrLookupString(m, "folder.id")),
		Raw:       m,
	}
}

func zephyrLooksLikeExecution(mapKey string, m map[string]interface{}) bool {
	if mapKey == "recordsCount" || mapKey == "offset" || mapKey == "maxRecords" {
		return false
	}
	for _, key := range []string{"id", "executionId", "executionID", "issueKey", "issueId", "cycleId", "executionStatus", "status", "issue", "execution"} {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return issueIDFallback(mapKey) != ""
}

func zephyrFilterExecutionCandidates(candidates []zephyrExecutionCandidate, issue zephyrIssueRef) []zephyrExecutionCandidate {
	var out []zephyrExecutionCandidate
	issueKey := strings.ToUpper(strings.TrimSpace(issue.Key))
	issueID := strings.TrimSpace(issue.ID)
	for _, candidate := range candidates {
		if issueKey != "" && strings.ToUpper(strings.TrimSpace(candidate.IssueKey)) == issueKey {
			out = append(out, candidate)
			continue
		}
		if issueID != "" && strings.TrimSpace(candidate.IssueID) == issueID {
			out = append(out, candidate)
		}
	}
	return dedupeExecutionCandidates(out)
}

func dedupeExecutionCandidates(candidates []zephyrExecutionCandidate) []zephyrExecutionCandidate {
	out := []zephyrExecutionCandidate{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		key := firstNonEmpty(candidate.ExecutionID, fmt.Sprintf("%s/%s/%s", candidate.IssueKey, candidate.IssueID, candidate.CycleID))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	return out
}

func zephyrCandidateHint(candidates []zephyrExecutionCandidate) string {
	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		fields := []string{}
		if candidate.ExecutionID != "" {
			fields = append(fields, "execution_id="+candidate.ExecutionID)
		}
		if candidate.IssueKey != "" {
			fields = append(fields, "issue_key="+candidate.IssueKey)
		}
		if candidate.CycleID != "" {
			fields = append(fields, "cycle_id="+candidate.CycleID)
		}
		if candidate.FolderID != "" {
			fields = append(fields, "folder_id="+candidate.FolderID)
		}
		parts = append(parts, "{"+strings.Join(fields, ", ")+"}")
	}
	return strings.Join(parts, ", ")
}

func zephyrResolveCycle(rt *zephyrRuntime, name, project, projectID, versionID string) (map[string]interface{}, error) {
	if strings.TrimSpace(name) == "" {
		return nil, zapi.NewError("invalid_args", "--name required", "Use jira zephyr cycle resolve --name 'Sprint 42 Regression' --project EFP --json.", 400)
	}
	var err error
	if projectID == "" && project != "" {
		projectID, err = zephyrProjectID(rt, project, "", false)
		if err != nil {
			return nil, err
		}
	}
	versionID = firstNonEmpty(strings.TrimSpace(versionID), rt.cfg.DefaultVersionID)
	q := map[string]string{}
	if projectID != "" {
		q["projectId"] = projectID
	}
	if versionID != "" {
		q["versionId"] = versionID
	}
	raw, err := rt.client.Get("cycle", q)
	if err != nil {
		return nil, err
	}
	cycles, ok := zephyrExtractCycles(raw)
	if !ok {
		cycles = nil
	}
	exact := zephyrMatchingCycles(cycles, name, true)
	if len(exact) == 0 {
		exact = zephyrMatchingCycles(cycles, name, false)
	}
	if len(exact) == 0 {
		return nil, zapi.NewError("zephyr_cycle_not_found", "Zephyr cycle was not found: "+name, "Run jira zephyr cycle list --project <PROJECT> --json to inspect cycle names.", 404)
	}
	if len(exact) > 1 {
		return nil, zapi.NewError("ambiguous_zephyr_cycle", "Multiple Zephyr cycles matched: "+name, "Candidates: "+zephyrCycleCandidateHint(exact), 409)
	}
	cycle := exact[0]
	out := map[string]interface{}{"raw": cycle}
	addStringField(out, "cycle_id", zephyrStringField(cycle, "id"))
	addStringField(out, "name", zephyrStringField(cycle, "name"))
	addStringField(out, "project_id", firstNonEmpty(zephyrStringField(cycle, "projectId"), projectID))
	addStringField(out, "version_id", firstNonEmpty(zephyrStringField(cycle, "versionId"), versionID))
	return out, nil
}

func zephyrMatchingCycles(cycles []map[string]interface{}, name string, caseSensitive bool) []map[string]interface{} {
	name = strings.TrimSpace(name)
	var out []map[string]interface{}
	for _, cycle := range cycles {
		cycleName := strings.TrimSpace(zephyrStringField(cycle, "name"))
		if caseSensitive && cycleName == name {
			out = append(out, cycle)
		}
		if !caseSensitive && strings.EqualFold(cycleName, name) {
			out = append(out, cycle)
		}
	}
	return out
}

func zephyrCycleCandidateHint(cycles []map[string]interface{}) string {
	parts := make([]string, 0, len(cycles))
	for _, cycle := range cycles {
		fields := []string{}
		if id := zephyrStringField(cycle, "id"); id != "" {
			fields = append(fields, "cycle_id="+id)
		}
		if name := zephyrStringField(cycle, "name"); name != "" {
			fields = append(fields, "name="+name)
		}
		if projectID := zephyrStringField(cycle, "projectId"); projectID != "" {
			fields = append(fields, "project_id="+projectID)
		}
		if versionID := zephyrStringField(cycle, "versionId"); versionID != "" {
			fields = append(fields, "version_id="+versionID)
		}
		parts = append(parts, "{"+strings.Join(fields, ", ")+"}")
	}
	return strings.Join(parts, ", ")
}

func zephyrLookupString(m map[string]interface{}, key string) string {
	if m == nil || key == "" {
		return ""
	}
	if v, ok := m[key]; ok {
		return zephyrAnyString(v)
	}
	parts := strings.Split(key, ".")
	current := interface{}(m)
	for _, part := range parts {
		child, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = child[part]
		if !ok {
			return ""
		}
	}
	return zephyrAnyString(current)
}

func zephyrAnyString(v interface{}) string {
	if v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func addStringField(m map[string]interface{}, key, value string) {
	if strings.TrimSpace(value) != "" {
		m[key] = value
	}
}
