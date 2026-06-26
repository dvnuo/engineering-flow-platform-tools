package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/mobileauto"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func assertCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "assert"}
	c.AddCommand(
		assertExistsCmd(o), assertNotExistsCmd(o), assertVisibleCmd(o), assertNotVisibleCmd(o),
		assertEnabledCmd(o), assertSelectedCmd(o), assertTextCmd(o), assertCountCmd(o),
	)
	return c
}

func assertExistsCmd(o *Opts) *cobra.Command {
	var runID, ref, name, role string
	c := &cobra.Command{Use: "exists", RunE: func(cmd *cobra.Command, args []string) error {
		return runAssertion(cmd, o, runID, ref, name, role, func(ctx context.Context, svc *services, st mobileauto.RunState, elementID string) (bool, any, error) {
			return true, map[string]any{"element_id": elementID}, nil
		})
	}}
	bindAssertFlags(c, &runID, &ref, &name, &role)
	return c
}

func assertNotExistsCmd(o *Opts) *cobra.Command {
	var runID string
	q := mobileauto.LocateQuery{}
	c := &cobra.Command{Use: "not-exists", RunE: func(cmd *cobra.Command, args []string) error {
		return runObservationAssertion(cmd, o, runID, q, func(obs mobileauto.Observation, res mobileauto.LocateResult) (bool, any) {
			return len(res.Matches) == 0, map[string]any{"matches": len(res.Matches), "locate": res, "observation_id": obs.ID}
		})
	}}
	bindLocateAssertionFlags(c, &runID, &q)
	return c
}

func assertVisibleCmd(o *Opts) *cobra.Command {
	var runID, ref, name, role string
	c := &cobra.Command{Use: "visible", RunE: func(cmd *cobra.Command, args []string) error {
		return runAssertion(cmd, o, runID, ref, name, role, func(ctx context.Context, svc *services, st mobileauto.RunState, elementID string) (bool, any, error) {
			ok, err := svc.Appium.ElementDisplayed(ctx, st.SessionID, elementID)
			return ok, map[string]any{"visible": ok}, err
		})
	}}
	bindAssertFlags(c, &runID, &ref, &name, &role)
	return c
}

func assertNotVisibleCmd(o *Opts) *cobra.Command {
	var runID, ref, name, role string
	c := &cobra.Command{Use: "not-visible", RunE: func(cmd *cobra.Command, args []string) error {
		if ref != "" {
			return runAssertion(cmd, o, runID, ref, name, role, func(ctx context.Context, svc *services, st mobileauto.RunState, elementID string) (bool, any, error) {
				ok, err := svc.Appium.ElementDisplayed(ctx, st.SessionID, elementID)
				return !ok, map[string]any{"visible": ok}, err
			})
		}
		q := mobileauto.LocateQuery{Name: name, Role: role, Visible: boolPtr(true), Limit: 1000}
		return runObservationAssertion(cmd, o, runID, q, func(obs mobileauto.Observation, res mobileauto.LocateResult) (bool, any) {
			return len(res.Matches) == 0, map[string]any{"visible_matches": len(res.Matches), "locate": res, "observation_id": obs.ID}
		})
	}}
	bindAssertFlags(c, &runID, &ref, &name, &role)
	return c
}

func assertEnabledCmd(o *Opts) *cobra.Command {
	var runID, ref, name, role string
	c := &cobra.Command{Use: "enabled", RunE: func(cmd *cobra.Command, args []string) error {
		return runAssertion(cmd, o, runID, ref, name, role, func(ctx context.Context, svc *services, st mobileauto.RunState, elementID string) (bool, any, error) {
			ok, err := svc.Appium.ElementEnabled(ctx, st.SessionID, elementID)
			return ok, map[string]any{"enabled": ok}, err
		})
	}}
	bindAssertFlags(c, &runID, &ref, &name, &role)
	return c
}

func assertSelectedCmd(o *Opts) *cobra.Command {
	var runID, ref, name, role string
	c := &cobra.Command{Use: "selected", RunE: func(cmd *cobra.Command, args []string) error {
		return runAssertion(cmd, o, runID, ref, name, role, func(ctx context.Context, svc *services, st mobileauto.RunState, elementID string) (bool, any, error) {
			ok, err := svc.Appium.ElementSelected(ctx, st.SessionID, elementID)
			return ok, map[string]any{"selected": ok}, err
		})
	}}
	bindAssertFlags(c, &runID, &ref, &name, &role)
	return c
}

func assertTextCmd(o *Opts) *cobra.Command {
	var runID, ref, name, role, equals, contains string
	c := &cobra.Command{Use: "text", RunE: func(cmd *cobra.Command, args []string) error {
		if (equals == "") == (contains == "") {
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --equals or --contains is required", "Pass a text assertion mode.", 400))
		}
		return runAssertion(cmd, o, runID, ref, name, role, func(ctx context.Context, svc *services, st mobileauto.RunState, elementID string) (bool, any, error) {
			text, err := svc.Appium.ElementText(ctx, st.SessionID, elementID)
			if err != nil {
				return false, nil, err
			}
			ok := false
			mode := "equals"
			expected := equals
			if equals != "" {
				ok = text == equals
			} else {
				mode = "contains"
				expected = contains
				ok = strings.Contains(text, contains)
			}
			return ok, map[string]any{"mode": mode, "expected": expected, "actual": text}, nil
		})
	}}
	bindAssertFlags(c, &runID, &ref, &name, &role)
	c.Flags().StringVar(&equals, "equals", "", "")
	c.Flags().StringVar(&contains, "contains", "", "")
	return c
}

func assertCountCmd(o *Opts) *cobra.Command {
	var runID string
	var expected int
	q := mobileauto.LocateQuery{Limit: 1000}
	c := &cobra.Command{Use: "count", RunE: func(cmd *cobra.Command, args []string) error {
		if expected < 0 {
			return print(cmd, o, output.Failure("invalid_args", "--expected cannot be negative", "Pass the expected match count.", 400))
		}
		return runObservationAssertion(cmd, o, runID, q, func(obs mobileauto.Observation, res mobileauto.LocateResult) (bool, any) {
			return len(res.Matches) == expected, map[string]any{"expected": expected, "actual": len(res.Matches), "locate": res, "observation_id": obs.ID}
		})
	}}
	bindLocateAssertionFlags(c, &runID, &q)
	c.Flags().IntVar(&expected, "expected", -1, "")
	return c
}

func bindAssertFlags(c *cobra.Command, runID, ref, name, role *string) {
	c.Flags().StringVar(runID, "run-id", "", "")
	c.Flags().StringVar(ref, "ref", "", "")
	c.Flags().StringVar(name, "name", "", "")
	c.Flags().StringVar(role, "role", "", "")
}

func bindLocateAssertionFlags(c *cobra.Command, runID *string, q *mobileauto.LocateQuery) {
	c.Flags().StringVar(runID, "run-id", "", "")
	c.Flags().StringVar(&q.Name, "name", "", "")
	c.Flags().StringVar(&q.Text, "text", "", "")
	c.Flags().StringVar(&q.Role, "role", "", "")
	c.Flags().StringVar(&q.ResourceID, "resource-id", "", "")
	c.Flags().StringVar(&q.AccessibilityID, "accessibility-id", "", "")
	c.Flags().StringVar(&q.ParentText, "parent-text", "", "")
	c.Flags().StringVar(&q.NearbyText, "nearby-text", "", "")
	c.Flags().StringVar(&q.WithinText, "within-text", "", "")
}

func runObservationAssertion(cmd *cobra.Command, o *Opts, runID string, q mobileauto.LocateQuery, fn func(mobileauto.Observation, mobileauto.LocateResult) (bool, any)) error {
	if runID == "" {
		return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
	}
	if !locateQueryHasCriteria(q) {
		return print(cmd, o, output.Failure("invalid_args", "at least one locate criterion is required", "Pass --name, --text, --role, --resource-id, --accessibility-id, or context text.", 400))
	}
	svc, err := newServices(o, false)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	st, err := svc.Store.LoadRun(runID)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	if st.LatestObservationID == "" {
		return print(cmd, o, output.Failure("stale_observation", "no current observation is available", "Run mobile-auto observe first.", 409))
	}
	obs, err := svc.Store.LoadObservation(runID, st.LatestObservationID)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	res := mobileauto.Locate(obs, q)
	ok, evidence := fn(obs, res)
	data := map[string]any{"passed": ok, "run_id": runID, "evidence": evidence}
	appendTimelineBestEffort(svc, runID, "assert", cmd.CalledAs(), obs.ID, st.Status, data)
	if !ok {
		env := output.Failure("assertion_failed", "mobile-auto assertion failed", "Inspect evidence and observe the page again if needed.", 412)
		env.Data = data
		return print(cmd, o, env)
	}
	return print(cmd, o, output.Success("", data))
}

func runAssertion(cmd *cobra.Command, o *Opts, runID, ref, name, role string, fn func(context.Context, *services, mobileauto.RunState, string) (bool, any, error)) error {
	if runID == "" {
		return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
	}
	svc, err := newServices(o, true)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	st, err := svc.Store.LoadRun(runID)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	var elementID string
	var locator any
	if ref != "" {
		_, _, _, element, loc, err := resolveRefElement(cmd.Context(), svc, runID, ref)
		if err != nil {
			markRunLostIfSessionGone(svc, &st, err)
			return renderErr(cmd, o, err)
		}
		elementID = element.ID
		locator = loc
	} else {
		if st.LatestObservationID == "" {
			return print(cmd, o, output.Failure("stale_observation", "no current observation is available", "Run mobile-auto observe first or pass --ref.", 409))
		}
		obs, err := svc.Store.LoadObservation(runID, st.LatestObservationID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		res := mobileauto.Locate(obs, mobileauto.LocateQuery{Name: name, Role: role, Actionable: false, Limit: 2})
		if res.RecommendedRef == "" {
			return print(cmd, o, output.Failure("ambiguous_element", "semantic assertion target was not unique", "Use mobile-auto locate, then assert with --ref.", 409))
		}
		_, _, _, element, loc, err := resolveRefElement(cmd.Context(), svc, runID, res.RecommendedRef)
		if err != nil {
			markRunLostIfSessionGone(svc, &st, err)
			return renderErr(cmd, o, err)
		}
		elementID = element.ID
		locator = loc
		ref = res.RecommendedRef
	}
	ok, evidence, err := fn(cmd.Context(), svc, st, elementID)
	if err != nil {
		markRunLostIfSessionGone(svc, &st, err)
		return renderErr(cmd, o, err)
	}
	data := map[string]any{"passed": ok, "run_id": runID, "ref": ref, "locator": locator, "evidence": evidence}
	appendTimelineBestEffort(svc, runID, "assert", cmd.CalledAs(), "", st.Status, data)
	if !ok {
		env := output.Failure("assertion_failed", "mobile-auto assertion failed", "Inspect evidence and observe the page again if needed.", 412)
		env.Data = data
		return print(cmd, o, env)
	}
	return print(cmd, o, output.Success("", data))
}

func waitCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "wait"}
	var runID, timeoutText, pollText string
	var stableCount int
	stable := &cobra.Command{Use: "stable", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		timeout, err := time.ParseDuration(timeoutText)
		if err != nil || timeout <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --timeout", "Use a duration such as 30s.", 400))
		}
		poll, err := time.ParseDuration(pollText)
		if err != nil || poll <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --poll-interval", "Use a duration such as 1s.", 400))
		}
		if stableCount <= 0 {
			stableCount = 2
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		t := time.NewTicker(poll)
		defer t.Stop()
		var lastHash string
		same := 0
		var latest mobileauto.Observation
		for {
			err := svc.Store.WithRunLock(runID, func() error {
				st, err := svc.Store.LoadRun(runID)
				if err != nil {
					return err
				}
				latest, err = captureObservation(ctx, svc, &st, 100)
				return err
			})
			if err != nil {
				return renderErr(cmd, o, err)
			}
			if latest.SourceHash == lastHash {
				same++
			} else {
				same = 1
				lastHash = latest.SourceHash
			}
			if same >= stableCount {
				data := map[string]any{"stable": true, "observation": latest, "stable_count": same}
				appendTimelineBestEffort(svc, runID, "wait", "stable", latest.ID, mobileauto.StatusRunning, data)
				return print(cmd, o, output.Success("", data))
			}
			select {
			case <-ctx.Done():
				return renderErr(cmd, o, mobileauto.RetryableError("assertion_failed", "page did not become stable before timeout", "Retry or inspect observations.", "observe", 408))
			case <-t.C:
			}
		}
	}}
	stable.Flags().StringVar(&runID, "run-id", "", "")
	stable.Flags().StringVar(&timeoutText, "timeout", "30s", "")
	stable.Flags().StringVar(&pollText, "poll-interval", "1s", "")
	stable.Flags().IntVar(&stableCount, "stable-count", 2, "")
	c.AddCommand(stable, waitVisibleCmd(o), waitGoneCmd(o), waitTextCmd(o), waitEnabledCmd(o))
	return c
}

func waitVisibleCmd(o *Opts) *cobra.Command {
	return waitLocateCmd(o, "visible", func(q *mobileauto.LocateQuery) {
		q.Visible = boolPtr(true)
	})
}

func waitGoneCmd(o *Opts) *cobra.Command {
	return waitLocateCmd(o, "gone", func(q *mobileauto.LocateQuery) {
		q.Visible = boolPtr(true)
	})
}

func waitTextCmd(o *Opts) *cobra.Command {
	return waitLocateCmd(o, "text", func(q *mobileauto.LocateQuery) {
		q.Visible = boolPtr(true)
	})
}

func waitEnabledCmd(o *Opts) *cobra.Command {
	return waitLocateCmd(o, "enabled", func(q *mobileauto.LocateQuery) {
		q.Enabled = boolPtr(true)
	})
}

func waitLocateCmd(o *Opts, mode string, configure func(*mobileauto.LocateQuery)) *cobra.Command {
	var runID, timeoutText, pollText string
	q := mobileauto.LocateQuery{Limit: 1000}
	c := &cobra.Command{Use: mode, RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		if !locateQueryHasCriteria(q) {
			return print(cmd, o, output.Failure("invalid_args", "at least one locate criterion is required", "Pass --name, --text, --role, --resource-id, --accessibility-id, or context text.", 400))
		}
		timeout, err := time.ParseDuration(timeoutText)
		if err != nil || timeout <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --timeout", "Use a duration such as 30s.", 400))
		}
		poll, err := time.ParseDuration(pollText)
		if err != nil || poll <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --poll-interval", "Use a duration such as 1s.", 400))
		}
		configure(&q)
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		t := time.NewTicker(poll)
		defer t.Stop()
		attempts := 0
		for {
			attempts++
			var obs mobileauto.Observation
			err := svc.Store.WithRunLock(runID, func() error {
				st, err := svc.Store.LoadRun(runID)
				if err != nil {
					return err
				}
				obs, err = captureObservation(ctx, svc, &st, 100)
				return err
			})
			if err != nil {
				return renderErr(cmd, o, err)
			}
			res := mobileauto.Locate(obs, q)
			matched := len(res.Matches) > 0
			if mode == "gone" {
				matched = !matched
			}
			if matched {
				data := map[string]any{"matched": true, "mode": mode, "attempts": attempts, "locate": res, "observation": obs}
				appendTimelineBestEffort(svc, runID, "wait", mode, obs.ID, mobileauto.StatusRunning, data)
				return print(cmd, o, output.Success("", data))
			}
			select {
			case <-ctx.Done():
				return renderErr(cmd, o, mobileauto.RetryableError("assertion_failed", "wait condition was not satisfied before timeout", "Retry or inspect observations.", "observe", 408))
			case <-t.C:
			}
		}
	}}
	bindLocateAssertionFlags(c, &runID, &q)
	c.Flags().StringVar(&timeoutText, "timeout", "30s", "")
	c.Flags().StringVar(&pollText, "poll-interval", "1s", "")
	return c
}

func artifactCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "artifact"}
	c.AddCommand(artifactListCmd(o), artifactCollectCmd(o), artifactDownloadCmd(o))
	return c
}

func artifactListCmd(o *Opts) *cobra.Command {
	var runID string
	c := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass a run id.", 400))
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		files, err := listRunArtifacts(svc, runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"files": files, "count": len(files), "sensitive": true}))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	return c
}

func artifactCollectCmd(o *Opts) *cobra.Command {
	var runID, outDir string
	c := &cobra.Command{Use: "collect", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass a run id.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err := svc.Store.LoadRun(runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		bundle, err := collectArtifacts(cmd.Context(), svc, st, outDir)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", bundle))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&outDir, "out", "", "")
	return c
}

func artifactDownloadCmd(o *Opts) *cobra.Command {
	var buildID, sessionID, kind, outDir string
	c := &cobra.Command{Use: "download", RunE: func(cmd *cobra.Command, args []string) error {
		if buildID == "" || sessionID == "" || kind == "" {
			return print(cmd, o, output.Failure("invalid_args", "--build-id, --session-id, and --kind are required", "Use a supported BrowserStack artifact kind.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		b, contentType, err := svc.Control.DownloadArtifact(cmd.Context(), buildID, sessionID, kind)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if outDir == "" {
			outDir = svc.Runtime.Mobile.ArtifactsDir
		}
		art, err := mobileauto.WriteArtifact(outDir, kind, sessionID+".log", b, contentType)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", art))
	}}
	c.Flags().StringVar(&buildID, "build-id", "", "")
	c.Flags().StringVar(&sessionID, "session-id", "", "")
	c.Flags().StringVar(&kind, "kind", "", "")
	c.Flags().StringVar(&outDir, "out", "", "")
	return c
}

func collectArtifacts(ctx context.Context, svc *services, st mobileauto.RunState, outDir string) (map[string]any, error) {
	if outDir == "" {
		outDir = filepath.Join(svc.Store.RunDir(st.RunID), "artifacts")
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return nil, err
	}
	files, _ := listRunArtifacts(svc, st.RunID)
	var remote []mobileauto.FileArtifact
	var warnings []string
	if st.BuildID != "" && st.SessionID != "" {
		for _, kind := range []string{"appiumlogs", "devicelogs", "crashlogs", "networklogs"} {
			b, contentType, err := svc.Control.DownloadArtifact(ctx, st.BuildID, st.SessionID, kind)
			if err != nil {
				warnings = append(warnings, kind+": "+err.Error())
				continue
			}
			art, err := mobileauto.WriteArtifact(outDir, kind, st.SessionID+".log", b, contentType)
			if err != nil {
				warnings = append(warnings, kind+": "+err.Error())
				continue
			}
			remote = append(remote, art)
		}
	}
	return map[string]any{"run_id": st.RunID, "out_dir": outDir, "local_files": files, "remote_files": remote, "warnings": warnings, "sensitive": true}, nil
}

func listRunArtifacts(svc *services, runID string) ([]mobileauto.FileArtifact, error) {
	root := svc.Store.RunDir(runID)
	var files []mobileauto.FileArtifact
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		if name == "lock" {
			return nil
		}
		kind := "local"
		switch {
		case strings.Contains(path, "observations"):
			kind = "observation"
		case strings.Contains(path, "artifacts"):
			kind = "artifact"
		case strings.HasSuffix(name, ".log"):
			kind = "log"
		}
		art, err := mobileauto.InspectArtifact(kind, path, "")
		if err == nil {
			files = append(files, art)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
