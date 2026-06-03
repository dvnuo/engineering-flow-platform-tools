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
