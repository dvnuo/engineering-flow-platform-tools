package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/splunk"
	"github.com/spf13/cobra"
)

const (
	defaultSearchCount   = 100
	defaultTimeoutSec    = 60
	defaultPollMS        = 1000
	defaultMaxFieldChars = 2000
	defaultSavedCount    = 100
	defaultIndexCount    = 200
	truncatedMarker      = "...(truncated)"
)

// searchOpts carries the flags shared by search run, search oneshot, saved
// run, and search job results.
type searchOpts struct {
	query, earliest, latest, output   string
	count, offset, timeoutSec, pollMS int
	maxFieldChars                     int
	fields                            []string
}

func addResultFlags(c *cobra.Command) {
	c.Flags().Int("count", defaultSearchCount, "Maximum results to return; capped by the instance max_results (default 1000).")
	c.Flags().Int("offset", 0, "Result offset for paging within the cap.")
	c.Flags().StringSlice("fields", nil, "Comma-separated field names to keep, for example _time,host,message.")
	c.Flags().Int("max-field-chars", defaultMaxFieldChars, "Truncate each printed field value to this many characters (0 disables); data.fields_truncated lists the affected fields.")
	c.Flags().String("output", "", "Write the untruncated results JSON to this file and print only path, bytes, and counts.")
}

func addTimeFlags(c *cobra.Command, forSaved bool) {
	if forSaved {
		c.Flags().String("earliest", "", "Override the saved search dispatch earliest_time, for example -1h or -24h@h.")
		c.Flags().String("latest", "", "Override the saved search dispatch latest_time, for example now.")
		return
	}
	c.Flags().String("earliest", "", "Search earliest_time modifier, for example -15m, -1h, -24h@h, or an ISO timestamp; defaults to the instance default_earliest or -1h.")
	c.Flags().String("latest", "", "Search latest_time modifier; defaults to now.")
}

func addPollFlags(c *cobra.Command, withPoll bool) {
	c.Flags().Int("timeout-sec", defaultTimeoutSec, "Seconds to wait for the search to finish before it is cancelled and wait_timeout is returned.")
	if withPoll {
		c.Flags().Int("poll-ms", defaultPollMS, "Milliseconds between search job status polls.")
	}
}

func readSearchOpts(cmd *cobra.Command) searchOpts {
	so := searchOpts{
		query:         mustS(cmd, "query"),
		earliest:      mustS(cmd, "earliest"),
		latest:        mustS(cmd, "latest"),
		output:        mustS(cmd, "output"),
		count:         mustI(cmd, "count"),
		offset:        mustI(cmd, "offset"),
		timeoutSec:    mustI(cmd, "timeout-sec"),
		pollMS:        mustI(cmd, "poll-ms"),
		maxFieldChars: mustI(cmd, "max-field-chars"),
	}
	for _, f := range mustSS(cmd, "fields") {
		if f = strings.TrimSpace(f); f != "" {
			so.fields = append(so.fields, f)
		}
	}
	if so.timeoutSec <= 0 {
		so.timeoutSec = defaultTimeoutSec
	}
	if so.pollMS <= 0 {
		so.pollMS = defaultPollMS
	}
	if strings.TrimSpace(so.latest) == "" {
		so.latest = splunk.DefaultLatest
	}
	return so
}

func validateResultOpts(so searchOpts, capacity int) *output.Envelope {
	var env output.Envelope
	switch {
	case so.count < 1:
		env = output.Failure("invalid_args", "--count must be at least 1", "", 400)
	case so.count > capacity:
		env = output.Failure("invalid_args", fmt.Sprintf("--count %d exceeds the instance max_results cap of %d", so.count, capacity), "Narrow the search (for example add | head N or aggregate with | stats), page with --offset within the cap, or raise max_results on the instance.", 400)
	case so.offset < 0:
		env = output.Failure("invalid_args", "--offset must not be negative", "", 400)
	case so.maxFieldChars < 0:
		env = output.Failure("invalid_args", "--max-field-chars must not be negative", "Pass 0 to disable truncation.", 400)
	default:
		return nil
	}
	return &env
}

func splBlocked(command string, extra map[string]any) output.Envelope {
	message := "the SPL command " + command + " writes data, triggers actions, or runs code"
	hint := "Searches must be read-only: remove " + command + " (blocked: " + strings.Join(splunk.BlockedCommands(), ", ") + ")."
	if command == splunk.MacroToken {
		message = "the search uses a macro, whose expansion cannot be checked for read-only SPL"
		hint = "Write the search without macros so every command is visible, or ask a Splunk admin what the macro expands to."
	}
	env := output.Failure("spl_blocked", message, hint, 400)
	data := map[string]any{"blocked_command": command}
	for k, v := range extra {
		data[k] = v
	}
	env.Data = data
	return env
}

func searchCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "search"}
	run := &cobra.Command{Use: "run", RunE: func(cmd *cobra.Command, args []string) error {
		return searchRun(o, cmd)
	}}
	run.Flags().String("query", "", "SPL search string; the search command is prepended when the query starts with neither search nor a pipe.")
	addTimeFlags(run, false)
	addResultFlags(run)
	addPollFlags(run, true)
	c.AddCommand(run)
	oneshot := &cobra.Command{Use: "oneshot", RunE: func(cmd *cobra.Command, args []string) error {
		return searchOneshot(o, cmd)
	}}
	oneshot.Flags().String("query", "", "SPL search string; the search command is prepended when the query starts with neither search nor a pipe.")
	addTimeFlags(oneshot, false)
	addResultFlags(oneshot)
	addPollFlags(oneshot, false)
	c.AddCommand(oneshot)
	c.AddCommand(jobCmd(o))
	return c
}

func searchRun(o *Opts, cmd *cobra.Command) error {
	so := readSearchOpts(cmd)
	if strings.TrimSpace(so.query) == "" {
		return print(cmd, o, output.Failure("invalid_args", "--query is required", "Pass the SPL, for example --query \"index=main error | head 100\".", 400))
	}
	if blocked, ok := splunk.BlockedCommand(so.query); ok {
		return print(cmd, o, splBlocked(blocked, nil))
	}
	cx, err := loadCtx(o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	capacity := cx.client.MaxResults()
	if env := validateResultOpts(so, capacity); env != nil {
		return print(cmd, o, *env)
	}
	search := splunk.PrepareSearch(so.query, cx.inst.DefaultIndex)
	earliest := cx.client.EffectiveEarliest(so.earliest)
	form := splunk.JobForm(search, earliest, so.latest, "normal", capacity)
	if o.DryRun {
		return print(cmd, o, output.Success(cx.inst.Name, dryRunData(http.MethodPost, "/services/search/jobs", form, map[string]any{"count": so.count, "offset": so.offset, "fields": so.fields, "timeout_sec": so.timeoutSec})))
	}
	sid, err := cx.client.CreateJob(form)
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	st, envErr := pollJob(cx, sid, so.timeoutSec, so.pollMS)
	if envErr != nil {
		return print(cmd, o, *envErr)
	}
	return collectJobResults(cmd, o, cx, st, so, map[string]any{"search": search, "earliest_time": earliest, "latest_time": so.latest})
}

func searchOneshot(o *Opts, cmd *cobra.Command) error {
	so := readSearchOpts(cmd)
	if strings.TrimSpace(so.query) == "" {
		return print(cmd, o, output.Failure("invalid_args", "--query is required", "Pass the SPL, for example --query \"index=main | stats count by host\".", 400))
	}
	if blocked, ok := splunk.BlockedCommand(so.query); ok {
		return print(cmd, o, splBlocked(blocked, nil))
	}
	cx, err := loadCtx(o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	capacity := cx.client.MaxResults()
	if env := validateResultOpts(so, capacity); env != nil {
		return print(cmd, o, *env)
	}
	search := splunk.PrepareSearch(so.query, cx.inst.DefaultIndex)
	earliest := cx.client.EffectiveEarliest(so.earliest)
	form := splunk.JobForm(search, earliest, so.latest, "oneshot", capacity)
	if o.DryRun {
		return print(cmd, o, output.Success(cx.inst.Name, dryRunData(http.MethodPost, "/services/search/jobs", form, map[string]any{"count": so.count, "offset": so.offset, "fields": so.fields, "timeout_sec": so.timeoutSec})))
	}
	res, err := cx.client.Oneshot(form, time.Duration(so.timeoutSec)*time.Second)
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	all := splunk.Results(res)
	total := len(all)
	page := pageResults(all, so.offset, so.count)
	var fields any = res["fields"]
	if len(so.fields) > 0 {
		page = keepFields(page, so.fields)
		fields = so.fields
	}
	data := map[string]any{
		"exec_mode":         "oneshot",
		"search":            search,
		"earliest_time":     earliest,
		"latest_time":       so.latest,
		"result_count":      total,
		"returned":          len(page),
		"offset":            so.offset,
		"cap":               capacity,
		"results_truncated": total > so.offset+len(page),
		"messages":          splunk.Messages(res["messages"]),
	}
	return finishResults(cmd, o, cx, data, page, fields, so)
}

func jobCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "job"}
	c.AddCommand(&cobra.Command{Use: "get <sid>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		st, err := cx.client.GetJob(args[0])
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		data := jobData(st)
		data["is_done"] = st.IsDone
		data["is_failed"] = st.IsFailed
		for out, in := range map[string]string{"search": "search", "earliest_time": "earliestTime", "latest_time": "latestTime", "run_duration": "runDuration", "ttl": "ttl", "is_finalized": "isFinalized", "is_preview_enabled": "isPreviewEnabled"} {
			if v, ok := st.Content[in]; ok {
				data[out] = v
			}
		}
		return print(cmd, o, output.Success(cx.inst.Name, data))
	}})
	results := &cobra.Command{Use: "results <sid>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		so := readSearchOpts(cmd)
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		if env := validateResultOpts(so, cx.client.MaxResults()); env != nil {
			return print(cmd, o, *env)
		}
		st, err := cx.client.GetJob(args[0])
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		if st.IsFailed {
			return print(cmd, o, searchFailed(st))
		}
		// Finished jobs expose the final result set; running jobs expose a
		// preview through results_preview, which unlike /events also works for
		// transforming searches created with status_buckets=0.
		endpoint := "results"
		if !st.IsDone {
			endpoint = "results_preview"
		}
		return collectJobResultsFrom(cmd, o, cx, st, so, endpoint, map[string]any{"preview": !st.IsDone})
	}}
	addResultFlags(results)
	c.AddCommand(results)
	c.AddCommand(&cobra.Command{Use: "cancel <sid>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming that the search job should be cancelled.", 400))
		}
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		path := "/services/search/jobs/" + url.PathEscape(args[0]) + "/control"
		form := url.Values{}
		form.Set("action", "cancel")
		if o.DryRun {
			return print(cmd, o, output.Success(cx.inst.Name, dryRunData(http.MethodPost, path, form, map[string]any{"sid": args[0]})))
		}
		if err := cx.client.ControlJob(args[0], "cancel"); err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"sid": args[0], "cancelled": true}))
	}})
	return c
}

func savedCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "saved"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		count := mustI(cmd, "count")
		if count < 1 {
			count = defaultSavedCount
		}
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		entries, err := cx.client.SavedSearches(count, mustI(cmd, "offset"), mustS(cmd, "filter"))
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		items := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			content := splunk.EntryContent(entry)
			search, _ := content["search"].(string)
			item := map[string]any{
				"name":          entry["name"],
				"search":        search,
				"disabled":      content["disabled"],
				"description":   content["description"],
				"is_scheduled":  content["is_scheduled"],
				"cron_schedule": content["cron_schedule"],
			}
			if blocked, ok := splunk.BlockedCommand(search); ok {
				item["blocked_command"] = blocked
			}
			items = append(items, item)
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"saved_searches": items, "count": len(items), "offset": mustI(cmd, "offset")}))
	}}
	list.Flags().Int("count", defaultSavedCount, "Maximum saved searches to list.")
	list.Flags().Int("offset", 0, "Listing offset.")
	list.Flags().String("filter", "", "Text filter applied to saved search names and definitions (Splunk `search` parameter).")
	c.AddCommand(list)
	run := &cobra.Command{Use: "run <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return savedRun(o, cmd, args[0])
	}}
	addTimeFlags(run, true)
	addResultFlags(run)
	addPollFlags(run, true)
	c.AddCommand(run)
	return c
}

func savedRun(o *Opts, cmd *cobra.Command, name string) error {
	so := readSearchOpts(cmd)
	cx, err := loadCtx(o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	if env := validateResultOpts(so, cx.client.MaxResults()); env != nil {
		return print(cmd, o, *env)
	}
	form := url.Values{}
	form.Set("trigger_actions", "0")
	if v := strings.TrimSpace(so.earliest); v != "" {
		form.Set("dispatch.earliest_time", v)
	}
	if v := strings.TrimSpace(mustS(cmd, "latest")); v != "" {
		form.Set("dispatch.latest_time", v)
	}
	path := "/services/saved/searches/" + url.PathEscape(name) + "/dispatch"
	if o.DryRun {
		return print(cmd, o, output.Success(cx.inst.Name, dryRunData(http.MethodPost, path, form, map[string]any{"saved_search": name, "count": so.count, "offset": so.offset, "timeout_sec": so.timeoutSec})))
	}
	// Guard the saved search definition before dispatching it. When the
	// definition cannot be read (for example a role that may dispatch but not
	// read the object), dispatch proceeds; Splunk's own permissions still apply.
	definition := ""
	if entry, err := cx.client.SavedSearch(name); err == nil {
		definition, _ = splunk.EntryContent(entry)["search"].(string)
		if blocked, ok := splunk.BlockedCommand(definition); ok {
			return print(cmd, o, splBlocked(blocked, map[string]any{"saved_search": name, "search": definition}))
		}
	}
	sid, err := cx.client.DispatchSaved(name, form)
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	st, envErr := pollJob(cx, sid, so.timeoutSec, so.pollMS)
	if envErr != nil {
		return print(cmd, o, *envErr)
	}
	extra := map[string]any{"saved_search": name}
	if definition != "" {
		extra["search"] = definition
	}
	return collectJobResults(cmd, o, cx, st, so, extra)
}

func indexCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "index"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		count := mustI(cmd, "count")
		if count < 1 {
			count = defaultIndexCount
		}
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		entries, err := cx.client.Indexes(count)
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		items := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			content := splunk.EntryContent(entry)
			items = append(items, map[string]any{
				"name":              entry["name"],
				"total_event_count": content["totalEventCount"],
				"max_time":          content["maxTime"],
				"min_time":          content["minTime"],
				"disabled":          content["disabled"],
			})
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"indexes": items, "count": len(items)}))
	}}
	list.Flags().Int("count", defaultIndexCount, "Maximum indexes to list.")
	c.AddCommand(list)
	return c
}

func apiCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	get := &cobra.Command{Use: "get <path>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := strings.TrimSpace(args[0])
		if !strings.HasPrefix(path, "/services/") && !strings.HasPrefix(path, "/servicesNS/") {
			return print(cmd, o, output.Failure("invalid_args", "path must start with /services/ or /servicesNS/", "Example: splunk api get /services/server/info --json.", 400))
		}
		if strings.ContainsAny(path, "?#") || strings.Contains(path, "..") {
			return print(cmd, o, output.Failure("invalid_args", "path must not contain a query string, fragment, or ..", "Pass parameters with --query key=value.", 400))
		}
		query := url.Values{}
		for _, item := range mustSA(cmd, "query") {
			key, value, ok := strings.Cut(item, "=")
			key = strings.TrimSpace(key)
			if !ok || key == "" {
				return print(cmd, o, output.Failure("invalid_args", "invalid --query "+item, "Pass --query key=value.", 400))
			}
			query.Add(key, value)
		}
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		if o.DryRun {
			return print(cmd, o, output.Success(cx.inst.Name, dryRunData(http.MethodGet, path, nil, map[string]any{"query": query})))
		}
		value, err := cx.client.DoValue(splunk.Request{Method: http.MethodGet, Path: path, Query: query})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		return print(cmd, o, output.Success(cx.inst.Name, value))
	}}
	get.Flags().StringArray("query", nil, "Query parameter in key=value form; repeat for multiple values. output_mode=json is always added.")
	c.AddCommand(get)
	return c
}

// pollJob waits for a search job to finish. It returns an error envelope when
// the job fails (search_failed) or does not finish before the deadline, in
// which case the job is cancelled and wait_timeout carries its last state.
func pollJob(cx *ctx, sid string, timeoutSec, pollMS int) (splunk.JobStatus, *output.Envelope) {
	if timeoutSec <= 0 {
		timeoutSec = defaultTimeoutSec
	}
	if pollMS <= 0 {
		pollMS = defaultPollMS
	}
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for {
		st, err := cx.client.GetJob(sid)
		if err != nil {
			env := envelopeError(err, "server_error")
			return st, &env
		}
		if st.IsFailed {
			env := searchFailed(st)
			return st, &env
		}
		if st.IsDone {
			return st, nil
		}
		if !time.Now().Before(deadline) {
			cancelErr := cx.client.ControlJob(sid, "cancel")
			env := output.Failure("wait_timeout", fmt.Sprintf("search job %s did not finish within %d seconds and was cancelled", sid, timeoutSec), "Narrow the time range or query, or increase --timeout-sec.", 408)
			data := jobData(st)
			data["cancelled"] = cancelErr == nil
			env.Data = data
			return st, &env
		}
		time.Sleep(time.Duration(pollMS) * time.Millisecond)
	}
}

func searchFailed(st splunk.JobStatus) output.Envelope {
	message := "search job " + st.SID + " failed"
	var texts []string
	for _, m := range st.Messages {
		if t, _ := m["text"].(string); t != "" {
			texts = append(texts, t)
		}
	}
	if len(texts) > 0 {
		message += ": " + strings.Join(texts, "; ")
	}
	env := output.Failure("search_failed", message, "Fix the SPL or time range and retry; data.messages carries the Splunk messages.", 500)
	env.Data = jobData(st)
	return env
}

func jobData(st splunk.JobStatus) map[string]any {
	return map[string]any{
		"sid":            st.SID,
		"dispatch_state": st.DispatchState,
		"done_progress":  st.DoneProgress,
		"event_count":    st.EventCount,
		"result_count":   st.ResultCount,
		"scan_count":     st.ScanCount,
		"messages":       st.Messages,
	}
}

func collectJobResults(cmd *cobra.Command, o *Opts, cx *ctx, st splunk.JobStatus, so searchOpts, extra map[string]any) error {
	return collectJobResultsFrom(cmd, o, cx, st, so, "results", extra)
}

func collectJobResultsFrom(cmd *cobra.Command, o *Opts, cx *ctx, st splunk.JobStatus, so searchOpts, endpoint string, extra map[string]any) error {
	res, err := cx.client.JobResults(st.SID, endpoint, so.count, so.offset, so.fields)
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	results := splunk.Results(res)
	data := jobData(st)
	for k, v := range extra {
		data[k] = v
	}
	data["returned"] = len(results)
	data["offset"] = so.offset
	data["cap"] = cx.client.MaxResults()
	data["results_truncated"] = st.ResultCount > so.offset+len(results)
	return finishResults(cmd, o, cx, data, results, res["fields"], so)
}

// finishResults either writes the untruncated results to --output and prints
// only file metadata, or prints the results with long field values truncated
// to --max-field-chars.
func finishResults(cmd *cobra.Command, o *Opts, cx *ctx, data map[string]any, results []map[string]any, fields any, so searchOpts) error {
	if fields == nil {
		fields = []string{}
	}
	if strings.TrimSpace(so.output) != "" {
		doc := map[string]any{
			"sid":            data["sid"],
			"dispatch_state": data["dispatch_state"],
			"search":         data["search"],
			"result_count":   data["result_count"],
			"fields":         fields,
			"results":        results,
		}
		n, err := writeJSONFile(so.output, doc)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "could not write --output file: "+err.Error(), "Pass a writable file path.", 400))
		}
		out := map[string]any{"path": so.output, "bytes": n, "result_count": data["result_count"], "results_written": len(results), "results_truncated": data["results_truncated"]}
		for _, key := range []string{"sid", "dispatch_state", "saved_search", "preview"} {
			if v, ok := data[key]; ok {
				out[key] = v
			}
		}
		return print(cmd, o, output.Success(cx.inst.Name, out))
	}
	truncated, names := truncateFieldValues(results, so.maxFieldChars)
	data["results"] = truncated
	data["fields"] = fields
	data["fields_truncated"] = names
	data["max_field_chars"] = so.maxFieldChars
	return print(cmd, o, output.Success(cx.inst.Name, data))
}

func writeJSONFile(path string, doc any) (int, error) {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return 0, err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return 0, err
	}
	return len(b), nil
}

func pageResults(all []map[string]any, offset, count int) []map[string]any {
	if offset >= len(all) {
		return []map[string]any{}
	}
	end := offset + count
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end]
}

func keepFields(results []map[string]any, fields []string) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, row := range results {
		kept := map[string]any{}
		for _, f := range fields {
			if v, ok := row[f]; ok {
				kept[f] = v
			}
		}
		out = append(out, kept)
	}
	return out
}

// truncateFieldValues cuts string field values (including multivalue
// entries) longer than max runes and reports the affected field names.
func truncateFieldValues(results []map[string]any, max int) ([]map[string]any, []string) {
	if max <= 0 {
		return results, []string{}
	}
	names := map[string]bool{}
	out := make([]map[string]any, 0, len(results))
	for _, row := range results {
		next := make(map[string]any, len(row))
		for key, value := range row {
			switch x := value.(type) {
			case string:
				if cut, ok := cutString(x, max); ok {
					names[key] = true
					next[key] = cut
				} else {
					next[key] = x
				}
			case []any:
				items := make([]any, len(x))
				for i, item := range x {
					if s, isString := item.(string); isString {
						if cut, ok := cutString(s, max); ok {
							names[key] = true
							items[i] = cut
							continue
						}
					}
					items[i] = item
				}
				next[key] = items
			default:
				next[key] = value
			}
		}
		out = append(out, next)
	}
	return out, sortedKeys(names)
}

func cutString(s string, max int) (string, bool) {
	runes := []rune(s)
	if len(runes) <= max {
		return s, false
	}
	return string(runes[:max]) + truncatedMarker, true
}
