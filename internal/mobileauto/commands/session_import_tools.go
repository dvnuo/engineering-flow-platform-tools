package commands

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browserstack"
	"engineering-flow-platform-tools/internal/mobileauto"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type parsedBrowserStackURL struct {
	SessionID string `json:"session_id,omitempty"`
	BuildID   string `json:"build_id,omitempty"`
}

func sessionProbeCmd(o *Opts) *cobra.Command {
	opts := runImportOptions{Status: "running", Probe: true}
	var deep bool
	c := &cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(opts.FromURL) != "" {
			parsed := parseBrowserStackSessionURL(opts.FromURL)
			opts.SessionID = firstNonEmpty(opts.SessionID, parsed.SessionID)
			opts.BuildID = firstNonEmpty(opts.BuildID, parsed.BuildID)
		}
		if strings.TrimSpace(opts.SessionID) == "" && strings.TrimSpace(opts.BuildID) == "" && strings.TrimSpace(opts.BuildName) == "" {
			return print(cmd, o, output.Failure("invalid_args", "--session-id, --build-id, --build, or --from-url is required", "Pass one BrowserStack session or enough filters to probe exactly one session.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		remote, buildID, err := resolveImportedRemoteSession(cmd.Context(), svc, opts)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		result := probeRemoteSession(cmd.Context(), svc, remote, buildID, opts.Probe, deep)
		return print(cmd, o, output.Success("", result))
	}}
	c.Flags().StringVar(&opts.SessionID, "session-id", "", "BrowserStack session hashed id to probe.")
	c.Flags().StringVar(&opts.BuildID, "build-id", "", "BrowserStack build hashed id that contains the session.")
	c.Flags().StringVar(&opts.BuildName, "build", "", "BrowserStack build name used to resolve the session.")
	c.Flags().StringVar(&opts.SessionName, "name", "", "BrowserStack session name to match exactly, case-insensitively.")
	c.Flags().StringVar(&opts.Status, "status", "running", "Required BrowserStack session status before it is considered importable.")
	c.Flags().StringVar(&opts.ProjectID, "project-id", "", "BrowserStack project id used to constrain build search.")
	c.Flags().StringVar(&opts.Platform, "platform", "", "Mobile platform to match, for example android or ios.")
	c.Flags().StringVar(&opts.Device, "device", "", "BrowserStack device name to match.")
	c.Flags().StringVar(&opts.FromURL, "from-url", "", "BrowserStack dashboard/session URL to parse for session and build identifiers.")
	c.Flags().BoolVar(&opts.Probe, "probe", true, "Call the Appium hub to verify the session is controllable.")
	c.Flags().BoolVar(&deep, "deep", false, "When controllable, also check window size, source availability, and screenshot availability.")
	return c
}

func sessionCandidatesCmd(o *Opts) *cobra.Command {
	opts := sessionSearchOptions{Status: "running", Limit: 20}
	var probe, deep bool
	probe = true
	c := &cobra.Command{Use: "candidates", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		matches, err := searchRemoteSessions(cmd.Context(), svc, opts)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		candidates := make([]map[string]any, 0, len(matches))
		importable := 0
		for _, match := range matches {
			result := probeRemoteSession(cmd.Context(), svc, match.Session, match.BuildID, probe, deep)
			if result["importable"] == true {
				importable++
			}
			candidates = append(candidates, result)
		}
		return print(cmd, o, output.Success("", map[string]any{
			"candidates":       candidates,
			"count":            len(candidates),
			"importable_count": importable,
			"auth_scope":       "current_credentials",
		}))
	}}
	c.Flags().StringVar(&opts.BuildID, "build-id", "", "BrowserStack build hashed id to search within.")
	c.Flags().StringVar(&opts.BuildName, "build", "", "BrowserStack build name to match exactly, case-insensitively.")
	c.Flags().StringVar(&opts.Name, "name", "", "BrowserStack session name to match exactly, case-insensitively.")
	c.Flags().StringVar(&opts.Status, "status", "running", "BrowserStack session/build status filter such as running, done, failed, or timeout.")
	c.Flags().StringVar(&opts.ProjectID, "project-id", "", "BrowserStack project id used to constrain build search.")
	c.Flags().StringVar(&opts.Platform, "platform", "", "Mobile platform to match, for example android or ios.")
	c.Flags().StringVar(&opts.Device, "device", "", "BrowserStack device name to match exactly, case-insensitively.")
	c.Flags().IntVar(&opts.Limit, "limit", 20, "Maximum number of builds to scan when --build-id is not supplied.")
	c.Flags().IntVar(&opts.Offset, "offset", 0, "Build list offset when --build-id is not supplied.")
	c.Flags().BoolVar(&probe, "probe", true, "Call the Appium hub for each candidate to verify controllability.")
	c.Flags().BoolVar(&deep, "deep", false, "When controllable, also check window size, source availability, and screenshot availability.")
	return c
}

func runGuardCmd(o *Opts) *cobra.Command {
	var runID, hold string
	c := &cobra.Command{Use: "guard", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runID) == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the run id to keep alive.", 400))
		}
		holdDur, err := time.ParseDuration(hold)
		if err != nil || holdDur <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --hold-for", "Use a duration such as 10m.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var result map[string]any
		err = svc.Store.WithRunLock(runID, func() error {
			st, err := svc.Store.LoadRun(runID)
			if err != nil {
				return err
			}
			if !localRunStatusMayBeRemoteActive(st.Status) {
				return mobileauto.NewError("invalid_args", "run is not active enough to guard", "Only starting, running, waiting, or resuming runs can be guarded.", 400)
			}
			deadline := time.Now().UTC().Add(boundHold(holdDur, svc.Runtime.Mobile.BrowserStack.Local.MaxHoldMinutes))
			_ = stopKeeper(svc, runID)
			keeper, err := startKeeperMode(o, svc, runID, deadline, "guard")
			if err != nil {
				return mobileauto.NewError("keepalive_start_failed", "failed to start keepalive guard", "Retry guard or finish the run to avoid idle timeout.", 500)
			}
			st.KeepaliveDeadline = &deadline
			st.ProgressMessage = "keepalive guard active"
			if err := svc.Store.SaveRun(st); err != nil {
				return err
			}
			appendTimelineBestEffort(svc, runID, "run", "guard", "", st.Status, map[string]any{"deadline": deadline, "keeper": keeper})
			result = map[string]any{"run": st, "keeper": keeper, "deadline": deadline}
			return nil
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", result))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "Run id to keep alive.")
	c.Flags().StringVar(&hold, "hold-for", "10m", "Maximum guard duration before keepalive stops.")
	return c
}

func runDiagnoseCmd(o *Opts) *cobra.Command {
	var runID, outDir string
	var collectRemote, observe bool
	collectRemote = true
	c := &cobra.Command{Use: "diagnose", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runID) == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the run id to diagnose.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err := svc.Store.LoadRun(runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if outDir == "" {
			outDir = filepath.Join(svc.Store.RunDir(runID), "diagnostics")
		}
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			return renderErr(cmd, o, err)
		}
		var observation any
		if observe {
			err = svc.Store.WithRunLock(runID, func() error {
				current, err := svc.Store.LoadRun(runID)
				if err != nil {
					return err
				}
				obs, err := captureObservation(cmd.Context(), svc, &current, 100)
				if err != nil {
					return err
				}
				st = current
				observation = obs
				return nil
			})
			if err != nil {
				return renderErr(cmd, o, err)
			}
		}
		timeline, _ := svc.Store.LoadTimeline(runID)
		probe := map[string]any{}
		if st.SessionID != "" {
			remote := browserstack.Session{HashedID: st.SessionID, Status: string(st.Status), BrowserURL: st.BrowserURL, PublicURL: st.PublicURL, AppiumLogsURL: st.AppiumLogsURL, DeviceLogsURL: st.DeviceLogsURL, VideoURL: st.VideoURL}
			if fetched, err := svc.Control.GetSession(cmd.Context(), st.SessionID); err == nil {
				remote = fetched
			}
			probe = probeRemoteSession(cmd.Context(), svc, remote, st.BuildID, true, false)
		}
		var artifacts any
		if collectRemote {
			if bundle, err := collectArtifacts(cmd.Context(), svc, st, filepath.Join(outDir, "artifacts")); err == nil {
				artifacts = bundle
			} else {
				artifacts = map[string]any{"warning": err.Error()}
			}
		}
		capacity := map[string]any{}
		if plan, err := svc.Control.GetPlan(cmd.Context()); err == nil {
			capacity["plan"] = plan
		}
		if usage, err := svc.Control.CurrentParallelQueueUsage(cmd.Context()); err == nil {
			capacity["current_usage"] = usage
		}
		report := map[string]any{
			"run":            st,
			"probe":          probe,
			"timeline":       timeline,
			"timeline_count": len(timeline),
			"observation":    observation,
			"artifacts":      artifacts,
			"capacity":       capacity,
			"out_dir":        outDir,
			"sensitive":      true,
		}
		reportPath := filepath.Join(outDir, "diagnostic-report.json")
		b, err := json.MarshalIndent(output.RedactValue(report), "", "  ")
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if err := os.WriteFile(reportPath, b, 0o600); err != nil {
			return renderErr(cmd, o, err)
		}
		report["report_path"] = reportPath
		appendTimelineBestEffort(svc, runID, "run", "diagnose", "", st.Status, map[string]any{"path": reportPath, "observe": observe, "collect_remote": collectRemote})
		return print(cmd, o, output.Success("", report))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "Run id to diagnose.")
	c.Flags().StringVar(&outDir, "out", "", "Directory for diagnostic report and collected artifacts.")
	c.Flags().BoolVar(&collectRemote, "collect-artifacts", true, "Download available BrowserStack logs into the diagnostic directory.")
	c.Flags().BoolVar(&observe, "observe", false, "Capture a fresh observation before writing the diagnostic report.")
	return c
}

func runClaimCmd(o *Opts) *cobra.Command {
	var runID, owner, ttlText string
	c := &cobra.Command{Use: "claim", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runID) == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the run id to claim.", 400))
		}
		ttl, err := time.ParseDuration(ttlText)
		if err != nil || ttl <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --ttl", "Use a duration such as 15m.", 400))
		}
		if strings.TrimSpace(owner) == "" {
			owner = "agent"
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var st mobileauto.RunState
		err = svc.Store.WithRunLock(runID, func() error {
			var err error
			st, err = svc.Store.LoadRun(runID)
			if err != nil {
				return err
			}
			expires := time.Now().UTC().Add(ttl)
			st.ControlLeaseOwner = owner
			st.ControlLeaseExpiresAt = &expires
			if st.Metadata == nil {
				st.Metadata = map[string]string{}
			}
			st.Metadata["exclusive_control"] = "unknown_remote_not_enforced"
			if err := svc.Store.SaveRun(st); err != nil {
				return err
			}
			appendTimelineBestEffort(svc, runID, "run", "claim", "", st.Status, map[string]any{"owner": owner, "expires_at": expires})
			return nil
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"run": st, "exclusive_control": "local_lease_only"}))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "Run id to claim locally.")
	c.Flags().StringVar(&owner, "owner", "agent", "Local lease owner label.")
	c.Flags().StringVar(&ttlText, "ttl", "15m", "Local lease duration.")
	return c
}

func runReleaseCmd(o *Opts) *cobra.Command {
	var runID string
	c := &cobra.Command{Use: "release", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runID) == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the run id to release.", 400))
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var st mobileauto.RunState
		err = svc.Store.WithRunLock(runID, func() error {
			var err error
			st, err = svc.Store.LoadRun(runID)
			if err != nil {
				return err
			}
			st.ControlLeaseOwner = ""
			st.ControlLeaseExpiresAt = nil
			if st.Metadata != nil {
				delete(st.Metadata, "exclusive_control")
			}
			if err := svc.Store.SaveRun(st); err != nil {
				return err
			}
			appendTimelineBestEffort(svc, runID, "run", "release", "", st.Status, nil)
			return nil
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"run": st, "released": true}))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "Run id to release locally.")
	return c
}

func runMonitorCmd(o *Opts) *cobra.Command {
	var durationText, pollText, status, projectID string
	c := &cobra.Command{Use: "monitor", RunE: func(cmd *cobra.Command, args []string) error {
		duration, err := time.ParseDuration(durationText)
		if err != nil || duration < 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --duration", "Use 0 for one snapshot or a duration such as 2m.", 400))
		}
		poll, err := time.ParseDuration(pollText)
		if err != nil || poll <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --poll-interval", "Use a duration such as 15s.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		deadline := time.Now().UTC().Add(duration)
		var snapshots []map[string]any
		for {
			snapshot := sessionCapacitySnapshot(cmd.Context(), svc, status, projectID)
			snapshots = append(snapshots, snapshot)
			if duration == 0 || !time.Now().UTC().Before(deadline) {
				break
			}
			select {
			case <-cmd.Context().Done():
				return renderErr(cmd, o, cmd.Context().Err())
			case <-time.After(poll):
			}
		}
		return print(cmd, o, output.Success("", map[string]any{"snapshots": snapshots, "count": len(snapshots)}))
	}}
	c.Flags().StringVar(&durationText, "duration", "0s", "Monitor duration; 0s captures one snapshot.")
	c.Flags().StringVar(&pollText, "poll-interval", "15s", "Delay between monitor snapshots.")
	c.Flags().StringVar(&status, "status", "running", "BrowserStack session/build status to count.")
	c.Flags().StringVar(&projectID, "project-id", "", "BrowserStack project id used to constrain build search.")
	return c
}

func parseBrowserStackSessionURL(raw string) parsedBrowserStackURL {
	out := parsedBrowserStackURL{}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return out
	}
	q := u.Query()
	out.SessionID = firstNonEmpty(q.Get("session_id"), q.Get("sessionId"), q.Get("session"), q.Get("sid"))
	out.BuildID = firstNonEmpty(q.Get("build_id"), q.Get("buildId"), q.Get("build"))
	parts := strings.FieldsFunc(u.EscapedPath(), func(r rune) bool { return r == '/' || r == '\\' })
	for i, part := range parts {
		unescaped, _ := url.PathUnescape(part)
		key := strings.ToLower(strings.TrimSpace(unescaped))
		if i+1 >= len(parts) {
			continue
		}
		next, _ := url.PathUnescape(parts[i+1])
		next = strings.TrimSpace(next)
		switch key {
		case "sessions", "session":
			out.SessionID = firstNonEmpty(out.SessionID, next)
		case "builds", "build":
			out.BuildID = firstNonEmpty(out.BuildID, next)
		}
	}
	return out
}

func probeRemoteSession(ctx context.Context, svc *services, remote browserstack.Session, buildID string, doProbe bool, deep bool) map[string]any {
	sessionID := remote.HashedID
	appiumProbe := map[string]any{"attempted": doProbe}
	controllable := false
	if strings.TrimSpace(sessionID) == "" {
		appiumProbe["accepted"] = false
		appiumProbe["error"] = "missing session id"
	} else if doProbe {
		if status, err := svc.Appium.SessionStatus(ctx, sessionID); err == nil {
			controllable = true
			appiumProbe["accepted"] = true
			appiumProbe["status"] = "passed"
			appiumProbe["session_status"] = status
			if deep {
				appiumProbe["deep"] = deepProbe(ctx, svc, sessionID)
			}
		} else {
			code, message := errorCodeAndMessage(err)
			appiumProbe["accepted"] = false
			appiumProbe["status"] = "failed"
			appiumProbe["error_code"] = code
			appiumProbe["error"] = message
		}
	} else {
		appiumProbe["accepted"] = false
		appiumProbe["status"] = "skipped"
	}
	remoteActive := remoteSessionLooksActive(remote.Status)
	importable := remoteActive && (!doProbe || controllable)
	reason := "importable"
	switch {
	case !remoteActive:
		reason = "remote_status_not_running"
	case doProbe && !controllable:
		reason = "appium_probe_failed"
	}
	return map[string]any{
		"session_id":          sessionID,
		"build_id":            buildID,
		"build_name":          remote.BuildName,
		"session_name":        remote.Name,
		"remote_status":       remote.Status,
		"device":              remote.Device,
		"platform":            remote.OS,
		"project_name":        remote.ProjectName,
		"control_plane":       map[string]any{"visible": strings.TrimSpace(sessionID) != "", "status": remote.Status, "dashboard_url": firstNonEmpty(remote.BrowserURL, remote.PublicURL), "appium_logs_url": remote.AppiumLogsURL, "device_logs_url": remote.DeviceLogsURL, "video_url": remote.VideoURL},
		"appium_probe":        appiumProbe,
		"importable":          importable,
		"reason":              reason,
		"recommended_command": recommendedImportCommand(sessionID, buildID),
		"session":             remote,
	}
}

func deepProbe(ctx context.Context, svc *services, sessionID string) map[string]any {
	out := map[string]any{}
	if rect, err := svc.Appium.WindowRect(ctx, sessionID); err == nil {
		out["window_rect"] = rect
	} else {
		out["window_rect_error"] = errorSummary(err)
	}
	if source, err := svc.Appium.GetSource(ctx, sessionID); err == nil {
		out["source_bytes"] = len(source)
	} else {
		out["source_error"] = errorSummary(err)
	}
	if shot, err := svc.Appium.Screenshot(ctx, sessionID); err == nil {
		out["screenshot_bytes"] = len(shot)
	} else {
		out["screenshot_error"] = errorSummary(err)
	}
	return out
}

func remoteSessionLooksActive(status string) bool {
	_, terminal := runStatusForRemoteSession(status)
	if terminal {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "running", "created", "queued":
		return true
	default:
		return false
	}
}

func recommendedImportCommand(sessionID, buildID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return ""
	}
	cmd := "mobile-auto run import --session-id " + sessionID
	if strings.TrimSpace(buildID) != "" {
		cmd += " --build-id " + buildID
	}
	return cmd + " --json"
}

func errorSummary(err error) string {
	code, message := errorCodeAndMessage(err)
	if strings.TrimSpace(code) == "" {
		return strings.TrimSpace(message)
	}
	if strings.TrimSpace(message) == "" {
		return strings.TrimSpace(code)
	}
	return strings.TrimSpace(code) + ": " + strings.TrimSpace(message)
}

func sessionCapacitySnapshot(ctx context.Context, svc *services, status, projectID string) map[string]any {
	snapshot := map[string]any{"time": time.Now().UTC(), "status": status}
	if plan, err := svc.Control.GetPlan(ctx); err == nil {
		snapshot["plan"] = plan
	} else {
		snapshot["plan_error"] = err.Error()
	}
	if usage, err := svc.Control.CurrentParallelQueueUsage(ctx); err == nil {
		snapshot["current_usage"] = usage
	} else {
		snapshot["usage_error"] = err.Error()
	}
	matches, err := searchRemoteSessions(ctx, svc, sessionSearchOptions{Status: status, ProjectID: projectID, Limit: 20})
	if err == nil {
		snapshot["sessions"] = matches
		snapshot["session_count"] = len(matches)
	} else {
		snapshot["session_error"] = err.Error()
	}
	return snapshot
}
