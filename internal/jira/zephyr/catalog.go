package zephyr

import (
	"fmt"
	"sort"
	"strings"
)

type Endpoint struct {
	ID           string `json:"id"`
	Group        string `json:"group"`
	Method       string `json:"method"`
	PathTemplate string `json:"path_template"`
	Summary      string `json:"summary"`
	Command      string `json:"command"`
	RawExample   string `json:"raw_example"`
}

func OfficialEndpointCatalog() []Endpoint {
	out := make([]Endpoint, 0, len(officialEndpoints))
	for _, endpoint := range officialEndpoints {
		if endpoint.Command == "" {
			endpoint.Command = rawCommand(endpoint)
		}
		if endpoint.RawExample == "" {
			endpoint.RawExample = rawExample(endpoint)
		}
		out = append(out, endpoint)
	}
	return out
}

func OfficialEndpointGroups() []string {
	seen := map[string]bool{}
	var groups []string
	for _, endpoint := range officialEndpoints {
		if seen[endpoint.Group] {
			continue
		}
		seen[endpoint.Group] = true
		groups = append(groups, endpoint.Group)
	}
	sort.Strings(groups)
	return groups
}

func FindOfficialEndpoint(id string) (Endpoint, bool) {
	id = strings.TrimSpace(id)
	for _, endpoint := range OfficialEndpointCatalog() {
		if endpoint.ID == id {
			return endpoint, true
		}
	}
	return Endpoint{}, false
}

func rawCommand(endpoint Endpoint) string {
	verb := strings.ToLower(endpoint.Method)
	cmd := "jira zephyr api " + verb + " " + rawPathExample(endpoint.PathTemplate)
	if endpoint.Method == "DELETE" {
		cmd += " --yes"
	}
	return cmd
}

func rawExample(endpoint Endpoint) string {
	cmd := rawCommand(endpoint)
	switch endpoint.Method {
	case "POST", "PUT", "PATCH":
		cmd += " --body '{}'"
	}
	return cmd + " --json"
}

func rawPathExample(pathTemplate string) string {
	path := pathWithoutQueryTemplate(pathTemplate)
	repl := strings.NewReplacer(
		"{id}", "30000",
		"{cycleId}", "20000",
		"{folderId}", "40000",
		"{issueId}", "10001",
		"{fromStepId}", "10",
		"{fileid}", "50000",
		"{token}", "token",
		"{jobProgressToken}", "token",
		"{scheduleId}", "30000",
	)
	return strings.TrimLeft(repl.Replace(path), "/")
}

func pathWithoutQueryTemplate(pathTemplate string) string {
	path := strings.TrimSpace(pathTemplate)
	if i := strings.Index(path, "{?"); i >= 0 {
		path = path[:i]
	}
	return strings.TrimRight(path, "?")
}

func CatalogEnvelope() map[string]interface{} {
	endpoints := OfficialEndpointCatalog()
	return map[string]interface{}{
		"base_path":      DefaultRESTPath,
		"endpoint_count": len(endpoints),
		"groups":         OfficialEndpointGroups(),
		"endpoints":      endpoints,
	}
}

func DescribeEndpoint(id string) (Endpoint, error) {
	endpoint, ok := FindOfficialEndpoint(id)
	if !ok {
		return Endpoint{}, NewError("zephyr_endpoint_not_found", fmt.Sprintf("unknown Zephyr endpoint id %q", id), "Run jira zephyr api catalog --json to list endpoint ids.", 404)
	}
	return endpoint, nil
}

var officialEndpoints = []Endpoint{
	{ID: "chart.issue-statuses", Group: "ChartResource", Method: "GET", PathTemplate: "/zchart/issueStatuses{?projectId}", Summary: "Get Issue Status by Project"},
	{ID: "chart.tests-created", Group: "ChartResource", Method: "GET", PathTemplate: "/zchart/testsCreated{?projectKey,daysPrevious,periodName}", Summary: "Generate Test Created Data"},

	{ID: "zql.execute-search", Group: "ExecutionSearchResource", Method: "GET", PathTemplate: "/zql/executeSearch{?zqlQuery,filterId,offset,maxRecords,expand}", Summary: "Execute Search to Get Search Result", Command: "jira zephyr zql search"},
	{ID: "zql.clauses", Group: "ExecutionSearchResource", Method: "GET", PathTemplate: "/zql/clauses", Summary: "Get Search Clauses", Command: "jira zephyr zql clauses"},
	{ID: "zql.autocomplete-json", Group: "ExecutionSearchResource", Method: "GET", PathTemplate: "/zql/autocompleteZQLJson", Summary: "Get AutoComplete ZQL Json", Command: "jira zephyr zql autocomplete-json"},

	{ID: "zql-filter.get", Group: "ZQLFilterResource", Method: "GET", PathTemplate: "/zql/executionFilter/{id}", Summary: "Get ZQL filter"},
	{ID: "zql-filter.list", Group: "ZQLFilterResource", Method: "GET", PathTemplate: "/zql/executionFilter{?byUser,fav,offset,maxRecords}", Summary: "Get All Execution Filters"},
	{ID: "zql-filter.search", Group: "ZQLFilterResource", Method: "GET", PathTemplate: "/zql/executionFilter/search{?filterName,owner,sharePerm}", Summary: "Search Execution Filters"},
	{ID: "zql-filter.quick-search", Group: "ZQLFilterResource", Method: "GET", PathTemplate: "/zql/executionFilter/quickSearch{?query}", Summary: "Quick Search ZQL Filters"},
	{ID: "zql-filter.copy", Group: "ZQLFilterResource", Method: "PUT", PathTemplate: "/zql/executionFilter/copy", Summary: "Copy a ZQL Filter"},
	{ID: "zql-filter.create", Group: "ZQLFilterResource", Method: "POST", PathTemplate: "/zql/executionFilter", Summary: "Create Execution Filter"},
	{ID: "zql-filter.update", Group: "ZQLFilterResource", Method: "PUT", PathTemplate: "/zql/executionFilter/update", Summary: "Update The ZQL Filter"},
	{ID: "zql-filter.rename", Group: "ZQLFilterResource", Method: "PUT", PathTemplate: "/zql/executionFilter/rename", Summary: "Rename a ZQL Filter"},
	{ID: "zql-filter.toggle-favorite", Group: "ZQLFilterResource", Method: "PUT", PathTemplate: "/zql/executionFilter/toggleFav", Summary: "Toggle ZQL filter isFavorites"},
	{ID: "zql-filter.delete", Group: "ZQLFilterResource", Method: "DELETE", PathTemplate: "/zql/executionFilter/{id}", Summary: "Deletes a ZQL filter"},
	{ID: "zql-filter.user", Group: "ZQLFilterResource", Method: "GET", PathTemplate: "/zql/executionFilter/user", Summary: "Get LoggedIn User"},

	{ID: "cycle.get", Group: "CycleResource", Method: "GET", PathTemplate: "/cycle/{id}", Summary: "Get Cycle Information", Command: "jira zephyr cycle get <cycle-id>"},
	{ID: "cycle.export", Group: "CycleResource", Method: "GET", PathTemplate: "/cycle/{id}/export{?versionId,projectId,folderId}", Summary: "Export Cycle Data"},
	{ID: "cycle.create", Group: "CycleResource", Method: "POST", PathTemplate: "/cycle", Summary: "Create New Cycle", Command: "jira zephyr cycle create"},
	{ID: "cycle.update", Group: "CycleResource", Method: "PUT", PathTemplate: "/cycle", Summary: "Update Cycle Information", Command: "jira zephyr cycle update <cycle-id>"},
	{ID: "cycle.delete", Group: "CycleResource", Method: "DELETE", PathTemplate: "/cycle/{id}{?isFolderCycleDelete}", Summary: "Delete Cycle", Command: "jira zephyr cycle delete <cycle-id>"},
	{ID: "cycle.list", Group: "CycleResource", Method: "GET", PathTemplate: "/cycle{?projectId,versionId,id,offset,issueId,expand}", Summary: "Get List of Cycle", Command: "jira zephyr cycle list"},
	{ID: "cycle.move-executions", Group: "CycleResource", Method: "PUT", PathTemplate: "/cycle/{id}/move", Summary: "Move Executions to Cycle"},
	{ID: "cycle.cycles-by-versions-and-sprint", Group: "CycleResource", Method: "POST", PathTemplate: "/cycle/cyclesByVersionsAndSprint", Summary: "Get Cycles By Versions/Sprint"},
	{ID: "cycle.cleanup-sprints", Group: "CycleResource", Method: "POST", PathTemplate: "/cycle/cleanupSprints", Summary: "Clean Up Sprint From Cycle"},
	{ID: "cycle.folders", Group: "CycleResource", Method: "GET", PathTemplate: "/cycle/{cycleId}/folders{?projectId,versionId,limit,offset}", Summary: "Get the list of folder for a cycle", Command: "jira zephyr folder list"},
	{ID: "cycle.move-executions-to-folder", Group: "CycleResource", Method: "PUT", PathTemplate: "/cycle/{cycleId}/move/executions/folder/{folderId}", Summary: "Move selected executions or all executions from cycle to folder"},
	{ID: "cycle.copy-executions", Group: "CycleResource", Method: "PUT", PathTemplate: "/cycle/{id}/copy", Summary: "Copy Executions to Cycle"},

	{ID: "znav.available-columns", Group: "ZNavResource", Method: "GET", PathTemplate: "/znav/availableColumns{?executionFilterId}", Summary: "Get Available Columns"},
	{ID: "znav.create-column-selection", Group: "ZNavResource", Method: "POST", PathTemplate: "/znav/createColumnSelection", Summary: "Create Column Selection"},
	{ID: "znav.update-column-selection", Group: "ZNavResource", Method: "PUT", PathTemplate: "/znav/updateColumnSelection/{id}", Summary: "Update Column Selection"},

	{ID: "license.get", Group: "LicenseResource", Method: "GET", PathTemplate: "/license", Summary: "Get License Status Information"},

	{ID: "preference.set-teststep-customization", Group: "PreferenceResource", Method: "POST", PathTemplate: "/preference/setteststepcustomization", Summary: "Set test step customization preference."},
	{ID: "preference.get-teststep-customization", Group: "PreferenceResource", Method: "GET", PathTemplate: "/preference/getteststepcustomization", Summary: "Get test step customization preference."},
	{ID: "preference.set-cycle-summary-customization", Group: "PreferenceResource", Method: "POST", PathTemplate: "/preference/setcyclesummarycustomization", Summary: "Set cycle summary columns customization preference."},
	{ID: "preference.get-cycle-summary-customization", Group: "PreferenceResource", Method: "GET", PathTemplate: "/preference/getcyclesummarycustomization", Summary: "Get cycle summary customization preference."},
	{ID: "preference.set-execution-customization", Group: "PreferenceResource", Method: "POST", PathTemplate: "/preference/setexecutioncustomization", Summary: "Set execution summary columns customization preference."},
	{ID: "preference.get-execution-customization", Group: "PreferenceResource", Method: "GET", PathTemplate: "/preference/getexecutioncustomization", Summary: "Get execution summary customization preference."},

	{ID: "step-result.list", Group: "StepResultResource", Method: "GET", PathTemplate: "/stepResult{?executionId,expand}", Summary: "Get list of Step Result", Command: "jira zephyr step-result list"},
	{ID: "step-result.get", Group: "StepResultResource", Method: "GET", PathTemplate: "/stepResult/{id}{?expand}", Summary: "Get StepResult Information"},
	{ID: "step-result.create", Group: "StepResultResource", Method: "POST", PathTemplate: "/stepResult", Summary: "Create New StepResult"},
	{ID: "step-result.update", Group: "StepResultResource", Method: "PUT", PathTemplate: "/stepResult/{id}", Summary: "Update StepResult Information", Command: "jira zephyr step-result update-status <step-result-id>"},
	{ID: "step-result.defects", Group: "StepResultResource", Method: "GET", PathTemplate: "/stepResult/{id}/defects", Summary: "Get List of StepDefect"},
	{ID: "step-result.step-defects", Group: "StepResultResource", Method: "GET", PathTemplate: "/stepResult/stepDefects{?executionId,expand}", Summary: "Get List of StepDefect by Execution"},

	{ID: "traceability.defect-statistics", Group: "TraceabilityResource", Method: "GET", PathTemplate: "/traceability/defectStatistics{?defectIdOrKeyList}", Summary: "Search Defect Statistics"},
	{ID: "traceability.executions-by-defect", Group: "TraceabilityResource", Method: "GET", PathTemplate: "/traceability/executionsByDefect{?defectIdOrKey,maxRecords,offset}", Summary: "Search Execution by Defect"},
	{ID: "traceability.executions-by-test", Group: "TraceabilityResource", Method: "GET", PathTemplate: "/traceability/executionsByTest{?testIdOrKey,maxRecords,offset}", Summary: "Get List of Search Execution By Test"},
	{ID: "traceability.tests-by-requirement", Group: "TraceabilityResource", Method: "GET", PathTemplate: "/traceability/testsByRequirement{?requirementIdOrKeyList}", Summary: "Get List of Search Test by Requirement"},
	{ID: "traceability.export", Group: "TraceabilityResource", Method: "POST", PathTemplate: "/traceability/export", Summary: "Export Traceability Report"},

	{ID: "test.count", Group: "TestcaseResource", Method: "GET", PathTemplate: "/test/count{?projectId,versionId,groupFld}", Summary: "Get Test Count List"},
	{ID: "test.my-searches", Group: "TestcaseResource", Method: "GET", PathTemplate: "/test/mySearches/{id}/", Summary: "Get List of Saved Searches"},
	{ID: "test.add-issue-link", Group: "TestcaseResource", Method: "POST", PathTemplate: "/test/addIssueLink{?parentIssueId,testcaseId}", Summary: "Add Issue Link"},
	{ID: "test.summary-by-label", Group: "TestcaseResource", Method: "GET", PathTemplate: "/test/summary/testsbylabel{?projectId,labelName,offset,maxRecords}", Summary: "Fetch Tests By Label"},
	{ID: "test.summary-by-component", Group: "TestcaseResource", Method: "GET", PathTemplate: "/test/summary/testsbycomponent{?projectId,componentName,offset,maxRecords}", Summary: "Fetch Tests By Component"},
	{ID: "test.summary-by-version", Group: "TestcaseResource", Method: "GET", PathTemplate: "/test/summary/testsbyversion{?projectId,versionName,offset,maxRecords}", Summary: "Fetch Tests By Version"},

	{ID: "util.version-board-list", Group: "UtilResource", Method: "GET", PathTemplate: "/util/versionBoard-list{?projectId,versionId}", Summary: "Get All Versions", Command: "jira zephyr version list"},
	{ID: "util.zephyr-test-issue-type", Group: "UtilResource", Method: "GET", PathTemplate: "/util/zephyrTestIssueType", Summary: "Get Zephyr IssueType", Command: "jira zephyr util test-issue-type"},
	{ID: "util.project-list", Group: "UtilResource", Method: "GET", PathTemplate: "/util/project-list", Summary: "Get All Projects"},
	{ID: "util.all-versions-text", Group: "UtilResource", Method: "GET", PathTemplate: "/util/allversionstext", Summary: "Get All Versions Text"},
	{ID: "util.sprints-by-project-and-version", Group: "UtilResource", Method: "GET", PathTemplate: "/util/sprintsByProjectAndVersion{?projectId,versionId}", Summary: "Get List of Sprints"},
	{ID: "util.cycle-criteria-info", Group: "UtilResource", Method: "GET", PathTemplate: "/util/cycleCriteriaInfo{?projectId}", Summary: "Get Cycle Criteria Info"},
	{ID: "util.test-execution-status", Group: "UtilResource", Method: "GET", PathTemplate: "/util/testExecutionStatus", Summary: "Get Execution Statuses, Priorities, Components, Labels", Command: "jira zephyr status list"},
	{ID: "util.teststep-execution-status", Group: "UtilResource", Method: "GET", PathTemplate: "/util/teststepExecutionStatus", Summary: "Get Test Step Execution Statuses, Priorities, Components, Labels", Command: "jira zephyr status list"},
	{ID: "util.dashboard", Group: "UtilResource", Method: "GET", PathTemplate: "/util/dashboard{?query,maxRecords}", Summary: "Get Dashboard Summary"},
	{ID: "util.render", Group: "UtilResource", Method: "POST", PathTemplate: "/util/render", Summary: "Convert Markup to HTML"},
	{ID: "util.component-list", Group: "UtilResource", Method: "GET", PathTemplate: "/util/component-list{?projectId}", Summary: "Get Components"},
	{ID: "util.teststatus-list", Group: "UtilResource", Method: "GET", PathTemplate: "/util/teststatus-list", Summary: "Get Test Status List"},

	{ID: "folder.create", Group: "FolderResource", Method: "POST", PathTemplate: "/folder/create", Summary: "Create a folder under cycle", Command: "jira zephyr folder create"},
	{ID: "folder.update", Group: "FolderResource", Method: "PUT", PathTemplate: "/folder/{folderId}", Summary: "Update a folder information", Command: "jira zephyr folder update <folder-id>"},
	{ID: "folder.delete", Group: "FolderResource", Method: "DELETE", PathTemplate: "/folder/{folderId}{?projectId,versionId,cycleId}", Summary: "Delete a folder under a cycle", Command: "jira zephyr folder delete <folder-id>"},

	{ID: "execution.get", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/{id}{?expand}", Summary: "Get Execution Information", Command: "jira zephyr execution get <execution-id>"},
	{ID: "execution.list", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution{?issueId,projectId,versionId,cycleId,offset,action,sorter,expand,limit,folderId}", Summary: "Get List of Execution", Command: "jira zephyr execution list"},
	{ID: "execution.defects", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/{id}/defects", Summary: "Get Defect List", Command: "jira zephyr defect list"},
	{ID: "execution.add-assignee", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/{scheduleId}{?assignee}", Summary: "Add Assignee to Execution"},
	{ID: "execution.create", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution", Summary: "Create New Execution", Command: "jira zephyr execution create"},
	{ID: "execution.count-summary", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/count{?projectId,versionId,groupFld,cycleId,sprintId,daysPrevious,periodName,graphType}", Summary: "Get Execution Count Summary", Command: "jira zephyr execution count"},
	{ID: "execution.top-defects", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/topDefects{?projectId,versionId,issueStatuses,howMany}", Summary: "Get Top Defect By Issue Status"},
	{ID: "execution.index-all", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/indexAll", Summary: "Re Index All Execution"},
	{ID: "execution.index-current-node", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/indexCurrentNode", Summary: "Re-Index All Execution for Current Node."},
	{ID: "execution.reindex-by-project", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/reindex/byProject{?projectIds}", Summary: "Re Index All Execution for given project id(s)."},
	{ID: "execution.index-status", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/indexStatus/{token}", Summary: "Get Index Status"},
	{ID: "execution.job-progress", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/jobProgress/{jobProgressToken}{?type}", Summary: "Get job progress status"},
	{ID: "execution.update-bulk-defects", Group: "ExecutionResource", Method: "PUT", PathTemplate: "/execution/updateWithBulkDefects", Summary: "Update Bulk Defects"},
	{ID: "execution.update-status", Group: "ExecutionResource", Method: "PUT", PathTemplate: "/execution/{id}/execute", Summary: "Update Execution Details", Command: "jira zephyr execution update-status <execution-id>", RawExample: "jira zephyr api put execution/30000/execute --body '{\"status\":\"1\"}' --json"},
	{ID: "execution.delete", Group: "ExecutionResource", Method: "DELETE", PathTemplate: "/execution/{id}", Summary: "Delete Execution", Command: "jira zephyr execution delete <execution-id>"},
	{ID: "execution.add-tests-to-cycle", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/addTestsToCycle/", Summary: "Add Tests to Cycle", Command: "jira zephyr execution add-tests-to-cycle"},
	{ID: "execution.refresh-links-status", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/refreshLinksStatus/{token}", Summary: "Refresh Issue Link Status"},
	{ID: "execution.export", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/export", Summary: "Export Execution", Command: "jira zephyr execution export"},
	{ID: "execution.navigator", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/navigator/{id}{?zql,offset,expand}", Summary: "Navigate Execution"},
	{ID: "execution.reorder", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/reorder", Summary: "Re Order Execution"},
	{ID: "execution.summaries-by-sprint-and-issue", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/executionSummariesBySprintAndIssue", Summary: "Get Execution Summaries By Sprint And Issue"},
	{ID: "execution.executions-by-issue", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/executionsByIssue{?issueIdOrKey,action,offset,maxRecords,expand}", Summary: "Get Execution Summary by Issue"},
	{ID: "execution.status-count-for-cycle-by-project-version", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/executionsStatusCountForCycleByProjectIdAndVersion{?projectId,versionId,components,offset,limit}", Summary: "Get Executions count for cycles by given project id and version id"},
	{ID: "execution.status-count-by-cycle", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/executionsStatusCountByCycle{?projectId,versionId,cycles,folders,offset,limit}", Summary: "Get Executions count for given cycle"},
	{ID: "execution.status-count-per-assignee-for-cycle", Group: "ExecutionResource", Method: "GET", PathTemplate: "/execution/executionsStatusCountPerAssigneeForCycle{?projectId,versionId,cycles,offset,limit}", Summary: "Get Executions count per assignee for given cycle"},
	{ID: "execution.delete-bulk", Group: "ExecutionResource", Method: "DELETE", PathTemplate: "/execution/deleteExecutions", Summary: "Delete bulk Execution"},
	{ID: "execution.refresh-remote-links", Group: "ExecutionResource", Method: "POST", PathTemplate: "/execution/refreshRemoteLinks{?issueLinkTypeId}", Summary: "Refresh Issue/Remote Link"},
	{ID: "execution.bulk-assign", Group: "ExecutionResource", Method: "PUT", PathTemplate: "/execution/bulkAssign", Summary: "Assign Bulk Executions"},
	{ID: "execution.update-bulk-status", Group: "ExecutionResource", Method: "PUT", PathTemplate: "/execution/updateBulkStatus", Summary: "Update Bulk Execution Status", Command: "jira zephyr execution bulk-update-status"},
	{ID: "execution.archive", Group: "ExecutionArchiveResource", Method: "POST", PathTemplate: "/execution/archive", Summary: "Archive Executions", Command: "jira zephyr archive executions"},
	{ID: "execution.restore", Group: "ExecutionArchiveResource", Method: "POST", PathTemplate: "/execution/restore", Summary: "Restore Archived Executions", Command: "jira zephyr archive restore"},
	{ID: "execution.archive-list", Group: "ExecutionArchiveResource", Method: "GET", PathTemplate: "/execution/archive{?projectId,versionId,cycleId,folderId,offset,maxRecords}", Summary: "Get Archived Executions", Command: "jira zephyr archive list"},
	{ID: "execution.archive-export", Group: "ExecutionArchiveResource", Method: "POST", PathTemplate: "/execution/archive/export", Summary: "Export Archived Executions", Command: "jira zephyr archive export"},

	{ID: "issue-picker.issues", Group: "IssuePickerResource", Method: "GET", PathTemplate: "/issues{?query,currentJQL,currentIssueKey,currentProjectId,showSubTasks,showSubTaskParent}", Summary: "Get Issues for Test"},
	{ID: "issue-picker.default", Group: "IssuePickerResource", Method: "GET", PathTemplate: "/issues/default{?project}", Summary: "Get Default Issue Type"},

	{ID: "audit.list", Group: "AuditResource", Method: "GET", PathTemplate: "/audit{?entityType,event,user,offset,maxRecords,issueId,executionId}", Summary: "Get Audit Log"},

	{ID: "teststep.delete", Group: "TeststepResource", Method: "DELETE", PathTemplate: "/teststep/{issueId}/{id}", Summary: "Delete TestStep", Command: "jira zephyr teststep delete"},
	{ID: "teststep.get", Group: "TeststepResource", Method: "GET", PathTemplate: "/teststep/{issueId}/{id}", Summary: "Get TestStep Information", Command: "jira zephyr teststep get"},
	{ID: "teststep.list", Group: "TeststepResource", Method: "GET", PathTemplate: "/teststep/{issueId}?offset=0&limit=50", Summary: "Get List of TestSteps", Command: "jira zephyr teststep list"},
	{ID: "teststep.update", Group: "TeststepResource", Method: "PUT", PathTemplate: "/teststep/{issueId}/{id}", Summary: "Update TestStep Data", Command: "jira zephyr teststep update"},
	{ID: "teststep.create", Group: "TeststepResource", Method: "POST", PathTemplate: "/teststep/{issueId}", Summary: "Create New TestStep", Command: "jira zephyr teststep create"},
	{ID: "teststep.move", Group: "TeststepResource", Method: "POST", PathTemplate: "/teststep/{issueId}/{id}/move", Summary: "Move TestStep to Issue"},
	{ID: "teststep.clone", Group: "TeststepResource", Method: "POST", PathTemplate: "/teststep/{issueId}/clone/{fromStepId}", Summary: "Clone TestStep"},
	{ID: "teststep.copy", Group: "TeststepResource", Method: "POST", PathTemplate: "/teststep/{issueId}/copyteststeps", Summary: "Copy Test steps from source to destination issues"},

	{ID: "attachment.delete", Group: "AttachmentResource", Method: "DELETE", PathTemplate: "/attachment/{id}", Summary: "Delete Attachment", Command: "jira zephyr attachment delete <attachment-id>"},
	{ID: "attachment.add", Group: "AttachmentResource", Method: "POST", PathTemplate: "/attachment{?entityId,entityType}", Summary: "Add Attachment into Entity", Command: "jira zephyr attachment upload"},
	{ID: "attachment.get", Group: "AttachmentResource", Method: "GET", PathTemplate: "/attachment/{id}", Summary: "Get Single Attachment", Command: "jira zephyr attachment get <attachment-id>"},
	{ID: "attachment.list", Group: "AttachmentResource", Method: "GET", PathTemplate: "/attachment/attachmentsByEntity{?entityId,entityType}", Summary: "Get Attachment By Entity", Command: "jira zephyr attachment list"},
	{ID: "attachment.file", Group: "AttachmentResource", Method: "GET", PathTemplate: "/attachment/{fileid}/file", Summary: "Get Attachment File"},

	{ID: "customfield.create", Group: "CustomFieldResource", Method: "POST", PathTemplate: "/customfield/create", Summary: "Create Custom Field", Command: "jira zephyr customfield create"},
	{ID: "customfield.update", Group: "CustomFieldResource", Method: "PUT", PathTemplate: "/customfield/{id}", Summary: "Update Custom Field", Command: "jira zephyr customfield update <customfield-id>"},
	{ID: "customfield.get", Group: "CustomFieldResource", Method: "GET", PathTemplate: "/customfield/{id}", Summary: "Get Custom Field", Command: "jira zephyr customfield get <customfield-id>"},
	{ID: "customfield.list-by-entity", Group: "CustomFieldResource", Method: "GET", PathTemplate: "/customfield/entity{?entityType}", Summary: "Get Custom Fields by Entity Type", Command: "jira zephyr customfield list"},
	{ID: "customfield.list-by-entity-and-project", Group: "CustomFieldResource", Method: "GET", PathTemplate: "/customfield/byEntityTypeAndProject{?entityType,projectId}", Summary: "Get Custom Fields by Entity Type and Project", Command: "jira zephyr customfield list"},
	{ID: "customfield.delete", Group: "CustomFieldResource", Method: "DELETE", PathTemplate: "/customfield/{id}", Summary: "Delete Custom Field", Command: "jira zephyr customfield delete <customfield-id>"},
	{ID: "customfield.delete-bulk", Group: "CustomFieldResource", Method: "DELETE", PathTemplate: "/customfield/delete-customfields", Summary: "Delete Custom Fields", Command: "jira zephyr customfield delete-bulk"},
	{ID: "customfield.enable", Group: "CustomFieldResource", Method: "DELETE", PathTemplate: "/customfield/{id}/{projectId}{?enable}", Summary: "Enable or Disable Custom Field for Project", Command: "jira zephyr customfield enable <customfield-id>"},

	{ID: "zapi.module-info", Group: "ZAPIResource", Method: "GET", PathTemplate: "/moduleInfo", Summary: "Get ZAPI Module Status", Command: "jira zephyr doctor"},
	{ID: "zql.autocomplete", Group: "ZQLAutoCompleteResource", Method: "GET", PathTemplate: "/zql/autocomplete{?fieldName,fieldValue}", Summary: "Get ZQL Auto Complete Result", Command: "jira zephyr zql autocomplete"},
	{ID: "system-info.get", Group: "SystemInfoResource", Method: "GET", PathTemplate: "/systemInfo", Summary: "Get System Information", Command: "jira zephyr doctor"},
	{ID: "filter-picker.search", Group: "FilterPickerResource", Method: "GET", PathTemplate: "/picker/filters{?query}", Summary: "Get Search For Filter"},
}
