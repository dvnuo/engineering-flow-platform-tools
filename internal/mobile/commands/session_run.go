package commands

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/appium"
	"engineering-flow-platform-tools/internal/browserstack"
	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func projectCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "project"}
	var limit, offset int
	var status string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		projects, err := svc.Control.ListProjects(cmd.Context(), limit, offset, status)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"projects": projects, "count": len(projects)}))
	}}
	list.Flags().IntVar(&limit, "limit", 20, "")
	list.Flags().IntVar(&offset, "offset", 0, "")
	list.Flags().StringVar(&status, "status", "", "")
	var id string
	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		if id == "" {
			return print(cmd, o, output.Failure("invalid_args", "--id is required", "Pass a BrowserStack project hashed id.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		project, err := svc.Control.GetProject(cmd.Context(), id)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", project))
	}}
	get.Flags().StringVar(&id, "id", "", "")
	c.AddCommand(list, get)
	return c
}

func buildCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "build"}
	var limit, offset int
	var status, projectID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		builds, err := svc.Control.ListBuilds(cmd.Context(), limit, offset, status, projectID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"builds": builds, "count": len(builds)}))
	}}
	list.Flags().IntVar(&limit, "limit", 20, "")
	list.Flags().IntVar(&offset, "offset", 0, "")
	list.Flags().StringVar(&status, "status", "", "")
	list.Flags().StringVar(&projectID, "project-id", "", "")
	var id string
	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		if id == "" {
			return print(cmd, o, output.Failure("invalid_args", "--id is required", "Pass a BrowserStack build hashed id.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		sessions, err := svc.Control.ListBuildSessions(cmd.Context(), id, 100, 0, "")
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"build_id": id, "sessions": sessions, "session_count": len(sessions)}))
	}}
	get.Flags().StringVar(&id, "id", "", "")
	c.AddCommand(list, get)
	return c
}

func sessionCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "session"}
	c.AddCommand(sessionStartCmd(o), sessionStatusCmd(o), sessionListCmd(o), sessionGetCmd(o), sessionMarkCmd(o), sessionStopCmd(o))
	return c
}

func sessionStartCmd(o *Opts) *cobra.Command {
	opts := runStartOptions{}
	c := &cobra.Command{Use: "start", RunE: func(cmd *cobra.Command, args []string) error {
		return runStart(cmd, o, opts)
	}}
	bindRunStartFlags(c, &opts)
	return c
}

func sessionStatusCmd(o *Opts) *cobra.Command {
	var runID, sessionID string
	c := &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if sessionID == "" {
			if runID == "" {
				return print(cmd, o, output.Failure("invalid_args", "--run-id or --session-id is required", "Pass the local run id or remote session id.", 400))
			}
			st, err := svc.Store.LoadRun(runID)
			if err != nil {
				return renderErr(cmd, o, err)
			}
			sessionID = st.SessionID
		}
		status, err := svc.Appium.SessionStatus(cmd.Context(), sessionID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", status))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&sessionID, "session-id", "", "")
	return c
}

func sessionListCmd(o *Opts) *cobra.Command {
	var buildID string
	var limit, offset int
	var status string
	c := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		if buildID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--build-id is required", "BrowserStack session listing is scoped to a build id.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		sessions, err := svc.Control.ListBuildSessions(cmd.Context(), buildID, limit, offset, status)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"sessions": sessions, "count": len(sessions)}))
	}}
	c.Flags().StringVar(&buildID, "build-id", "", "")
	c.Flags().IntVar(&limit, "limit", 20, "")
	c.Flags().IntVar(&offset, "offset", 0, "")
	c.Flags().StringVar(&status, "status", "", "")
	return c
}

func sessionGetCmd(o *Opts) *cobra.Command {
	var sessionID string
	c := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		if sessionID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--session-id is required", "Pass the BrowserStack session id.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		session, err := svc.Control.GetSession(cmd.Context(), sessionID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", session))
	}}
	c.Flags().StringVar(&sessionID, "session-id", "", "")
	return c
}

func sessionMarkCmd(o *Opts) *cobra.Command {
	var sessionID, name, status, reason string
	c := &cobra.Command{Use: "mark", RunE: func(cmd *cobra.Command, args []string) error {
		if sessionID == "" || (name == "" && status == "" && reason == "") {
			return print(cmd, o, output.Failure("invalid_args", "--session-id and at least one of --name/--status/--reason are required", "Pass documented BrowserStack session metadata updates.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		session, err := svc.Control.UpdateSession(cmd.Context(), sessionID, browserstack.UpdateSessionRequest{Name: name, Status: status, Reason: reason})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", session))
	}}
	c.Flags().StringVar(&sessionID, "session-id", "", "")
	c.Flags().StringVar(&name, "name", "", "")
	c.Flags().StringVar(&status, "status", "", "")
	c.Flags().StringVar(&reason, "reason", "", "")
	return c
}

func sessionStopCmd(o *Opts) *cobra.Command {
	var runID, sessionID string
	var yes, dryRun bool
	c := &cobra.Command{Use: "stop", RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes is required for session stop", "Re-run with --yes after confirming.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if sessionID == "" && runID != "" {
			st, err := svc.Store.LoadRun(runID)
			if err != nil {
				return renderErr(cmd, o, err)
			}
			sessionID = st.SessionID
		}
		if sessionID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--session-id or --run-id is required", "Pass the target remote session.", 400))
		}
		if dryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "session_id": sessionID}))
		}
		if err := svc.Appium.DeleteSession(cmd.Context(), sessionID); err != nil {
			return renderErr(cmd, o, err)
		}
		if runID != "" {
			_ = svc.Store.WithRunLock(runID, func() error {
				st, err := svc.Store.LoadRun(runID)
				if err != nil {
					return err
				}
				now := time.Now().UTC()
				st.Status = mobile.StatusFinished
				st.FinishedAt = &now
				st.LatestObservationID = ""
				return svc.Store.SaveRun(st)
			})
		}
		return print(cmd, o, output.Success("", map[string]any{"stopped": true, "session_id": sessionID}))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&sessionID, "session-id", "", "")
	c.Flags().BoolVar(&yes, "yes", false, "")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "")
	return c
}

func runCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "run"}
	c.AddCommand(runStartCmd(o), runStatusCmd(o), runHandoffCmd(o), runResumeCmd(o), runFinishCmd(o))
	return c
}

type runStartOptions struct {
	AppURL    string
	File      string
	URL       string
	CustomID  string
	Platform  string
	Device    string
	OSVersion string
	MinOS     string
	Network   string
	LocalID   string
	Project   string
	Build     string
	Name      string
	WaitCap   bool
	Timeout   string
	Poll      string
}

func runStartCmd(o *Opts) *cobra.Command {
	opts := runStartOptions{}
	c := &cobra.Command{Use: "start", RunE: func(cmd *cobra.Command, args []string) error {
		return runStart(cmd, o, opts)
	}}
	bindRunStartFlags(c, &opts)
	return c
}

func bindRunStartFlags(c *cobra.Command, opts *runStartOptions) {
	c.Flags().StringVar(&opts.AppURL, "app", "", "")
	c.Flags().StringVar(&opts.File, "file", "", "")
	c.Flags().StringVar(&opts.URL, "url", "", "")
	c.Flags().StringVar(&opts.CustomID, "custom-id", "", "")
	c.Flags().StringVar(&opts.Platform, "platform", "", "")
	c.Flags().StringVar(&opts.Device, "device", "", "")
	c.Flags().StringVar(&opts.OSVersion, "os-version", "", "")
	c.Flags().StringVar(&opts.MinOS, "min-os-version", "", "")
	c.Flags().StringVar(&opts.Network, "network", "", "")
	c.Flags().StringVar(&opts.LocalID, "local-identifier", "", "")
	c.Flags().StringVar(&opts.Project, "project", "", "")
	c.Flags().StringVar(&opts.Build, "build", "", "")
	c.Flags().StringVar(&opts.Name, "name", "", "")
	c.Flags().BoolVar(&opts.WaitCap, "wait-capacity", false, "")
	c.Flags().StringVar(&opts.Timeout, "timeout", "5m", "")
	c.Flags().StringVar(&opts.Poll, "poll-interval", "15s", "")
}

func runStart(cmd *cobra.Command, o *Opts, opts runStartOptions) error {
	timeout, err := time.ParseDuration(opts.Timeout)
	if err != nil || timeout <= 0 {
		return print(cmd, o, output.Failure("invalid_args", "invalid --timeout", "Use a positive duration such as 5m.", 400))
	}
	svc, err := newServices(o, true)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	cfg := svc.Runtime.Mobile
	if opts.Platform == "" {
		opts.Platform = cfg.Defaults.Platform
	}
	if opts.Network == "" {
		opts.Network = cfg.Defaults.NetworkMode
	}
	if opts.Network != "public" && opts.Network != "private-managed" && opts.Network != "private-external" {
		return print(cmd, o, output.Failure("invalid_args", "--network must be public, private-managed, or private-external", "Use public unless the app needs private/internal hosts.", 400))
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	appRef, err := resolveApp(ctx, svc, opts)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	devices, err := svc.Control.ListDevices(ctx)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	dev, err := mobile.ResolveDevice(deviceInfos(devices), mobile.DeviceQuery{Platform: opts.Platform, Name: opts.Device, OSVersion: opts.OSVersion, MinOSVersion: opts.MinOS, RealOnly: true, Strategy: "latest-compatible"})
	if err != nil {
		return renderErr(cmd, o, err)
	}
	if opts.WaitCap {
		if err := waitCapacity(ctx, svc, 1, opts.Poll); err != nil {
			return renderErr(cmd, o, err)
		}
	}
	runID := mobile.NewRunID()
	var tunnel mobile.TunnelState
	if opts.Network != "public" {
		hold := time.Duration(cfg.BrowserStack.Local.DefaultHoldMinutes) * time.Minute
		tunnel, err = svc.Tunnel.Start(mobile.TunnelStartRequest{RunID: runID, NetworkMode: opts.Network, LocalIdentifier: opts.LocalID, HoldFor: hold})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		opts.LocalID = tunnel.LocalIdentifier
	}
	automation := "UiAutomator2"
	if equalFold(opts.Platform, "ios") {
		automation = "XCUITest"
	}
	session, err := svc.Appium.CreateSession(ctx, appium.CreateSessionRequest{
		PlatformName:             opts.Platform,
		AutomationName:           automation,
		App:                      appRef.AppURL,
		DeviceName:               dev.Recommended.Name,
		PlatformVersion:          dev.Recommended.OSVersion,
		ProjectName:              opts.Project,
		BuildName:                opts.Build,
		SessionName:              opts.Name,
		NetworkMode:              opts.Network,
		LocalIdentifier:          opts.LocalID,
		InteractiveDebugging:     cfg.Defaults.InteractiveDebugging != nil && *cfg.Defaults.InteractiveDebugging,
		Video:                    cfg.Defaults.Video != nil && *cfg.Defaults.Video,
		NewCommandTimeoutSeconds: cfg.Defaults.NewCommandTimeoutSeconds,
	})
	if err != nil {
		if tunnel.Managed {
			_, _ = svc.Tunnel.Stop(tunnel)
		}
		return renderErr(cmd, o, err)
	}
	now := time.Now().UTC()
	st := mobile.RunState{
		Version:      1,
		RunID:        runID,
		Provider:     "browserstack",
		Status:       mobile.StatusRunning,
		ControlOwner: "agent",
		SessionID:    session.ID,
		Platform:     opts.Platform,
		Device:       dev.Recommended,
		App:          appRef,
		Network:      mobile.NetworkState{Mode: opts.Network, LocalIdentifier: opts.LocalID, TunnelID: tunnel.TunnelID},
		ProjectName:  opts.Project,
		BuildName:    opts.Build,
		SessionName:  opts.Name,
		StartedAt:    now,
		UpdatedAt:    now,
	}
	if err := svc.Store.SaveRun(st); err != nil {
		return renderErr(cmd, o, err)
	}
	return print(cmd, o, output.Success("", map[string]any{"run": st, "session": session, "device_resolution": dev, "tunnel": tunnel}))
}

func resolveApp(ctx context.Context, svc *services, opts runStartOptions) (mobile.AppRef, error) {
	if strings.HasPrefix(opts.AppURL, "bs://") {
		return mobile.AppRef{AppURL: opts.AppURL, CustomID: opts.CustomID}, nil
	}
	if opts.File == "" && opts.URL == "" && opts.CustomID == "" {
		return mobile.AppRef{}, mobile.NewError("invalid_args", "--app, --file, --url, or --custom-id is required", "Pass an existing bs:// app or app source.", 400)
	}
	var sha string
	var err error
	if opts.File != "" {
		if !browserstack.ValidAppExtension(opts.File) {
			return mobile.AppRef{}, mobile.NewError("invalid_args", "unsupported app extension", "Use .apk, .aab, .xapk, or .ipa.", 400)
		}
		sha, err = browserstack.SHA256File(opts.File)
		if err != nil {
			return mobile.AppRef{}, err
		}
		if cached, err := svc.Store.LoadAppCache(sha); err == nil && cached.AppURL != "" {
			return cached, nil
		}
	}
	if opts.CustomID != "" {
		apps, err := svc.Control.ListApps(ctx, browserstack.ListAppsRequest{Limit: 20, CustomID: opts.CustomID})
		if err == nil {
			for _, app := range apps {
				if app.AppURL != "" {
					return mobile.AppRef{AppURL: app.AppURL, CustomID: opts.CustomID, SHA256: sha, Name: app.AppName}, nil
				}
			}
		}
	}
	app, err := svc.Control.UploadApp(ctx, browserstack.UploadAppRequest{FilePath: opts.File, URL: opts.URL, CustomID: opts.CustomID, SHA256: sha})
	if err != nil {
		return mobile.AppRef{}, err
	}
	ref := mobile.AppRef{AppURL: app.AppURL, CustomID: opts.CustomID, SHA256: sha, Name: app.AppName}
	_ = svc.Store.SaveAppCache(ref)
	return ref, nil
}

func waitCapacity(ctx context.Context, svc *services, required int, pollText string) error {
	poll, err := time.ParseDuration(pollText)
	if err != nil || poll <= 0 {
		poll = 15 * time.Second
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		plan, err := svc.Control.GetPlan(ctx)
		if err != nil {
			return err
		}
		allowed := plan.TeamParallelSessionsMaxAllowed
		if allowed == 0 {
			allowed = plan.ParallelSessionsMaxAllowed
		}
		if allowed-plan.ParallelSessionsRunning >= required {
			return nil
		}
		select {
		case <-ctx.Done():
			return mobile.RetryableError("capacity_wait_timeout", "timed out waiting for BrowserStack capacity", "Retry later or lower required parallel sessions.", "retry", 408)
		case <-t.C:
		}
	}
}

func runStatusCmd(o *Opts) *cobra.Command {
	var runID string
	c := &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if runID == "" {
			runs, err := svc.Store.ListRuns()
			if err != nil {
				return renderErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"runs": runs, "count": len(runs)}))
		}
		st, err := svc.Store.LoadRun(runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", st))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	return c
}

func runHandoffCmd(o *Opts) *cobra.Command {
	var runID, hold string
	c := &cobra.Command{Use: "handoff", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the running run id.", 400))
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
			obs, err := captureObservation(cmd.Context(), svc, &st, 100)
			if err != nil {
				return err
			}
			deadline := time.Now().UTC().Add(boundHold(holdDur, svc.Runtime.Mobile.BrowserStack.Local.MaxHoldMinutes))
			st.ControlOwner = "human"
			st.Status = mobile.StatusWaitingForHuman
			if err := svc.Store.SaveRun(st); err != nil {
				return err
			}
			keeper, _ := startKeeper(o, svc, st.RunID, deadline)
			result = map[string]any{
				"run":              st,
				"observation":      obs,
				"hold_deadline":    deadline,
				"keeper":           keeper,
				"capacity_warning": "This BrowserStack session continues to consume parallel capacity until resume or finish.",
			}
			return nil
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", result))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&hold, "hold-for", "10m", "")
	return c
}

func runResumeCmd(o *Opts) *cobra.Command {
	var runID string
	c := &cobra.Command{Use: "resume", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the waiting run id.", 400))
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
			if st.Status != mobile.StatusWaitingForHuman || st.ControlOwner != "human" {
				return mobile.NewError("invalid_args", "run is not waiting for human control", "Only waiting_for_human runs can resume.", 400)
			}
			_ = stopKeeper(svc, runID)
			st.Status = mobile.StatusResuming
			st.ControlOwner = "agent"
			st.LatestObservationID = ""
			st.ObservationVersion++
			if err := svc.Store.SaveRun(st); err != nil {
				return err
			}
			obs, err := captureObservation(cmd.Context(), svc, &st, 100)
			if err != nil {
				st.Status = mobile.StatusLost
				_ = svc.Store.SaveRun(st)
				return err
			}
			st.Status = mobile.StatusRunning
			if err := svc.Store.SaveRun(st); err != nil {
				return err
			}
			result = map[string]any{"run": st, "observation": obs}
			return nil
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", result))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	return c
}

func runFinishCmd(o *Opts) *cobra.Command {
	var runID, status, reason string
	var collect bool
	c := &cobra.Command{Use: "finish", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the run to finish.", 400))
		}
		status, ok := normalizeFinishStatus(status)
		if !ok {
			return print(cmd, o, output.Failure("invalid_args", "--status must be passed or failed", "Use --status passed or --status failed.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var result map[string]any
		cleanup := []string{}
		err = svc.Store.WithRunLock(runID, func() error {
			st, err := svc.Store.LoadRun(runID)
			if err != nil {
				return err
			}
			if collect {
				if bundle, err := collectArtifacts(cmd.Context(), svc, st, ""); err == nil {
					result = map[string]any{"artifacts": bundle}
				} else {
					cleanup = append(cleanup, "artifact collection: "+err.Error())
				}
			}
			if st.SessionID != "" {
				mark := status
				if _, err := svc.Control.UpdateSession(cmd.Context(), st.SessionID, browserstack.UpdateSessionRequest{Status: mark, Reason: reason}); err != nil {
					cleanup = append(cleanup, "session mark: "+err.Error())
				}
				if err := svc.Appium.DeleteSession(cmd.Context(), st.SessionID); err != nil {
					cleanup = append(cleanup, "session delete: "+err.Error())
				}
			}
			_ = stopKeeper(svc, runID)
			if st.Network.LocalIdentifier != "" {
				if tunnel, err := svc.Tunnel.Load(runID, st.Network.LocalIdentifier); err == nil && tunnel.Managed {
					if _, err := svc.Tunnel.Stop(tunnel); err != nil {
						cleanup = append(cleanup, "tunnel stop: "+err.Error())
					}
				}
			}
			now := time.Now().UTC()
			if status == "failed" {
				st.Status = mobile.StatusFailed
			} else {
				st.Status = mobile.StatusFinished
			}
			st.ControlOwner = "agent"
			st.FinishedAt = &now
			st.LatestObservationID = ""
			if err := svc.Store.SaveRun(st); err != nil {
				return err
			}
			if result == nil {
				result = map[string]any{}
			}
			result["run"] = st
			result["cleanup_errors"] = cleanup
			if len(cleanup) > 0 {
				result["warning"] = "cleanup_partial"
			}
			return nil
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", result))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&status, "status", "passed", "")
	c.Flags().StringVar(&reason, "reason", "", "")
	c.Flags().BoolVar(&collect, "collect-artifacts", false, "")
	return c
}

func normalizeFinishStatus(status string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		normalized = "passed"
	}
	if normalized != "passed" && normalized != "failed" {
		return "", false
	}
	return normalized, true
}

func boundHold(d time.Duration, maxMinutes int) time.Duration {
	if maxMinutes <= 0 {
		maxMinutes = 30
	}
	max := time.Duration(maxMinutes) * time.Minute
	if d > max {
		return max
	}
	return d
}

type keeperInfo struct {
	PID      int       `json:"pid"`
	Deadline time.Time `json:"deadline"`
	LogPath  string    `json:"log_path"`
}

func startKeeper(o *Opts, svc *services, runID string, deadline time.Time) (keeperInfo, error) {
	exe, err := os.Executable()
	if err != nil {
		return keeperInfo{}, err
	}
	logPath := filepath.Join(svc.Store.RunDir(runID), "keeper.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return keeperInfo{}, err
	}
	args := []string{"_keepalive", "--run-id", runID, "--deadline", deadline.Format(time.RFC3339)}
	if o.ConfigPath != "" {
		args = append(args, "--config", o.ConfigPath)
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return keeperInfo{}, err
	}
	_ = log.Close()
	info := keeperInfo{PID: cmd.Process.Pid, Deadline: deadline, LogPath: logPath}
	_ = writeJSON(filepath.Join(svc.Store.RunDir(runID), "keeper.json"), info)
	return info, nil
}

func stopKeeper(svc *services, runID string) error {
	path := filepath.Join(svc.Store.RunDir(runID), "keeper.json")
	var info keeperInfo
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(b, &info); err != nil {
		return nil
	}
	if info.PID > 0 {
		if p, err := os.FindProcess(info.PID); err == nil {
			_ = p.Kill()
		}
	}
	_ = os.Remove(path)
	return nil
}

func keepaliveCmd(o *Opts) *cobra.Command {
	var runID, deadlineText string
	c := &cobra.Command{Use: "_keepalive", Hidden: true, RunE: func(cmd *cobra.Command, args []string) error {
		deadline, err := time.Parse(time.RFC3339, deadlineText)
		if err != nil || runID == "" {
			return nil
		}
		svc, err := newServices(o, true)
		if err != nil {
			return nil
		}
		heartbeat := time.Duration(svc.Runtime.Mobile.BrowserStack.Local.HeartbeatSeconds) * time.Second
		if heartbeat <= 0 {
			heartbeat = 60 * time.Second
		}
		t := time.NewTicker(heartbeat)
		defer t.Stop()
		for time.Now().Before(deadline) {
			st, err := svc.Store.LoadRun(runID)
			if err != nil || st.ControlOwner != "human" || st.Status != mobile.StatusWaitingForHuman {
				return nil
			}
			_, _ = svc.Appium.GetSource(cmd.Context(), st.SessionID)
			select {
			case <-cmd.Context().Done():
				return nil
			case <-t.C:
			}
		}
		return nil
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&deadlineText, "deadline", "", "")
	return c
}

func writeJSON(path string, value any) error {
	b, _ := json.MarshalIndent(value, "", "  ")
	return os.WriteFile(path, b, 0o600)
}
