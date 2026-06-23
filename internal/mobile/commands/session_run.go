package commands

import (
	"context"
	"encoding/json"
	"errors"
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
	c.AddCommand(runStartCmd(o), runStatusCmd(o), runRecoverCmd(o), runHandoffCmd(o), runResumeCmd(o), runFinishCmd(o))
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
	var runID string
	var st mobile.RunState
	var startingPersisted bool
	var runStartCause error
	fail := func(err error) error {
		runStartCause = err
		return renderErr(cmd, o, err)
	}
	defer func() {
		if startingPersisted {
			settleStartingRunBestEffort(svc, runID, st, runStartCause)
		}
	}()
	appRef, err := resolveApp(ctx, svc, opts)
	if err != nil {
		return fail(err)
	}
	devices, err := svc.Control.ListDevices(ctx)
	if err != nil {
		return fail(err)
	}
	dev, err := mobile.ResolveDevice(deviceInfos(devices), mobile.DeviceQuery{Platform: opts.Platform, Name: opts.Device, OSVersion: opts.OSVersion, MinOSVersion: opts.MinOS, RealOnly: true, Strategy: "latest-compatible"})
	if err != nil {
		return fail(err)
	}
	if opts.WaitCap {
		if err := waitCapacity(ctx, svc, 1, opts.Poll); err != nil {
			return fail(err)
		}
	}
	runID = mobile.NewRunID()
	var tunnel mobile.TunnelState
	if opts.Network != "public" {
		hold := time.Duration(cfg.BrowserStack.Local.DefaultHoldMinutes) * time.Minute
		tunnel, err = svc.Tunnel.Start(mobile.TunnelStartRequest{RunID: runID, NetworkMode: opts.Network, LocalIdentifier: opts.LocalID, HoldFor: hold})
		if err != nil {
			return fail(err)
		}
		opts.LocalID = tunnel.LocalIdentifier
	}
	startedAt := time.Now().UTC()
	st = mobile.RunState{
		Version:      1,
		RunID:        runID,
		Provider:     "browserstack",
		Status:       mobile.StatusStarting,
		ControlOwner: "agent",
		Platform:     opts.Platform,
		Device:       dev.Recommended,
		App:          appRef,
		Network:      mobile.NetworkState{Mode: opts.Network, LocalMode: localModeForNetwork(opts.Network), LocalIdentifier: opts.LocalID, TunnelID: tunnel.TunnelID},
		ProjectName:  opts.Project,
		BuildName:    opts.Build,
		SessionName:  opts.Name,
		StartedAt:    startedAt,
		UpdatedAt:    startedAt,
	}
	if err := svc.Store.SaveRun(st); err != nil {
		if tunnel.Managed {
			_, _ = svc.Tunnel.Stop(tunnel)
		}
		return fail(mobile.NewError("state_persist_failed", "failed to persist initial run state", "Check mobile.state_dir permissions and free disk space.", 500))
	}
	startingPersisted = true
	automation := "UiAutomator2"
	if equalFold(opts.Platform, "ios") {
		automation = "XCUITest"
	}
	sessionCreateStarted := time.Now().UTC()
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
		IdleTimeoutSeconds:       cfg.Defaults.IdleTimeoutSeconds,
		NewCommandTimeoutSeconds: cfg.Defaults.NewCommandTimeoutSeconds,
	})
	var remoteSession browserstack.Session
	var haveRemoteSession bool
	var recoveredSession bool
	var recoveryAttempted bool
	buildID := ""
	if err != nil {
		if shouldRecoverCreateSessionError(err) {
			recoveryAttempted = true
			recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), 20*time.Second)
			recovered, recoveredBuildID, recoverErr := recoverCreatedSession(recoveryCtx, svc, opts, dev.Recommended, sessionCreateStarted)
			recoveryCancel()
			if recoverErr == nil {
				remoteSession = recovered
				haveRemoteSession = true
				recoveredSession = true
				buildID = recoveredBuildID
				session = appium.Session{ID: recovered.HashedID}
			} else {
				if tunnel.Managed {
					_, _ = svc.Tunnel.Stop(tunnel)
				}
				err = sessionRecoveryFailedError(err, recoverErr)
				markRunFailedBestEffort(svc, &st, err)
				return fail(err)
			}
		} else {
			if tunnel.Managed {
				_, _ = svc.Tunnel.Stop(tunnel)
			}
			markRunFailedBestEffort(svc, &st, err)
			return fail(err)
		}
	}
	now := time.Now().UTC()
	st.Status = mobile.StatusRunning
	st.SessionID = session.ID
	st.BuildID = buildID
	st.DashboardURL = dashboardURLFromSession(session)
	st.UpdatedAt = now
	if haveRemoteSession {
		enrichRunStateFromRemote(&st, remoteSession)
	}
	if err := svc.Store.SaveRun(st); err != nil {
		return fail(mobile.NewError("state_persist_failed", "failed to persist running run state after session creation", "The remote session may be active; check mobile.state_dir and use run recover if needed.", 500))
	}
	warnings := enrichRunStateBestEffort(svc, &st, session.ID, opts.Build, sessionCreateStarted, haveRemoteSession)
	return print(cmd, o, output.Success("", map[string]any{
		"run":                    st,
		"session":                session,
		"device_resolution":      dev,
		"tunnel":                 tunnel,
		"effective":              effectiveRunStart(st, dev),
		"state_persisted":        true,
		"remote_session_created": st.SessionID != "",
		"recovery_attempted":     recoveryAttempted,
		"recovered_session":      recoveredSession,
		"enrich_warnings":        warnings,
		"warnings":               warnings,
	}))
}

func markRunFailedBestEffort(svc *services, st *mobile.RunState, err error) {
	markRunTerminal(st, mobile.StatusFailed, "run start failed before a usable session was saved", err)
	_ = svc.Store.SaveRun(*st)
}

func settleStartingRunBestEffort(svc *services, runID string, fallback mobile.RunState, cause error) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	current, err := svc.Store.LoadRun(runID)
	if err != nil || current.Status != mobile.StatusStarting {
		return
	}
	mergeKnownRunState(&current, fallback)
	status := mobile.StatusFailed
	reason := "run start exited before a usable remote session was identified"
	if strings.TrimSpace(current.SessionID) != "" || strings.TrimSpace(current.BrowserStackSessionID) != "" {
		status = mobile.StatusLost
		reason = "remote session was identified but run start exited before local control became usable"
	}
	if cause == nil {
		cause = mobile.NewError("run_start_incomplete", "run start exited while local state was still starting", "Inspect the command output and retry or use mobile run recover.", 500)
	}
	markRunTerminal(&current, status, reason, cause)
	_ = svc.Store.SaveRun(current)
}

func mergeKnownRunState(dst *mobile.RunState, src mobile.RunState) {
	dst.SessionID = firstNonEmpty(dst.SessionID, src.SessionID)
	dst.BrowserStackSessionID = firstNonEmpty(dst.BrowserStackSessionID, src.BrowserStackSessionID)
	dst.BuildID = firstNonEmpty(dst.BuildID, src.BuildID)
	dst.DashboardURL = firstNonEmpty(dst.DashboardURL, src.DashboardURL)
	dst.BrowserURL = firstNonEmpty(dst.BrowserURL, src.BrowserURL)
	dst.PublicURL = firstNonEmpty(dst.PublicURL, src.PublicURL)
	dst.AppiumLogsURL = firstNonEmpty(dst.AppiumLogsURL, src.AppiumLogsURL)
	dst.DeviceLogsURL = firstNonEmpty(dst.DeviceLogsURL, src.DeviceLogsURL)
	dst.VideoURL = firstNonEmpty(dst.VideoURL, src.VideoURL)
}

func shouldRecoverCreateSessionError(err error) bool {
	var missingID *appium.SessionIDMissingError
	if errors.As(err, &missingID) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var me *mobile.Error
	if !errors.As(err, &me) {
		return false
	}
	switch me.Code {
	case "network_error", "session_creation_failed", "browserstack_session_error":
		return true
	case "server_error":
		return me.Status == 0 || me.Status >= 500
	default:
		return false
	}
}

func sessionRecoveryFailedError(createErr, recoverErr error) error {
	createCode, createMessage := errorCodeAndMessage(createErr)
	recoverCode, recoverMessage := errorCodeAndMessage(recoverErr)
	message := "Appium session creation did not leave a usable local session and BrowserStack control-plane recovery failed"
	if createCode != "" || recoverCode != "" {
		message += ": create=" + createCode + " " + createMessage + "; recovery=" + recoverCode + " " + recoverMessage
	}
	return mobile.NewError("session_recovery_failed", message, "Use unique --build and --name values, inspect BrowserStack builds/sessions, or run mobile run recover with the BrowserStack session id.", 502)
}

func enrichRunStateBestEffort(svc *services, st *mobile.RunState, sessionID, buildName string, since time.Time, alreadyHaveRemote bool) []string {
	warnings := []string{}
	changed := false
	enrichCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if !alreadyHaveRemote && sessionID != "" {
		if remote, err := svc.Control.GetSession(enrichCtx, sessionID); err == nil {
			enrichRunStateFromRemote(st, remote)
			changed = true
		} else {
			warnings = append(warnings, "browserstack session enrich: "+err.Error())
		}
	}
	if st.BuildID == "" {
		if buildID := findBuildIDByName(enrichCtx, svc, buildName, since); buildID != "" {
			st.BuildID = buildID
			changed = true
		}
	}
	if changed {
		if err := svc.Store.SaveRun(*st); err != nil {
			warnings = append(warnings, "run state enrich save: "+err.Error())
		}
	}
	return warnings
}

func dashboardURLFromSession(session appium.Session) string {
	for _, key := range []string{"browserstack.sessionUrl", "browserstack.session_url", "sessionUrl", "session_url"} {
		if value, ok := session.Capabilities[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type recoveredSessionCandidate struct {
	Session browserstack.Session
	BuildID string
}

func recoverCreatedSession(ctx context.Context, svc *services, opts runStartOptions, dev mobile.DeviceSelection, since time.Time) (browserstack.Session, string, error) {
	if strings.TrimSpace(opts.Build) == "" && strings.TrimSpace(opts.Name) == "" {
		return browserstack.Session{}, "", mobile.NewError("session_recovery_not_possible", "Appium response did not include a session id and no build/session name is available for recovery", "Pass --build and --name so BrowserStack control-plane recovery can uniquely identify the session.", 502)
	}
	builds, err := svc.Control.ListBuilds(ctx, 20, 0, "", "")
	if err != nil {
		return browserstack.Session{}, "", err
	}
	var candidates []recoveredSessionCandidate
	for _, build := range builds {
		if opts.Build != "" && !equalFold(build.Name, opts.Build) {
			continue
		}
		if build.HashedID == "" {
			continue
		}
		sessions, err := svc.Control.ListBuildSessions(ctx, build.HashedID, 100, 0, "")
		if err != nil {
			continue
		}
		for _, session := range sessions {
			if browserStackSessionMatches(session, opts, dev, since) {
				candidates = append(candidates, recoveredSessionCandidate{Session: session, BuildID: build.HashedID})
			}
		}
	}
	if len(candidates) == 1 && candidates[0].Session.HashedID != "" {
		return candidates[0].Session, candidates[0].BuildID, nil
	}
	if len(candidates) > 1 {
		return browserstack.Session{}, "", mobile.NewError("session_recovery_ambiguous", "BrowserStack control-plane recovery found multiple matching sessions", "Use unique --build and --name values, then retry.", 409)
	}
	return browserstack.Session{}, "", mobile.NewError("session_recovery_not_found", "BrowserStack control-plane recovery did not find the created session", "Inspect BrowserStack Appium logs and dashboard for the attempted run.", 502)
}

func browserStackSessionMatches(session browserstack.Session, opts runStartOptions, dev mobile.DeviceSelection, since time.Time) bool {
	if session.HashedID == "" {
		return false
	}
	if opts.Name != "" && !equalFold(session.Name, opts.Name) {
		return false
	}
	if opts.Build != "" && session.BuildName != "" && !equalFold(session.BuildName, opts.Build) {
		return false
	}
	if opts.Platform != "" && session.OS != "" && !equalFold(session.OS, opts.Platform) {
		return false
	}
	if dev.Name != "" && session.Device != "" && !equalFold(session.Device, dev.Name) {
		return false
	}
	if t, ok := sessionCreatedAt(session.Raw); ok && t.Before(since.Add(-5*time.Minute)) {
		return false
	}
	return true
}

func sessionCreatedAt(raw any) (time.Time, bool) {
	m, _ := raw.(map[string]any)
	if m == nil {
		return time.Time{}, false
	}
	for _, key := range []string{"created_at", "createdAt", "start_time", "started_at", "created_time"} {
		if t, ok := parseRemoteTime(m[key]); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseRemoteTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case string:
		x = strings.TrimSpace(x)
		if x == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, x); err == nil {
				return t, true
			}
		}
	case float64:
		if x > 1_000_000_000_000 {
			return time.UnixMilli(int64(x)), true
		}
		if x > 0 {
			return time.Unix(int64(x), 0), true
		}
	}
	return time.Time{}, false
}

func findBuildIDByName(ctx context.Context, svc *services, buildName string, since time.Time) string {
	if strings.TrimSpace(buildName) == "" {
		return ""
	}
	builds, err := svc.Control.ListBuilds(ctx, 20, 0, "", "")
	if err != nil {
		return ""
	}
	matches := []browserstack.Build{}
	for _, build := range builds {
		if equalFold(build.Name, buildName) && build.HashedID != "" && buildRecentEnough(build.Raw, since) {
			matches = append(matches, build)
		}
	}
	if len(matches) == 1 {
		return matches[0].HashedID
	}
	return ""
}

func buildRecentEnough(raw any, since time.Time) bool {
	if t, ok := sessionCreatedAt(raw); ok {
		return !t.Before(since.Add(-5 * time.Minute))
	}
	return true
}

func enrichRunStateFromRemote(st *mobile.RunState, remote browserstack.Session) {
	if remote.HashedID != "" {
		st.BrowserStackSessionID = remote.HashedID
	}
	st.BrowserURL = firstNonEmpty(st.BrowserURL, remote.BrowserURL)
	st.PublicURL = firstNonEmpty(st.PublicURL, remote.PublicURL)
	st.AppiumLogsURL = firstNonEmpty(st.AppiumLogsURL, remote.AppiumLogsURL)
	st.DeviceLogsURL = firstNonEmpty(st.DeviceLogsURL, remote.DeviceLogsURL)
	st.VideoURL = firstNonEmpty(st.VideoURL, remote.VideoURL)
	st.DashboardURL = firstNonEmpty(st.DashboardURL, remote.BrowserURL, remote.PublicURL)
}

func localModeForNetwork(network string) string {
	switch network {
	case "private-managed":
		return "managed"
	case "private-external":
		return "external"
	default:
		return "none"
	}
}

func effectiveRunStart(st mobile.RunState, dev mobile.DeviceResolveResult) map[string]any {
	return map[string]any{
		"network_mode":       st.Network.Mode,
		"local_mode":         st.Network.LocalMode,
		"local_identifier":   st.Network.LocalIdentifier,
		"build_name":         st.BuildName,
		"session_name":       st.SessionName,
		"device_resolution":  dev,
		"browserstack_links": map[string]string{"dashboard_url": st.DashboardURL, "appium_logs_url": st.AppiumLogsURL, "device_logs_url": st.DeviceLogsURL, "video_url": st.VideoURL},
	}
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
		var cachedURL string
		if cached, err := svc.Store.LoadAppCache(sha); err == nil && cached.AppURL != "" {
			cachedURL = cached.AppURL
			if mobile.AppCacheReusable(cached, time.Now().UTC()) {
				return cached, nil
			}
		}
		if ref, ok := findRecentAppRef(ctx, svc, opts.CustomID, sha, cachedURL); ok {
			_ = svc.Store.SaveAppCache(ref)
			return ref, nil
		}
	}
	if opts.CustomID != "" {
		if ref, ok := findRecentAppRef(ctx, svc, opts.CustomID, sha, ""); ok {
			_ = svc.Store.SaveAppCache(ref)
			return ref, nil
		}
	}
	app, err := svc.Control.UploadApp(ctx, browserstack.UploadAppRequest{FilePath: opts.File, URL: opts.URL, CustomID: opts.CustomID, SHA256: sha})
	if err != nil {
		return mobile.AppRef{}, err
	}
	ref := appRefFromUploaded(app, opts.CustomID, sha, app.AppName)
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
		st, _ = reconcileRunStatusBestEffort(cmd.Context(), svc, st)
		return print(cmd, o, output.Success("", st))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	return c
}

type runRecoverOptions struct {
	RunID           string
	SessionID       string
	BuildID         string
	BuildName       string
	SessionName     string
	Network         string
	LocalIdentifier string
	Platform        string
	App             string
}

func runRecoverCmd(o *Opts) *cobra.Command {
	opts := runRecoverOptions{}
	c := &cobra.Command{Use: "recover", RunE: func(cmd *cobra.Command, args []string) error {
		return runRecover(cmd, o, opts)
	}}
	c.Flags().StringVar(&opts.RunID, "run-id", "", "")
	c.Flags().StringVar(&opts.SessionID, "session-id", "", "")
	c.Flags().StringVar(&opts.BuildID, "build-id", "", "")
	c.Flags().StringVar(&opts.BuildName, "build", "", "")
	c.Flags().StringVar(&opts.SessionName, "name", "", "")
	c.Flags().StringVar(&opts.Network, "network", "", "")
	c.Flags().StringVar(&opts.LocalIdentifier, "local-identifier", "", "")
	c.Flags().StringVar(&opts.Platform, "platform", "", "")
	c.Flags().StringVar(&opts.App, "app", "", "")
	return c
}

func runRecover(cmd *cobra.Command, o *Opts, opts runRecoverOptions) error {
	if strings.TrimSpace(opts.SessionID) == "" && strings.TrimSpace(opts.BuildID) == "" && strings.TrimSpace(opts.BuildName) == "" {
		return print(cmd, o, output.Failure("invalid_args", "--session-id, --build-id, or --build is required", "Pass a BrowserStack session or build identifier to attach local run state.", 400))
	}
	svc, err := newServices(o, true)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	remote, buildID, err := recoverRemoteSession(cmd.Context(), svc, opts)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	if opts.RunID == "" {
		opts.RunID = mobile.NewRunID()
	}
	network := opts.Network
	if network == "" {
		if opts.LocalIdentifier != "" {
			network = "private-external"
		} else {
			network = "public"
		}
	}
	if network != "public" && network != "private-managed" && network != "private-external" {
		return print(cmd, o, output.Failure("invalid_args", "--network must be public, private-managed, or private-external", "Use private-external when attaching to a pre-running BrowserStack Local tunnel.", 400))
	}
	sessionID := firstNonEmpty(remote.HashedID, opts.SessionID)
	now := time.Now().UTC()
	st := mobile.RunState{
		Version:               1,
		RunID:                 opts.RunID,
		Provider:              "browserstack",
		Status:                mobile.StatusRunning,
		ControlOwner:          "agent",
		SessionID:             sessionID,
		BrowserStackSessionID: sessionID,
		Platform:              firstNonEmpty(opts.Platform, remote.OS),
		Device:                remoteDeviceSelection(remote),
		App:                   remoteAppRef(remote, opts.App),
		Network:               mobile.NetworkState{Mode: network, LocalMode: localModeForNetwork(network), LocalIdentifier: opts.LocalIdentifier},
		BuildID:               buildID,
		ProjectName:           remote.ProjectName,
		BuildName:             firstNonEmpty(remote.BuildName, opts.BuildName),
		SessionName:           firstNonEmpty(remote.Name, opts.SessionName),
		StartedAt:             now,
		UpdatedAt:             now,
	}
	enrichRunStateFromRemote(&st, remote)
	if status, terminal := runStatusForRemoteSession(remote.Status); terminal {
		st.Status = status
		st.FinishedAt = &now
	}
	if err := svc.Store.SaveRun(st); err != nil {
		return renderErr(cmd, o, err)
	}
	return print(cmd, o, output.Success("", map[string]any{"run": st, "recovered": true, "remote_status": remote.Status}))
}

func recoverRemoteSession(ctx context.Context, svc *services, opts runRecoverOptions) (browserstack.Session, string, error) {
	if opts.SessionID != "" {
		remote, err := svc.Control.GetSession(ctx, opts.SessionID)
		if err != nil {
			return browserstack.Session{}, "", err
		}
		buildID := opts.BuildID
		if buildID == "" {
			buildID = findBuildIDByName(ctx, svc, firstNonEmpty(remote.BuildName, opts.BuildName), time.Now().UTC().Add(-24*time.Hour))
		}
		return remote, buildID, nil
	}
	buildID := opts.BuildID
	if buildID == "" {
		builds, err := svc.Control.ListBuilds(ctx, 20, 0, "", "")
		if err != nil {
			return browserstack.Session{}, "", err
		}
		matches := []browserstack.Build{}
		for _, build := range builds {
			if equalFold(build.Name, opts.BuildName) && build.HashedID != "" {
				matches = append(matches, build)
			}
		}
		if len(matches) != 1 {
			return browserstack.Session{}, "", mobile.NewError("session_recovery_ambiguous", "BrowserStack build recovery did not find exactly one matching build", "Pass --build-id for the build you want to attach.", 409)
		}
		buildID = matches[0].HashedID
	}
	sessions, err := svc.Control.ListBuildSessions(ctx, buildID, 100, 0, "")
	if err != nil {
		return browserstack.Session{}, "", err
	}
	matches := []browserstack.Session{}
	for _, session := range sessions {
		if opts.SessionName != "" && !equalFold(session.Name, opts.SessionName) {
			continue
		}
		matches = append(matches, session)
	}
	if len(matches) != 1 || matches[0].HashedID == "" {
		return browserstack.Session{}, "", mobile.NewError("session_recovery_ambiguous", "BrowserStack session recovery did not find exactly one matching session", "Pass --session-id or add --name to uniquely identify the session.", 409)
	}
	return matches[0], buildID, nil
}

func remoteDeviceSelection(remote browserstack.Session) mobile.DeviceSelection {
	return mobile.DeviceSelection{Name: remote.Device, OS: remote.OS, OSVersion: remote.OSVersion, Reason: "recovered"}
}

func remoteAppRef(remote browserstack.Session, app string) mobile.AppRef {
	ref := mobile.AppRef{AppURL: app}
	if remote.AppDetails == nil {
		return ref
	}
	ref.AppURL = firstNonEmpty(ref.AppURL, stringMapValue(remote.AppDetails, "app_url"), stringMapValue(remote.AppDetails, "app"))
	ref.Name = firstNonEmpty(stringMapValue(remote.AppDetails, "app_name"), stringMapValue(remote.AppDetails, "name"))
	ref.CustomID = stringMapValue(remote.AppDetails, "custom_id")
	return ref
}

func stringMapValue(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func reconcileRunStatusBestEffort(ctx context.Context, svc *services, st mobile.RunState) (mobile.RunState, []string) {
	if st.SessionID == "" || !localRunStatusMayBeRemoteActive(st.Status) {
		return st, nil
	}
	if !svc.Runtime.Username || !svc.Runtime.AccessKey {
		return st, nil
	}
	reconcileCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	remote, err := svc.Control.GetSession(reconcileCtx, st.SessionID)
	if err != nil {
		if isRemoteSessionGone(err) {
			markRunLost(&st)
			_ = svc.Store.SaveRun(st)
		}
		return st, []string{err.Error()}
	}
	enrichRunStateFromRemote(&st, remote)
	if status, terminal := runStatusForRemoteSession(remote.Status); terminal {
		st.Status = status
		now := time.Now().UTC()
		st.FinishedAt = &now
		st.ControlOwner = "agent"
		st.LatestObservationID = ""
	}
	_ = svc.Store.SaveRun(st)
	return st, nil
}

func localRunStatusMayBeRemoteActive(status mobile.RunStatus) bool {
	return status == mobile.StatusStarting || status == mobile.StatusRunning || status == mobile.StatusWaitingForHuman || status == mobile.StatusResuming
}

func runStatusForRemoteSession(status string) (mobile.RunStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "running", "queued", "created":
		return "", false
	case "passed", "completed", "done":
		return mobile.StatusFinished, true
	case "failed", "error", "errored":
		return mobile.StatusFailed, true
	case "timeout", "timed_out", "stopped", "aborted":
		return mobile.StatusLost, true
	default:
		return "", false
	}
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
			keeper, err := startKeeper(o, svc, st.RunID, deadline)
			if err != nil {
				st.ControlOwner = "agent"
				st.Status = mobile.StatusRunning
				_ = svc.Store.SaveRun(st)
				return mobile.NewError("keepalive_start_failed", "failed to start keepalive helper", "Retry handoff or finish the run to avoid idle timeout.", 500)
			}
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
