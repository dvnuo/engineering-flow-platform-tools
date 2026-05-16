package commands

import (
	"errors"
	"fmt"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/jira"
	"engineering-flow-platform-tools/internal/jira/bulkcsv"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func issueMapCSVCmd(o *Opts) *cobra.Command {
	cmd := &cobra.Command{Use: "map-csv", RunE: func(cmd *cobra.Command, args []string) error {
		if mustS(cmd, "from-csv") == "" || mustS(cmd, "template-issue") == "" {
			return invalidArgs(cmd, o, "--from-csv and --template-issue required", "Use jira issue map-csv --from-csv testcases.csv --template-issue QA-1234 --output mapping-plan.json --json.")
		}
		plan, err := buildCSVMappingPlan(o, cmd)
		if err != nil {
			return printBulkCSVError(cmd, o, err)
		}
		if out := mustS(cmd, "output"); out != "" {
			if err := bulkcsv.WritePrettyJSON(out, plan); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "failed to write --output: "+err.Error(), "Choose a writable output path.", 400))
			}
		}
		data := map[string]interface{}{
			"summary": map[string]interface{}{
				"csv_rows":              plan.CSV.RowCount,
				"mapped_columns":        len(plan.FieldMappings),
				"ambiguous_columns":     len(plan.AmbiguousColumns),
				"unmapped_columns":      len(plan.UnmappedColumns),
				"requires_confirmation": len(plan.RequiresConfirmation),
				"warnings":              len(plan.Warnings),
			},
			"plan": plan,
		}
		return print(cmd, o, output.Success(plan.Jira.Instance, data))
	}}
	cmd.Flags().String("from-csv", "", "")
	cmd.Flags().String("template-issue", "", "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().String("field-catalog", "", "")
	cmd.Flags().String("example-issue", "", "")
	cmd.Flags().String("create-meta", "", "")
	cmd.Flags().String("edit-meta", "", "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Int("sample-rows", 5, "")
	cmd.Flags().Float64("min-confidence", 0.75, "")
	cmd.Flags().Bool("include-template-defaults", true, "")
	return cmd
}

func issueBulkCreateCmd(o *Opts) *cobra.Command {
	cmd := &cobra.Command{Use: "bulk-create", RunE: func(cmd *cobra.Command, args []string) error {
		return runIssueBulkCreate(o, cmd, false)
	}}
	addBulkCreateFlags(cmd)
	return cmd
}

func issueBulkValidateCmd(o *Opts) *cobra.Command {
	cmd := &cobra.Command{Use: "bulk-validate", RunE: func(cmd *cobra.Command, args []string) error {
		return runIssueBulkCreate(o, cmd, true)
	}}
	addBulkCreateFlags(cmd)
	return cmd
}

func addBulkCreateFlags(cmd *cobra.Command) {
	cmd.Flags().String("from-csv", "", "")
	cmd.Flags().String("mapping", "", "")
	cmd.Flags().String("template-issue", "", "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Int("max-create", 0, "")
	cmd.Flags().Bool("fail-fast", false, "")
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("type", "", "")
	cmd.Flags().String("field-catalog", "", "")
	cmd.Flags().String("example-issue", "", "")
	cmd.Flags().String("create-meta", "", "")
	cmd.Flags().String("edit-meta", "", "")
	cmd.Flags().Int("sample-rows", 5, "")
	cmd.Flags().Float64("min-confidence", 0.75, "")
	cmd.Flags().Bool("include-template-defaults", true, "")
}

func buildCSVMappingPlan(o *Opts, cmd *cobra.Command) (bulkcsv.MappingPlan, error) {
	csvPath := mustS(cmd, "from-csv")
	sampleRows, _ := cmd.Flags().GetInt("sample-rows")
	csvData, err := bulkcsv.ParseCSV(csvPath, sampleRows)
	if err != nil {
		return bulkcsv.MappingPlan{}, err
	}
	minConfidence, _ := cmd.Flags().GetFloat64("min-confidence")
	includeDefaults, _ := cmd.Flags().GetBool("include-template-defaults")
	templateIssue := mustS(cmd, "template-issue")
	metadata, err := loadBulkCSVMetadata(o, cmd, templateIssue)
	if err != nil {
		return bulkcsv.MappingPlan{}, err
	}
	input := bulkcsv.MappingInput{
		CSV:                     csvData.Summary,
		Rows:                    csvData.Rows,
		FieldCatalog:            metadata.fieldCatalog,
		ExampleIssue:            metadata.exampleIssue,
		CreateMeta:              metadata.createMeta,
		EditMeta:                metadata.editMeta,
		Jira:                    metadata.jiraInfo,
		MinConfidence:           minConfidence,
		IncludeTemplateDefaults: includeDefaults,
	}
	return bulkcsv.BuildMappingPlan(input)
}

type bulkCSVMetadata struct {
	fieldCatalog interface{}
	exampleIssue map[string]interface{}
	createMeta   map[string]interface{}
	editMeta     map[string]interface{}
	jiraInfo     bulkcsv.JiraInfo
}

func loadBulkCSVMetadata(o *Opts, cmd *cobra.Command, templateIssue string) (bulkCSVMetadata, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return bulkCSVMetadata{}, err
	}
	ctx, err := jira.NewContext(cfg, o.Instance, templateIssue, o.DryRun)
	if err != nil {
		return bulkCSVMetadata{}, err
	}
	meta := bulkCSVMetadata{jiraInfo: bulkcsv.JiraInfo{Instance: ctx.Instance, TemplateIssue: templateIssue, ProjectKey: mustS(cmd, "project"), IssueTypeName: mustS(cmd, "type")}}

	if path := mustS(cmd, "field-catalog"); path != "" {
		v, err := bulkcsv.LoadJSONValue(path)
		if err != nil {
			return meta, err
		}
		meta.fieldCatalog = unwrapEnvelopeValue(v)
	} else {
		v, err := getJiraValue(ctx, "field", nil)
		if err != nil {
			return meta, err
		}
		meta.fieldCatalog = v
	}

	if path := mustS(cmd, "example-issue"); path != "" {
		v, err := bulkcsv.LoadJSONObject(path)
		if err != nil {
			return meta, err
		}
		meta.exampleIssue = unwrapEnvelopeMap(v)
	} else {
		v, err := getJiraMap(ctx, "issue/"+jira.IssueKey(templateIssue), map[string]string{"fields": "*all", "expand": "names,schema,editmeta"})
		if err != nil {
			return meta, err
		}
		meta.exampleIssue = v
	}

	if path := mustS(cmd, "create-meta"); path != "" {
		v, err := bulkcsv.LoadJSONObject(path)
		if err != nil {
			return meta, err
		}
		meta.createMeta = unwrapEnvelopeMap(v)
	} else {
		v, err := fetchBulkCreateMeta(ctx, templateIssue, meta.jiraInfo.ProjectKey, meta.jiraInfo.IssueTypeName)
		if err != nil {
			return meta, err
		}
		meta.createMeta = v
	}

	if path := mustS(cmd, "edit-meta"); path != "" {
		v, err := bulkcsv.LoadJSONObject(path)
		if err != nil {
			return meta, err
		}
		meta.editMeta = unwrapEnvelopeMap(v)
	} else {
		v, err := getJiraMap(ctx, "issue/"+jira.IssueKey(templateIssue)+"/editmeta", nil)
		if err != nil {
			return meta, err
		}
		meta.editMeta = v
	}

	project := mapFromInterface(meta.createMeta["project"])
	issueType := mapFromInterface(meta.createMeta["issuetype"])
	meta.jiraInfo.ProjectKey = firstNonEmpty(meta.jiraInfo.ProjectKey, metadataString(project["key"]))
	meta.jiraInfo.ProjectID = firstNonEmpty(meta.jiraInfo.ProjectID, metadataString(project["id"]))
	meta.jiraInfo.IssueTypeName = firstNonEmpty(meta.jiraInfo.IssueTypeName, metadataString(issueType["name"]))
	meta.jiraInfo.IssueTypeID = firstNonEmpty(meta.jiraInfo.IssueTypeID, metadataString(issueType["id"]))
	return meta, nil
}

func fetchBulkCreateMeta(ctx *jira.Context, templateIssue, project, issueType string) (map[string]interface{}, error) {
	opts := createMetaOptions{FromIssue: templateIssue, ProjectKey: project, TypeName: issueType}
	if err := populateCreateMetaFromIssue(ctx, &opts); err != nil {
		return nil, err
	}
	out, err := fetchSplitCreateMeta(ctx, opts)
	if err == nil {
		return out, nil
	}
	if !isCreateMetaFallbackError(err) {
		return nil, err
	}
	return fetchLegacyCreateMeta(ctx, opts)
}

func runIssueBulkCreate(o *Opts, cmd *cobra.Command, forceDryRun bool) error {
	if mustS(cmd, "from-csv") == "" {
		return invalidArgs(cmd, o, "--from-csv required", "Use jira issue bulk-create --from-csv testcases.csv --mapping mapping-plan.json --dry-run --json.")
	}
	dryRun := o.DryRun || forceDryRun
	if !dryRun && !o.Yes {
		return print(cmd, o, output.Failure("confirmation_required", "--yes required for actual bulk create", "Run with --dry-run first, then pass --yes with a reviewed mapping file.", 400))
	}
	csvData, err := bulkcsv.ParseCSV(mustS(cmd, "from-csv"), 5)
	if err != nil {
		return printBulkCSVError(cmd, o, err)
	}
	plan, err := loadOrBuildBulkPlan(o, cmd, dryRun)
	if err != nil {
		return printBulkCSVError(cmd, o, err)
	}
	dryRunResult := bulkcsv.DryRunRows(csvData.Rows, plan, 5)
	if dryRun {
		if out := mustS(cmd, "output"); out != "" {
			if err := bulkcsv.WritePrettyJSON(out, dryRunResult); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "failed to write --output: "+err.Error(), "Choose a writable output path.", 400))
			}
		}
		return print(cmd, o, output.Success(plan.Jira.Instance, dryRunResult))
	}
	if dryRunResult.InvalidRows > 0 {
		return print(cmd, o, output.Failure("invalid_args", "CSV contains invalid rows; run --dry-run to inspect row errors", "No issues were created.", 400))
	}
	maxCreate, _ := cmd.Flags().GetInt("max-create")
	if maxCreate > 0 && dryRunResult.ValidRows > maxCreate {
		return print(cmd, o, output.Failure("invalid_args", "--max-create would be exceeded", "Increase --max-create or reduce the CSV rows.", 400))
	}
	result, err := createBulkIssues(o, cmd, csvData.Rows, plan)
	if err != nil {
		return printBulkCSVError(cmd, o, err)
	}
	if out := mustS(cmd, "output"); out != "" {
		if err := bulkcsv.WritePrettyJSON(out, result); err != nil {
			return print(cmd, o, output.Failure("invalid_args", "failed to write --output: "+err.Error(), "Choose a writable output path.", 400))
		}
	}
	return print(cmd, o, output.Success(plan.Jira.Instance, result))
}

func loadOrBuildBulkPlan(o *Opts, cmd *cobra.Command, dryRun bool) (bulkcsv.MappingPlan, error) {
	if path := mustS(cmd, "mapping"); path != "" {
		return bulkcsv.LoadMappingPlan(path)
	}
	if !dryRun {
		return bulkcsv.MappingPlan{}, &bulkcsv.Error{Code: "invalid_args", Message: "--mapping is required for actual create", Hint: "Review a mapping plan file before creating issues.", Status: 400}
	}
	if mustS(cmd, "template-issue") == "" {
		return bulkcsv.MappingPlan{}, &bulkcsv.Error{Code: "invalid_args", Message: "--template-issue required when --mapping is omitted", Status: 400}
	}
	return buildCSVMappingPlan(o, cmd)
}

func createBulkIssues(o *Opts, cmd *cobra.Command, rows []bulkcsv.CSVRow, plan bulkcsv.MappingPlan) (bulkcsv.CreateResult, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return bulkcsv.CreateResult{}, err
	}
	ctx, err := jira.NewContext(cfg, o.Instance, plan.Jira.TemplateIssue, false)
	if err != nil {
		return bulkcsv.CreateResult{}, err
	}
	failFast, _ := cmd.Flags().GetBool("fail-fast")
	result := bulkcsv.CreateResult{Rows: len(rows), Created: []bulkcsv.CreatedIssue{}, Failures: []bulkcsv.CreateFailure{}, Warnings: plan.Warnings}
	for _, row := range rows {
		payload, rowErrors, post := bulkcsv.BuildCreatePayload(row, plan)
		if post != nil {
			result.PlannedPostCreateUpdates = append(result.PlannedPostCreateUpdates, *post)
		}
		if len(rowErrors) > 0 {
			result.Failures = append(result.Failures, bulkcsv.CreateFailure{RowNumber: row.RowNumber, Code: rowErrors[0].Code, Message: rowErrors[0].Message})
			if failFast {
				break
			}
			continue
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: "issue", JSONBody: payload})
		if err != nil {
			result.Failures = append(result.Failures, bulkcsv.CreateFailure{RowNumber: row.RowNumber, Code: httpErrorCode(err), Message: err.Error()})
			if failFast {
				break
			}
			continue
		}
		defer resp.Body.Close()
		issue, _ := jira.ReadJSON(resp.Body)
		result.Created = append(result.Created, bulkcsv.CreatedIssue{RowNumber: row.RowNumber, Issue: issue})
	}
	if len(result.PlannedPostCreateUpdates) > 0 {
		result.Warnings = append(result.Warnings, bulkcsv.PlanWarning{Code: "post_create_update_not_applied", Message: "post-create update fields are planned but not implemented by bulk-create"})
	}
	return result, nil
}

func getJiraValue(ctx *jira.Context, path string, q map[string]string) (interface{}, error) {
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: path, Query: q})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return jira.ReadJSONValue(resp.Body)
}

func unwrapEnvelopeValue(v interface{}) interface{} {
	m := mapFromInterface(v)
	if _, ok := m["ok"]; ok {
		if data, ok := m["data"]; ok {
			return data
		}
	}
	return v
}

func unwrapEnvelopeMap(v interface{}) map[string]interface{} {
	v = unwrapEnvelopeValue(v)
	return mapFromInterface(v)
}

func mapFromInterface(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func printBulkCSVError(cmd *cobra.Command, o *Opts, err error) error {
	var bulkErr *bulkcsv.Error
	if errors.As(err, &bulkErr) {
		status := bulkErr.Status
		if status == 0 {
			status = 400
		}
		return print(cmd, o, output.Failure(bulkErr.Code, bulkErr.Message, bulkErr.Hint, status))
	}
	return print(cmd, o, envelopeError(err, "server_error"))
}

func httpErrorCode(err error) string {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) && httpErr.Code != "" {
		return httpErr.Code
	}
	return "server_error"
}

func metadataString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
