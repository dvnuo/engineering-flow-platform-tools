package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type PageDiffOptions struct {
	BeforeFile string
	AfterFile  string
	OutPath    string
	Limit      int
}

type PageDiffChange struct {
	Path          string `json:"path"`
	BeforePreview string `json:"before_preview,omitempty"`
	AfterPreview  string `json:"after_preview,omitempty"`
	BeforeBytes   int    `json:"before_bytes,omitempty"`
	AfterBytes    int    `json:"after_bytes,omitempty"`
}

type PageDiffResult struct {
	BeforeFile  string           `json:"before_file"`
	AfterFile   string           `json:"after_file"`
	Path        string           `json:"path,omitempty"`
	Bytes       int64            `json:"bytes,omitempty"`
	Limit       int              `json:"limit"`
	ChangeCount int              `json:"change_count"`
	Changes     []PageDiffChange `json:"changes"`
	GeneratedAt time.Time        `json:"generated_at"`
}

func PageDiff(opts PageDiffOptions) (PageDiffResult, error) {
	opts = normalizePageDiffOptions(opts)
	if strings.TrimSpace(opts.BeforeFile) == "" || strings.TrimSpace(opts.AfterFile) == "" {
		return PageDiffResult{}, invalidArgs("--before and --after are required", "Pass two JSON files to compare, such as before.json and after.json.")
	}
	before, err := readDiffJSON(opts.BeforeFile)
	if err != nil {
		return PageDiffResult{}, err
	}
	after, err := readDiffJSON(opts.AfterFile)
	if err != nil {
		return PageDiffResult{}, err
	}
	changes := diffValues("$", before, after, opts.Limit)
	result := PageDiffResult{
		BeforeFile:  filepath.Clean(expandHome(opts.BeforeFile)),
		AfterFile:   filepath.Clean(expandHome(opts.AfterFile)),
		Limit:       opts.Limit,
		ChangeCount: len(changes),
		Changes:     changes,
		GeneratedAt: time.Now().UTC(),
	}
	if strings.TrimSpace(opts.OutPath) != "" {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return PageDiffResult{}, NewError("automation_failed", err.Error(), "Page diff could not be encoded.", 500)
		}
		path, size, err := writeExportArtifact(opts.OutPath, b)
		if err != nil {
			return PageDiffResult{}, err
		}
		result.Path = path
		result.Bytes = size
	}
	return result, nil
}

func normalizePageDiffOptions(opts PageDiffOptions) PageDiffOptions {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}
	return opts
}

func readDiffJSON(path string) (any, error) {
	clean := filepath.Clean(expandHome(path))
	b, err := os.ReadFile(clean)
	if err != nil {
		return nil, NewError("diff_read_failed", err.Error(), "Check that --before and --after point to readable JSON files.", 400)
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return nil, NewError("diff_invalid_json", err.Error(), "Page diff expects JSON files, including browser --json envelopes.", 400)
	}
	if m, ok := value.(map[string]any); ok {
		if data, ok := m["data"]; ok {
			return data, nil
		}
	}
	return value, nil
}

func diffValues(path string, before, after any, limit int) []PageDiffChange {
	var out []PageDiffChange
	var walk func(string, any, any)
	walk = func(path string, before, after any) {
		if len(out) >= limit {
			return
		}
		switch b := before.(type) {
		case map[string]any:
			a, ok := after.(map[string]any)
			if !ok {
				out = append(out, diffChange(path, before, after))
				return
			}
			keys := map[string]bool{}
			for key := range b {
				keys[key] = true
			}
			for key := range a {
				keys[key] = true
			}
			ordered := make([]string, 0, len(keys))
			for key := range keys {
				ordered = append(ordered, key)
			}
			sort.Strings(ordered)
			for _, key := range ordered {
				walk(path+"."+key, b[key], a[key])
			}
		case []any:
			a, ok := after.([]any)
			if !ok {
				out = append(out, diffChange(path, before, after))
				return
			}
			max := len(b)
			if len(a) > max {
				max = len(a)
			}
			for i := 0; i < max; i++ {
				var bv, av any
				if i < len(b) {
					bv = b[i]
				}
				if i < len(a) {
					av = a[i]
				}
				walk(path+"["+strconv.Itoa(i)+"]", bv, av)
			}
		default:
			if !jsonScalarEqual(before, after) {
				out = append(out, diffChange(path, before, after))
			}
		}
	}
	walk(path, before, after)
	return out
}

func jsonScalarEqual(before, after any) bool {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	return string(b) == string(a)
}

func diffChange(path string, before, after any) PageDiffChange {
	beforeText := diffPreview(before)
	afterText := diffPreview(after)
	return PageDiffChange{
		Path:          path,
		BeforePreview: TruncateBytes(RedactString(beforeText), 500),
		AfterPreview:  TruncateBytes(RedactString(afterText), 500),
		BeforeBytes:   len(beforeText),
		AfterBytes:    len(afterText),
	}
}

func diffPreview(value any) string {
	if value == nil {
		return "null"
	}
	if s, ok := value.(string); ok {
		return s
	}
	b, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(b)
}
