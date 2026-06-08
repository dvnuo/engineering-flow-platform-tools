package automation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type NetworkExportOptions struct {
	PageOptions
	OutPath string
	Format  string
	Filter  string
	Limit   int
}

type NetworkExportResult struct {
	Session        string    `json:"session"`
	TargetID       string    `json:"target_id"`
	Path           string    `json:"path"`
	Format         string    `json:"format"`
	Count          int       `json:"count"`
	Bytes          int64     `json:"bytes"`
	SourceArtifact string    `json:"source_artifact_path,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
	Limitation     string    `json:"limitation"`
}

type NetworkExportJSONArtifact struct {
	Session    string               `json:"session"`
	TargetID   string               `json:"target_id"`
	Format     string               `json:"format"`
	Filter     string               `json:"filter,omitempty"`
	Limit      int                  `json:"limit"`
	Count      int                  `json:"count"`
	Entries    []NetworkRecordEntry `json:"entries"`
	UpdatedAt  time.Time            `json:"updated_at"`
	Limitation string               `json:"limitation"`
}

type NetworkHARLiteArtifact struct {
	Log        NetworkHARLiteLog `json:"log"`
	Count      int               `json:"count"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Limitation string            `json:"limitation"`
}

type NetworkHARLiteLog struct {
	Version string                `json:"version"`
	Creator NetworkHARLiteCreator `json:"creator"`
	Entries []NetworkHARLiteEntry `json:"entries"`
}

type NetworkHARLiteCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type NetworkHARLiteEntry struct {
	StartedAt     string                 `json:"started_at,omitempty"`
	Time          float64                `json:"time_ms,omitempty"`
	Request       NetworkHARLiteRequest  `json:"request"`
	Response      NetworkHARLiteResponse `json:"response"`
	ResourceType  string                 `json:"resource_type,omitempty"`
	InitiatorType string                 `json:"initiator_type,omitempty"`
	Sizes         NetworkHARLiteSizes    `json:"sizes,omitempty"`
	Source        string                 `json:"source,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

type NetworkHARLiteRequest struct {
	Method string `json:"method,omitempty"`
	URL    string `json:"url"`
}

type NetworkHARLiteResponse struct {
	Status int `json:"status,omitempty"`
}

type NetworkHARLiteSizes struct {
	TransferBytes int64 `json:"transfer_bytes,omitempty"`
	EncodedBytes  int64 `json:"encoded_bytes,omitempty"`
	DecodedBytes  int64 `json:"decoded_bytes,omitempty"`
}

func (m *Manager) NetworkExport(ctx context.Context, opts NetworkExportOptions) (NetworkExportResult, error) {
	opts, err := normalizeNetworkExportOptions(opts)
	if err != nil {
		return NetworkExportResult{}, err
	}
	session, target, err := m.ResolveTarget(ctx, opts.SessionName, opts.TargetID)
	if err != nil {
		return NetworkExportResult{}, err
	}
	if err := m.ensureStore(); err != nil {
		return NetworkExportResult{}, err
	}
	sourcePath, err := m.Store.NetworkArtifactPath(session.Name, target.ID)
	if err != nil {
		return NetworkExportResult{}, err
	}
	b, err := os.ReadFile(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return NetworkExportResult{}, NewError("network_artifact_not_found", "Recorded network metadata was not found for this page target.", "Run browser network start --json, interact with the page, then run browser network list or export.", 404)
	}
	if err != nil {
		return NetworkExportResult{}, NewError("automation_failed", err.Error(), "Network recorder artifact could not be read.", 500)
	}
	var artifact NetworkRecorderArtifact
	if err := json.Unmarshal(b, &artifact); err != nil {
		return NetworkExportResult{}, NewError("automation_failed", err.Error(), "Network recorder artifact is not valid JSON. Run browser network list --json again.", 500)
	}
	entries, count := networkExportEntries(artifact.Entries, opts)
	now := m.now()
	outPath := filepath.Clean(expandHome(opts.OutPath))
	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return NetworkExportResult{}, NewError("artifact_write_failed", err.Error(), "Check --out permissions and available disk space.", 500)
	}
	outBytes, err := marshalNetworkExportArtifact(session.Name, target.ID, opts, entries, count, now)
	if err != nil {
		return NetworkExportResult{}, err
	}
	if err := os.WriteFile(outPath, outBytes, 0o600); err != nil {
		return NetworkExportResult{}, NewError("artifact_write_failed", err.Error(), "Network export could not be written.", 500)
	}
	stat, err := os.Stat(outPath)
	if err != nil {
		return NetworkExportResult{}, NewError("artifact_write_failed", err.Error(), "Network export was written but metadata could not be read.", 500)
	}
	return NetworkExportResult{
		Session:        session.Name,
		TargetID:       target.ID,
		Path:           outPath,
		Format:         opts.Format,
		Count:          count,
		Bytes:          stat.Size(),
		SourceArtifact: sourcePath,
		UpdatedAt:      now,
		Limitation:     networkRecorderLimitation,
	}, nil
}

func normalizeNetworkExportOptions(opts NetworkExportOptions) (NetworkExportOptions, error) {
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if opts.Format == "" {
		opts.Format = "har-lite"
	}
	switch opts.Format {
	case "json", "har-lite":
	default:
		return opts, invalidArgs("--format must be json or har-lite", "Pass --format json or --format har-lite.")
	}
	if strings.TrimSpace(opts.OutPath) == "" {
		return opts, invalidArgs("--out is required", "Pass a file path for the sanitized network export.")
	}
	if opts.Limit <= 0 {
		opts.Limit = 500
	}
	if opts.Limit > 5000 {
		opts.Limit = 5000
	}
	return opts, nil
}

func networkExportEntries(raw []NetworkRecordEntry, opts NetworkExportOptions) ([]NetworkRecordEntry, int) {
	opts, _ = normalizeNetworkExportOptions(opts)
	recorderOpts := NetworkRecorderOptions{Filter: opts.Filter, Limit: opts.Limit, Status: -1}
	out := make([]NetworkRecordEntry, 0, minInt(opts.Limit, len(raw)))
	count := 0
	for _, entry := range raw {
		entry.URL = RedactURL(entry.URL)
		entry.Method = normalizeNetworkMethod(entry.Method)
		entry.Status = normalizeStatus(entry.Status)
		entry.ResourceType = strings.ToLower(TruncateBytes(RedactString(entry.ResourceType), 80))
		entry.InitiatorType = strings.ToLower(TruncateBytes(RedactString(entry.InitiatorType), 80))
		entry.Source = strings.ToLower(TruncateBytes(RedactString(entry.Source), 80))
		entry.Error = TruncateBytes(RedactError(entry.Error), 500)
		if !networkRecordMatches(entry, recorderOpts) {
			continue
		}
		count++
		if len(out) >= opts.Limit {
			continue
		}
		entry.Index = count - 1
		out = append(out, entry)
	}
	return out, count
}

func marshalNetworkExportArtifact(sessionName, targetID string, opts NetworkExportOptions, entries []NetworkRecordEntry, count int, now time.Time) ([]byte, error) {
	var value any
	switch opts.Format {
	case "json":
		value = NetworkExportJSONArtifact{
			Session:    sessionName,
			TargetID:   targetID,
			Format:     opts.Format,
			Filter:     RedactString(opts.Filter),
			Limit:      opts.Limit,
			Count:      count,
			Entries:    entries,
			UpdatedAt:  now,
			Limitation: networkRecorderLimitation,
		}
	case "har-lite":
		value = networkHARLiteArtifact(entries, count, now)
	default:
		return nil, invalidArgs("--format must be json or har-lite", "Pass --format json or --format har-lite.")
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, NewError("automation_failed", err.Error(), "Network export could not be encoded.", 500)
	}
	return append(b, '\n'), nil
}

func networkHARLiteArtifact(entries []NetworkRecordEntry, count int, now time.Time) NetworkHARLiteArtifact {
	out := make([]NetworkHARLiteEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, NetworkHARLiteEntry{
			StartedAt: entry.StartedAt,
			Time:      entry.DurationMilliseconds,
			Request: NetworkHARLiteRequest{
				Method: entry.Method,
				URL:    entry.URL,
			},
			Response:      NetworkHARLiteResponse{Status: entry.Status},
			ResourceType:  entry.ResourceType,
			InitiatorType: entry.InitiatorType,
			Sizes: NetworkHARLiteSizes{
				TransferBytes: entry.TransferSizeBytes,
				EncodedBytes:  entry.EncodedSizeBytes,
				DecodedBytes:  entry.DecodedSizeBytes,
			},
			Source: entry.Source,
			Error:  entry.Error,
		})
	}
	return NetworkHARLiteArtifact{
		Log: NetworkHARLiteLog{
			Version: "1.0",
			Creator: NetworkHARLiteCreator{
				Name:    "browser",
				Version: "har-lite",
			},
			Entries: out,
		},
		Count:      count,
		UpdatedAt:  now,
		Limitation: networkRecorderLimitation,
	}
}
