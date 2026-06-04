package logtool

type TimeRange struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

type Manifest struct {
	Version          int         `json:"version"`
	RunID            string      `json:"run_id"`
	CreatedAt        string      `json:"created_at"`
	FormatHint       string      `json:"format_hint"`
	Sources          []SourceRef `json:"sources"`
	TotalBytes       int64       `json:"total_bytes"`
	TotalLines       int64       `json:"total_lines"`
	EntriesCount     int         `json:"entries_count"`
	TemplatesCount   int         `json:"templates_count"`
	TimeRange        TimeRange   `json:"time_range"`
	RedactionEnabled bool        `json:"redaction_enabled"`
	ToolVersion      string      `json:"tool_version"`
	Truncated        bool        `json:"truncated"`
}

type SourceRef struct {
	SourceID  string `json:"source_id"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	Lines     int64  `json:"lines"`
	Truncated bool   `json:"truncated,omitempty"`
}

type Entry struct {
	EntryID        string   `json:"entry_id"`
	SourceID       string   `json:"source_id"`
	SourcePath     string   `json:"source_path"`
	ByteStart      int64    `json:"byte_start"`
	ByteEnd        int64    `json:"byte_end"`
	LineStart      int64    `json:"line_start"`
	LineEnd        int64    `json:"line_end"`
	Timestamp      string   `json:"timestamp,omitempty"`
	Level          string   `json:"level,omitempty"`
	Service        string   `json:"service,omitempty"`
	MessagePreview string   `json:"message_preview"`
	TemplateID     string   `json:"template_id"`
	Variables      []string `json:"variables,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	GoldenSignal   string   `json:"golden_signal,omitempty"`
}

type Template struct {
	TemplateID            string         `json:"template_id"`
	Template              string         `json:"template"`
	RepresentativeEntryID string         `json:"representative_entry_id"`
	RepresentativeText    string         `json:"representative_text"`
	Count                 int            `json:"count"`
	FirstSeen             string         `json:"first_seen,omitempty"`
	LastSeen              string         `json:"last_seen,omitempty"`
	Levels                map[string]int `json:"levels"`
	Examples              []string       `json:"examples"`
	GoldenSignal          string         `json:"golden_signal,omitempty"`
	Tags                  []string       `json:"tags,omitempty"`
}

type AnalyzeOptions struct {
	Source       string
	RunDir       string
	FormatHint   string
	MaxBytes     int64
	MaxLineBytes int64
	ToolVersion  string
	DryRun       bool
}

type AnalyzeResult struct {
	RunID          string    `json:"run_id"`
	RunDir         string    `json:"run_dir"`
	Sources        int       `json:"sources"`
	TotalBytes     int64     `json:"total_bytes"`
	TotalLines     int64     `json:"total_lines"`
	EntriesCount   int       `json:"entries_count"`
	TemplatesCount int       `json:"templates_count"`
	TimeRange      TimeRange `json:"time_range"`
	Truncated      bool      `json:"truncated"`
}

type AnalyzeDryRunSource struct {
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	Bytes    int64  `json:"bytes"`
	Readable bool   `json:"readable"`
}

type AnalyzeDryRunWork struct {
	SourceCount        int   `json:"source_count"`
	TotalBytes         int64 `json:"total_bytes"`
	WillWriteWorkspace bool  `json:"will_write_workspace"`
}

type AnalyzeDryRunResult struct {
	DryRun        bool                  `json:"dry_run"`
	RunDir        string                `json:"run_dir"`
	Sources       []AnalyzeDryRunSource `json:"sources"`
	EstimatedWork AnalyzeDryRunWork     `json:"estimated_work"`
}

type ParseOptions struct {
	FormatHint   string
	MaxBytes     int64
	MaxLineBytes int64
}

type ParsedEvent struct {
	Raw       string
	Timestamp string
	Level     string
	Service   string
	Message   string
	LineStart int64
	LineEnd   int64
	ByteStart int64
	ByteEnd   int64
}

type ParseResult struct {
	Bytes     int64
	Lines     int64
	Truncated bool
}

type SearchOptions struct {
	Query      string
	Regex      bool
	Level      string
	Service    string
	TemplateID string
	Since      string
	Until      string
	Limit      int
	Cursor     string
}

type EntryListOptions struct {
	TemplateID string
	Level      string
	Limit      int
	Cursor     string
}

type SourceLocation struct {
	Path      string `json:"path"`
	LineStart int64  `json:"line_start"`
	LineEnd   int64  `json:"line_end"`
	ByteStart int64  `json:"byte_start"`
	ByteEnd   int64  `json:"byte_end"`
}

type SearchItem struct {
	EntryID        string         `json:"entry_id"`
	TemplateID     string         `json:"template_id"`
	Timestamp      string         `json:"timestamp,omitempty"`
	Level          string         `json:"level,omitempty"`
	MessagePreview string         `json:"message_preview"`
	Source         SourceLocation `json:"source"`
	EvidenceRef    string         `json:"evidence_ref"`
}

type SearchResult struct {
	Query      string       `json:"query"`
	Matches    int          `json:"matches"`
	Items      []SearchItem `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

type EntryListResult struct {
	Items      []Entry `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type ProfileResult struct {
	RunID          string         `json:"run_id"`
	Sources        []SourceRef    `json:"sources"`
	TotalLines     int64          `json:"total_lines"`
	EntriesCount   int            `json:"entries_count"`
	TemplatesCount int            `json:"templates_count"`
	Levels         map[string]int `json:"levels"`
	TopTemplates   []Template     `json:"top_templates"`
	TimeRange      TimeRange      `json:"time_range"`
	Truncated      bool           `json:"truncated"`
}

type TemplatesResult struct {
	Templates []Template `json:"templates"`
}

type TemplateVariablesResult struct {
	TemplateID string             `json:"template_id"`
	Template   string             `json:"template"`
	Variables  []TemplateVariable `json:"variables"`
}

type TemplateVariable struct {
	Position  int              `json:"position"`
	TypeGuess string           `json:"type_guess,omitempty"`
	TopValues []VariableSample `json:"top_values"`
}

type VariableSample struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type WindowLine struct {
	Line   int64  `json:"line"`
	Text   string `json:"text"`
	Target bool   `json:"target"`
}

type WindowSource struct {
	Path      string `json:"path"`
	LineStart int64  `json:"line_start"`
	LineEnd   int64  `json:"line_end"`
}

type WindowResult struct {
	EntryID string       `json:"entry_id,omitempty"`
	Source  WindowSource `json:"source"`
	Before  int          `json:"before"`
	After   int          `json:"after"`
	Lines   []WindowLine `json:"lines"`
}

type ExtractItem struct {
	TemplateID            string         `json:"template_id"`
	Template              string         `json:"template,omitempty"`
	RepresentativeEntryID string         `json:"representative_entry_id"`
	RepresentativeText    string         `json:"representative_text"`
	Count                 int            `json:"count"`
	Levels                map[string]int `json:"levels,omitempty"`
	EvidenceRefs          []string       `json:"evidence_refs"`
}

type ExtractResult struct {
	Kind  string        `json:"kind"`
	Items []ExtractItem `json:"items"`
}

type RunListItem struct {
	RunID          string    `json:"run_id"`
	RunDir         string    `json:"run_dir"`
	CreatedAt      string    `json:"created_at"`
	Sources        int       `json:"sources"`
	EntriesCount   int       `json:"entries_count"`
	TemplatesCount int       `json:"templates_count"`
	TimeRange      TimeRange `json:"time_range"`
	Truncated      bool      `json:"truncated"`
}

type RunListResult struct {
	Workspace string        `json:"workspace"`
	Runs      []RunListItem `json:"runs"`
}

type RunGetResult struct {
	RunID     string   `json:"run_id"`
	RunDir    string   `json:"run_dir"`
	Status    string   `json:"status"`
	Manifest  Manifest `json:"manifest"`
	Index     RunIndex `json:"index"`
	Workspace string   `json:"workspace,omitempty"`
}

type RunIndex struct {
	Entries          int  `json:"entries"`
	Templates        int  `json:"templates"`
	RedactionEnabled bool `json:"redaction_enabled"`
}

type RunDeleteResult struct {
	RunID   string `json:"run_id"`
	RunDir  string `json:"run_dir"`
	Deleted bool   `json:"deleted"`
	DryRun  bool   `json:"dry_run"`
}

type RunVerifyResult struct {
	RunID  string          `json:"run_id"`
	RunDir string          `json:"run_dir"`
	OK     bool            `json:"ok"`
	Files  map[string]bool `json:"files"`
	Counts map[string]int  `json:"counts"`
	Hints  []string        `json:"hints,omitempty"`
}

type DoctorResult struct {
	Tool             string        `json:"tool"`
	DefaultWorkspace string        `json:"default_workspace"`
	LocalOnly        bool          `json:"local_only"`
	Checks           []DoctorCheck `json:"checks"`
}

type DoctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type GroupOptions struct {
	By         string
	Level      string
	Query      string
	TemplateID string
	Bucket     string
	Limit      int
}

type GroupResult struct {
	RunID   string      `json:"run_id"`
	GroupBy string      `json:"group_by"`
	Groups  []GroupItem `json:"groups"`
}

type GroupItem struct {
	Key                   string         `json:"key"`
	TemplateID            string         `json:"template_id,omitempty"`
	Count                 int            `json:"count"`
	FirstSeen             string         `json:"first_seen,omitempty"`
	LastSeen              string         `json:"last_seen,omitempty"`
	Levels                map[string]int `json:"levels,omitempty"`
	Services              map[string]int `json:"services,omitempty"`
	RepresentativeEntryID string         `json:"representative_entry_id,omitempty"`
	EvidenceRef           string         `json:"evidence_ref,omitempty"`
}

type TimelineOptions struct {
	Bucket     string
	Level      string
	TemplateID string
	Limit      int
}

type TimelineResult struct {
	RunID  string           `json:"run_id"`
	Bucket string           `json:"bucket"`
	Series []TimelineBucket `json:"series"`
}

type TimelineBucket struct {
	Start  string         `json:"start"`
	End    string         `json:"end"`
	Count  int            `json:"count"`
	Levels map[string]int `json:"levels,omitempty"`
	Spike  bool           `json:"spike,omitempty"`
}

type SummaryOptions struct {
	Focus string
	Since string
	Until string
}

type SummaryResult struct {
	RunID                   string           `json:"run_id"`
	Focus                   string           `json:"focus,omitempty"`
	Headline                string           `json:"headline"`
	TimeRange               TimeRange        `json:"time_range"`
	Findings                []SummaryFinding `json:"findings"`
	RecommendedNextCommands []string         `json:"recommended_next_commands"`
}

type SummaryFinding struct {
	Finding      string   `json:"finding"`
	Confidence   string   `json:"confidence"`
	Count        int      `json:"count"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type ExportEvidenceOptions struct {
	Evidence  string
	Format    string
	Output    string
	Overwrite bool
	DryRun    bool
}

type ExportEvidenceResult struct {
	RunID     string `json:"run_id"`
	Evidence  string `json:"evidence"`
	Format    string `json:"format"`
	Output    string `json:"output"`
	DryRun    bool   `json:"dry_run"`
	Written   bool   `json:"written"`
	Bytes     int    `json:"bytes"`
	Redacted  bool   `json:"redacted"`
	Truncated bool   `json:"truncated"`
}

type ToolError struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *ToolError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewError(code, message, hint string, status int) *ToolError {
	return &ToolError{Code: code, Message: message, Hint: hint, Status: status}
}
