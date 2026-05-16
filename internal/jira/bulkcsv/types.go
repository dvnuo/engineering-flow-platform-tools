package bulkcsv

import "fmt"

const (
	PlanVersion = 1
	PlanMode    = "jira_csv_bulk_create"

	PhaseCreate           = "create"
	PhasePostCreateUpdate = "post_create_update"
)

type Error struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Message
}

func InvalidArgs(format string, args ...interface{}) *Error {
	return &Error{Code: "invalid_args", Message: fmt.Sprintf(format, args...), Status: 400}
}

type CSVSummary struct {
	Path       string              `json:"path"`
	Columns    []string            `json:"columns"`
	RowCount   int                 `json:"row_count"`
	SampleRows []map[string]string `json:"sample_rows,omitempty"`
}

type CSVRow struct {
	RowNumber int               `json:"row_number"`
	Values    map[string]string `json:"values"`
}

type CSVData struct {
	Summary CSVSummary `json:"summary"`
	Rows    []CSVRow   `json:"rows"`
}

type AllowedValue struct {
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Value string                 `json:"value,omitempty"`
	Raw   map[string]interface{} `json:"raw,omitempty"`
}

type FieldCandidate struct {
	JiraFieldID   string         `json:"jira_field_id"`
	JiraFieldName string         `json:"jira_field_name"`
	SchemaType    string         `json:"schema_type,omitempty"`
	SchemaCustom  string         `json:"schema_custom,omitempty"`
	Required      bool           `json:"required"`
	AllowedValues []AllowedValue `json:"allowed_values,omitempty"`
	Confidence    float64        `json:"confidence"`
	Signals       []string       `json:"signals,omitempty"`
	Reason        string         `json:"reason,omitempty"`
	Phase         string         `json:"phase,omitempty"`
}

type FieldMapping struct {
	CSVColumn     string         `json:"csv_column"`
	JiraFieldID   string         `json:"jira_field_id"`
	JiraFieldName string         `json:"jira_field_name"`
	SchemaType    string         `json:"schema_type,omitempty"`
	SchemaCustom  string         `json:"schema_custom,omitempty"`
	Required      bool           `json:"required"`
	Phase         string         `json:"phase"`
	Transform     string         `json:"transform"`
	Confidence    float64        `json:"confidence"`
	Reason        string         `json:"reason,omitempty"`
	AllowedValues []AllowedValue `json:"allowed_values,omitempty"`
}

type TemplateDefault struct {
	JiraFieldID   string      `json:"jira_field_id"`
	JiraFieldName string      `json:"jira_field_name"`
	Required      bool        `json:"required,omitempty"`
	Value         interface{} `json:"value"`
}

type BlockedField struct {
	JiraFieldID   string `json:"jira_field_id"`
	JiraFieldName string `json:"jira_field_name,omitempty"`
	Reason        string `json:"reason"`
}

type AmbiguousColumn struct {
	CSVColumn  string           `json:"csv_column"`
	Candidates []FieldCandidate `json:"candidates"`
}

type UnmappedColumn struct {
	CSVColumn string `json:"csv_column"`
	Reason    string `json:"reason"`
}

type ConfirmationItem struct {
	CSVColumn   string `json:"csv_column,omitempty"`
	JiraFieldID string `json:"jira_field_id,omitempty"`
	Reason      string `json:"reason"`
}

type PlanWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type JiraInfo struct {
	Instance      string `json:"instance,omitempty"`
	ProjectKey    string `json:"project,omitempty"`
	ProjectID     string `json:"project_id,omitempty"`
	IssueTypeName string `json:"issuetype,omitempty"`
	IssueTypeID   string `json:"issuetype_id,omitempty"`
	TemplateIssue string `json:"template_issue,omitempty"`
}

type FieldRef struct {
	JiraFieldID   string `json:"jira_field_id"`
	JiraFieldName string `json:"jira_field_name,omitempty"`
}

type MappingPlan struct {
	Version              int                 `json:"version"`
	Mode                 string              `json:"mode"`
	Jira                 JiraInfo            `json:"jira"`
	CSV                  CSVSummary          `json:"csv"`
	FieldMappings        []FieldMapping      `json:"field_mappings"`
	TemplateDefaults     []TemplateDefault   `json:"template_defaults"`
	BlockedFields        []BlockedField      `json:"blocked_fields"`
	AmbiguousColumns     []AmbiguousColumn   `json:"ambiguous_columns"`
	UnmappedColumns      []UnmappedColumn    `json:"unmapped_columns"`
	RequiresConfirmation []ConfirmationItem  `json:"requires_confirmation"`
	Warnings             []PlanWarning       `json:"warnings"`
	RequiredFields       []FieldRef          `json:"required_fields,omitempty"`
	CreateFields         map[string]FieldRef `json:"create_fields,omitempty"`
}

type MappingInput struct {
	CSV                     CSVSummary
	Rows                    []CSVRow
	FieldCatalog            interface{}
	ExampleIssue            map[string]interface{}
	CreateMeta              map[string]interface{}
	EditMeta                map[string]interface{}
	Jira                    JiraInfo
	MinConfidence           float64
	IncludeTemplateDefaults bool
}

type RowError struct {
	RowNumber   int    `json:"row_number"`
	CSVColumn   string `json:"csv_column,omitempty"`
	JiraFieldID string `json:"jira_field_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type PayloadPreview struct {
	RowNumber int                    `json:"row_number"`
	Payload   map[string]interface{} `json:"payload"`
}

type PostCreateUpdate struct {
	RowNumber int                    `json:"row_number"`
	Fields    map[string]interface{} `json:"fields"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

type DryRunResult struct {
	DryRun                   bool               `json:"dry_run"`
	Rows                     int                `json:"rows"`
	ValidRows                int                `json:"valid_rows"`
	InvalidRows              int                `json:"invalid_rows"`
	PreviewPayloads          []PayloadPreview   `json:"preview_payloads"`
	Errors                   []RowError         `json:"errors"`
	PlannedPostCreateUpdates []PostCreateUpdate `json:"planned_post_create_updates,omitempty"`
	Warnings                 []PlanWarning      `json:"warnings,omitempty"`
}

type CreatedIssue struct {
	RowNumber              int                    `json:"row_number"`
	Created                bool                   `json:"created"`
	Issue                  map[string]interface{} `json:"issue"`
	PostCreateUpdateStatus string                 `json:"post_create_update_status,omitempty"`
	Error                  *CreateFailure         `json:"error,omitempty"`
}

type CreateFailure struct {
	RowNumber int    `json:"row_number"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type CreateResult struct {
	Rows                     int                `json:"rows"`
	Created                  []CreatedIssue     `json:"created"`
	Failures                 []CreateFailure    `json:"failures"`
	PlannedPostCreateUpdates []PostCreateUpdate `json:"planned_post_create_updates,omitempty"`
	Warnings                 []PlanWarning      `json:"warnings,omitempty"`
}
