package commands

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/jenkins"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

const (
	buildListDefaultLimit        = 20
	buildListMaxLimit            = 200
	buildListMaxScan             = 800
	buildParamsDefaultMaxChanges = 50

	// buildActionsTree selects the parameters and causes carried by a
	// build's actions[] so list and params share one normalizer.
	buildActionsTree   = "actions[parameters[name,value],causes[shortDescription,userId,userName,upstreamProject,upstreamBuild]]"
	buildSummaryFields = "number,url,result,building,timestamp,duration,displayName," + buildActionsTree
	changeItemsTree    = "items[commitId,msg,author[fullName],timestamp,affectedPaths]"
)

var validBuildResults = map[string]bool{"SUCCESS": true, "FAILURE": true, "UNSTABLE": true, "ABORTED": true, "NOT_BUILT": true}

// buildListCmd answers "which build of this job deployed X?" without a raw
// tree expression: it fetches the newest builds with their parameters and
// causes, filters them client-side, and reports how far back it looked so
// an agent can tell a complete answer from a truncated one.
func buildListCmd(o *Opts) *cobra.Command {
	var limit int
	var result, since string
	var params []string
	var building bool
	c := &cobra.Command{Use: "list <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return buildList(o, cmd, args[0], limit, result, since, params, building)
	}}
	c.Flags().IntVar(&limit, "limit", buildListDefaultLimit, "Maximum builds to return after filtering (1-200); the scan window is limit*4, capped at 800 newest builds.")
	c.Flags().StringVar(&result, "result", "", "Only builds with this result: SUCCESS, FAILURE, UNSTABLE, ABORTED, or NOT_BUILT (case-insensitive).")
	c.Flags().StringVar(&since, "since", "", "Only builds started at or after this point: a duration back from now (30m, 24h, 7d) or an RFC3339 timestamp.")
	c.Flags().StringArrayVar(&params, "param", nil, "Only builds whose parameter NAME equals VALUE exactly (NAME=VALUE); repeat to require several parameters.")
	c.Flags().BoolVar(&building, "building", false, "Only builds that are still in progress.")
	return c
}

func buildList(o *Opts, cmd *cobra.Command, job string, limit int, result, since string, rawParams []string, building bool) error {
	if limit <= 0 {
		limit = buildListDefaultLimit
	}
	if limit > buildListMaxLimit {
		limit = buildListMaxLimit
	}
	result = strings.ToUpper(strings.TrimSpace(result))
	if result != "" && !validBuildResults[result] {
		return print(cmd, o, output.Failure("invalid_args", fmt.Sprintf("unsupported --result %q", result), "Use SUCCESS, FAILURE, UNSTABLE, ABORTED, or NOT_BUILT.", 400))
	}
	sinceAt, err := parseSince(since, time.Now())
	if err != nil {
		return print(cmd, o, output.Failure("invalid_args", err.Error(), "Pass --since as a duration such as 30m, 24h, or 7d, or as an RFC3339 timestamp.", 400))
	}
	params, err := jenkins.ParseKeyValue(rawParams)
	if err != nil {
		return print(cmd, o, output.Failure("invalid_args", err.Error(), "Pass --param NAME=VALUE for each parameter the build must carry.", 400))
	}
	cx, err := loadCtx(o, job)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	window := limit * 4
	if window > buildListMaxScan {
		window = buildListMaxScan
	}
	// allBuilds (not builds) because Jenkins caps the builds property at the
	// 100 newest runs; the {0,N} range keeps the response bounded either way.
	resp, err := cx.client.Do(jenkins.Request{
		Method: http.MethodGet,
		Path:   jenkins.JobPath(job) + "/api/json",
		Query:  map[string]string{"tree": fmt.Sprintf("allBuilds[%s]{0,%d}", buildSummaryFields, window)},
	})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	data := jenkins.JSONMap(resp.Body)
	builds := make([]map[string]any, 0, limit)
	scanned, matched := 0, 0
	var oldestMS int64
	oldestKnown := false
	for _, raw := range listAny(data["allBuilds"]) {
		b, _ := raw.(map[string]any)
		if b == nil {
			continue
		}
		scanned++
		if ts, ok := timestampMS(b["timestamp"]); ok && (!oldestKnown || ts < oldestMS) {
			oldestMS, oldestKnown = ts, true
		}
		if !buildMatchesFilters(b, result, sinceAt, params, building) {
			continue
		}
		matched++
		if len(builds) < limit {
			builds = append(builds, summarizeBuild(b))
		}
	}
	// The scan window was exhausted, so older builds may also match, unless
	// --since is set and the window already reached past that point.
	windowExhausted := scanned >= window
	windowCoversSince := !sinceAt.IsZero() && oldestKnown && oldestMS < sinceAt.UnixMilli()
	var oldestISO any
	if oldestKnown {
		oldestISO = isoFromMS(oldestMS)
	}
	return print(cmd, o, output.Success(cx.inst.Name, map[string]any{
		"job":                          job,
		"builds":                       builds,
		"count_returned":               len(builds),
		"scanned":                      scanned,
		"filtered_from":                matched,
		"truncated":                    matched > len(builds) || (windowExhausted && !windowCoversSince),
		"limit":                        limit,
		"scan_window":                  window,
		"oldest_scanned_timestamp_iso": oldestISO,
	}))
}

func buildMatchesFilters(b map[string]any, result string, since time.Time, params map[string]string, building bool) bool {
	isBuilding, _ := b["building"].(bool)
	if building && !isBuilding {
		return false
	}
	if result != "" {
		got, _ := b["result"].(string)
		if !strings.EqualFold(got, result) {
			return false
		}
	}
	if !since.IsZero() {
		ts, ok := timestampMS(b["timestamp"])
		if !ok || ts < since.UnixMilli() {
			return false
		}
	}
	if len(params) > 0 {
		have := extractBuildParameters(b)
		for name, want := range params {
			got, ok := have[name]
			if !ok || paramValueString(got) != want {
				return false
			}
		}
	}
	return true
}

func summarizeBuild(b map[string]any) map[string]any {
	causes := []string{}
	for _, cause := range extractBuildCauses(b) {
		if desc, _ := cause["description"].(string); desc != "" {
			causes = append(causes, desc)
		}
	}
	building, _ := b["building"].(bool)
	return map[string]any{
		"number":        b["number"],
		"url":           b["url"],
		"display_name":  b["displayName"],
		"result":        b["result"],
		"building":      building,
		"timestamp":     b["timestamp"],
		"timestamp_iso": isoFromAny(b["timestamp"]),
		"duration_ms":   b["duration"],
		"parameters":    redactParameters(extractBuildParameters(b)),
		"causes":        causes,
	}
}

// buildParamsCmd exposes what a single build ran with (parameters), why it
// ran (causes), and which commits it carried (SCM changes) in one call, so
// deployment forensics do not need three raw API reads and knowledge of the
// pipeline-vs-freestyle changeSets/changeSet split.
func buildParamsCmd(o *Opts) *cobra.Command {
	var maxChanges int
	c := &cobra.Command{Use: "params <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return buildParams(o, cmd, args[0], args[1], maxChanges)
	}}
	c.Flags().IntVar(&maxChanges, "max-changes", buildParamsDefaultMaxChanges, "Maximum SCM change entries to return; 0 returns only changes_count_total.")
	return c
}

func buildParams(o *Opts, cmd *cobra.Command, job, build string, maxChanges int) error {
	if maxChanges < 0 {
		maxChanges = buildParamsDefaultMaxChanges
	}
	cx, err := loadCtx(o, job)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	// Pipeline runs expose changeSets[]; freestyle builds expose a single
	// changeSet. Jenkins ignores tree entries that do not apply, so asking
	// for both in one request covers either job type.
	tree := buildSummaryFields + ",changeSets[" + changeItemsTree + "],changeSet[" + changeItemsTree + "]"
	resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: jenkins.BuildPath(job, build) + "/api/json", Query: map[string]string{"tree": tree}})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	data := jenkins.JSONMap(resp.Body)
	changes, total := extractBuildChanges(data, maxChanges)
	building, _ := data["building"].(bool)
	return print(cmd, o, output.Success(cx.inst.Name, map[string]any{
		"number":              data["number"],
		"url":                 data["url"],
		"display_name":        data["displayName"],
		"result":              data["result"],
		"building":            building,
		"timestamp":           data["timestamp"],
		"timestamp_iso":       isoFromAny(data["timestamp"]),
		"duration_ms":         data["duration"],
		"parameters":          redactParameters(extractBuildParameters(data)),
		"causes":              extractBuildCauses(data),
		"changes":             changes,
		"changes_count_total": total,
		"changes_truncated":   total > len(changes),
	}))
}

func extractBuildParameters(b map[string]any) map[string]any {
	out := map[string]any{}
	for _, rawAction := range listAny(b["actions"]) {
		action, _ := rawAction.(map[string]any)
		for _, rawParam := range listAny(action["parameters"]) {
			p, _ := rawParam.(map[string]any)
			name, _ := p["name"].(string)
			if name == "" {
				continue
			}
			out[name] = p["value"]
		}
	}
	return out
}

func extractBuildCauses(b map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, rawAction := range listAny(b["actions"]) {
		action, _ := rawAction.(map[string]any)
		for _, rawCause := range listAny(action["causes"]) {
			c, _ := rawCause.(map[string]any)
			if c == nil {
				continue
			}
			out = append(out, map[string]any{
				"description":      c["shortDescription"],
				"user_id":          c["userId"],
				"user_name":        c["userName"],
				"upstream_project": c["upstreamProject"],
				"upstream_build":   c["upstreamBuild"],
			})
		}
	}
	return out
}

func extractBuildChanges(data map[string]any, maxChanges int) ([]map[string]any, int) {
	sets := listAny(data["changeSets"])
	if single, ok := data["changeSet"].(map[string]any); ok {
		sets = append(sets, single)
	}
	changes := make([]map[string]any, 0)
	total := 0
	for _, rawSet := range sets {
		set, _ := rawSet.(map[string]any)
		for _, rawItem := range listAny(set["items"]) {
			item, _ := rawItem.(map[string]any)
			if item == nil {
				continue
			}
			total++
			if len(changes) >= maxChanges {
				continue
			}
			var author any
			if a, ok := item["author"].(map[string]any); ok {
				author = a["fullName"]
			}
			changes = append(changes, map[string]any{
				"commit":        item["commitId"],
				"message":       item["msg"],
				"author":        author,
				"timestamp_iso": isoFromAny(item["timestamp"]),
				"files_count":   len(listAny(item["affectedPaths"])),
			})
		}
	}
	return changes, total
}

// redactParameters masks values of secret-looking parameter names so build
// listings never echo tokens that a job accepted as a plain string param.
func redactParameters(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for name, value := range params {
		if secretKey(name) && paramValueString(value) != "" {
			out[name] = "***REDACTED***"
			continue
		}
		out[name] = value
	}
	return out
}

func paramValueString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

func timestampMS(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		if x <= 0 {
			return 0, false
		}
		return int64(x), true
	case int64:
		if x <= 0 {
			return 0, false
		}
		return x, true
	case int:
		if x <= 0 {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
}

func isoFromMS(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func isoFromAny(v any) any {
	if ms, ok := timestampMS(v); ok {
		return isoFromMS(ms)
	}
	return nil
}

var sinceCalendarUnits = regexp.MustCompile(`^(\d+)([dw])$`)

// parseSince accepts a look-back duration (30m, 24h, 7d, 2w) or an absolute
// RFC3339 / date timestamp. An empty value means no lower bound.
func parseSince(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	if m := sinceCalendarUnits.FindStringSubmatch(strings.ToLower(raw)); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			return time.Time{}, fmt.Errorf("invalid --since %q: the look-back must be a positive number of days or weeks", raw)
		}
		unit := 24 * time.Hour
		if m[2] == "w" {
			unit = 7 * 24 * time.Hour
		}
		return now.Add(-time.Duration(n) * unit), nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q: use a duration such as 30m, 24h, or 7d, or an RFC3339 timestamp", raw)
	}
	if d <= 0 {
		return time.Time{}, fmt.Errorf("invalid --since %q: the look-back duration must be positive", raw)
	}
	return now.Add(-d), nil
}
