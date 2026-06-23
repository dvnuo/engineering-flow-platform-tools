package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/appium"
	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func observeCmd(o *Opts) *cobra.Command {
	var runID string
	var limit int
	c := &cobra.Command{Use: "observe", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var obs mobile.Observation
		err = svc.Store.WithRunLock(runID, func() error {
			st, err := svc.Store.LoadRun(runID)
			if err != nil {
				return err
			}
			obs, err = captureObservation(cmd.Context(), svc, &st, limit)
			return err
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", obs))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().IntVar(&limit, "limit", 100, "")
	return c
}

func captureObservation(ctx context.Context, svc *services, st *mobile.RunState, limit int) (mobile.Observation, error) {
	source, err := svc.Appium.GetSource(ctx, st.SessionID)
	if err != nil {
		return mobile.Observation{}, err
	}
	screen, err := svc.Appium.Screenshot(ctx, st.SessionID)
	if err != nil {
		return mobile.Observation{}, err
	}
	st.ObservationVersion++
	obsID := mobile.NewObservationID(st.ObservationVersion)
	obs := mobile.BuildObservation(st.RunID, st.SessionID, obsID, source, screen, limit)
	dir := filepath.Join(svc.Store.ObservationDir(st.RunID), obs.ID)
	obs.SourcePath = filepath.Join(dir, "source.xml")
	obs.ScreenshotPath = filepath.Join(dir, "screenshot.png")
	obs.CandidatesPath = filepath.Join(dir, "candidates.json")
	if err := svc.Store.SaveObservation(st.RunID, obs); err != nil {
		return mobile.Observation{}, err
	}
	st.LatestObservationID = obs.ID
	if err := svc.Store.SaveRun(*st); err != nil {
		return mobile.Observation{}, err
	}
	return obs, nil
}

func locateCmd(o *Opts) *cobra.Command {
	var runID string
	q := mobile.LocateQuery{}
	var visible, enabled bool
	var useVisible, useEnabled bool
	c := &cobra.Command{Use: "locate", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
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
			return print(cmd, o, output.Failure("stale_observation", "no current observation is available", "Run mobile observe --run-id ... --json first.", 409))
		}
		obs, err := svc.Store.LoadObservation(runID, st.LatestObservationID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if useVisible {
			q.Visible = &visible
		}
		if useEnabled {
			q.Enabled = &enabled
		}
		res := mobile.Locate(obs, q)
		return print(cmd, o, output.Success("", res))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&q.Name, "name", "", "")
	c.Flags().StringVar(&q.Text, "text", "", "")
	c.Flags().StringVar(&q.Role, "role", "", "")
	c.Flags().StringVar(&q.ResourceID, "resource-id", "", "")
	c.Flags().StringVar(&q.AccessibilityID, "accessibility-id", "", "")
	c.Flags().StringVar(&q.ParentText, "parent-text", "", "")
	c.Flags().StringVar(&q.NearbyText, "nearby-text", "", "")
	c.Flags().BoolVar(&q.Actionable, "actionable", false, "")
	c.Flags().BoolVar(&visible, "visible", true, "")
	c.Flags().BoolVar(&enabled, "enabled", true, "")
	c.Flags().BoolVar(&useVisible, "require-visible", true, "")
	c.Flags().BoolVar(&useEnabled, "require-enabled", false, "")
	c.Flags().IntVar(&q.Limit, "limit", 10, "")
	return c
}

func tapCmd(o *Opts) *cobra.Command {
	var runID, ref string
	c := &cobra.Command{Use: "tap", RunE: func(cmd *cobra.Command, args []string) error {
		return mutateRef(cmd, o, runID, ref, "tap", func(ctx context.Context, svc *services, st mobile.RunState, element appium.RemoteElement) error {
			return svc.Appium.Click(ctx, st.SessionID, element.ID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&ref, "ref", "", "")
	return c
}

func clearCmd(o *Opts) *cobra.Command {
	var runID, ref string
	c := &cobra.Command{Use: "clear", RunE: func(cmd *cobra.Command, args []string) error {
		return mutateRef(cmd, o, runID, ref, "clear", func(ctx context.Context, svc *services, st mobile.RunState, element appium.RemoteElement) error {
			return svc.Appium.Clear(ctx, st.SessionID, element.ID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&ref, "ref", "", "")
	return c
}

func typeCmd(o *Opts) *cobra.Command {
	var runID, ref, text, textEnv string
	var textStdin bool
	c := &cobra.Command{Use: "type", RunE: func(cmd *cobra.Command, args []string) error {
		value, source, err := readTextValue(cmd, text, textEnv, textStdin)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Use exactly one of --text, --text-env, or --text-stdin.", 400))
		}
		var typed int
		err = mutateRefCore(cmd, o, runID, ref, "type", func(ctx context.Context, svc *services, st mobile.RunState, element appium.RemoteElement) (map[string]any, error) {
			typed = len(value)
			return map[string]any{"text_source": source, "text_length": typed}, svc.Appium.SendKeys(ctx, st.SessionID, element.ID, value)
		})
		return err
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&ref, "ref", "", "")
	c.Flags().StringVar(&text, "text", "", "")
	c.Flags().StringVar(&textEnv, "text-env", "", "")
	c.Flags().BoolVar(&textStdin, "text-stdin", false, "")
	return c
}

func readTextValue(cmd *cobra.Command, text, textEnv string, textStdin bool) (string, string, error) {
	count := 0
	if text != "" {
		count++
	}
	if textEnv != "" {
		count++
	}
	if textStdin {
		count++
	}
	if count != 1 {
		return "", "", fmt.Errorf("exactly one text source is required")
	}
	switch {
	case text != "":
		return text, "flag", nil
	case textEnv != "":
		return os.Getenv(textEnv), "env", nil
	default:
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(b), "\r\n"), "stdin", nil
	}
}

func mutateRef(cmd *cobra.Command, o *Opts, runID, ref, action string, fn func(context.Context, *services, mobile.RunState, appium.RemoteElement) error) error {
	return mutateRefCore(cmd, o, runID, ref, action, func(ctx context.Context, svc *services, st mobile.RunState, element appium.RemoteElement) (map[string]any, error) {
		return nil, fn(ctx, svc, st, element)
	})
}

func mutateRefCore(cmd *cobra.Command, o *Opts, runID, ref, action string, fn func(context.Context, *services, mobile.RunState, appium.RemoteElement) (map[string]any, error)) error {
	if runID == "" || ref == "" {
		return print(cmd, o, output.Failure("invalid_args", "--run-id and --ref are required", "Use a ref from the latest observation.", 400))
	}
	svc, err := newServices(o, true)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	var data map[string]any
	err = svc.Store.WithRunLock(runID, func() error {
		st, obs, candidate, element, locator, err := resolveRefElement(cmd.Context(), svc, runID, ref)
		if err != nil {
			return err
		}
		extra, err := fn(cmd.Context(), svc, st, element)
		if err != nil {
			return err
		}
		st.LatestObservationID = ""
		if err := svc.Store.SaveRun(st); err != nil {
			return err
		}
		data = map[string]any{"action": action, "run_id": runID, "ref": ref, "observation_id": obs.ID, "candidate_id": candidate.CandidateID, "locator": locator, "observation_invalidated": true}
		for k, v := range extra {
			data[k] = v
		}
		return nil
	})
	if err != nil {
		return renderErr(cmd, o, err)
	}
	return print(cmd, o, output.Success("", data))
}

func resolveRefElement(ctx context.Context, svc *services, runID, ref string) (mobile.RunState, mobile.Observation, mobile.Candidate, appium.RemoteElement, appium.Locator, error) {
	st, err := svc.Store.LoadRun(runID)
	if err != nil {
		return st, mobile.Observation{}, mobile.Candidate{}, appium.RemoteElement{}, appium.Locator{}, err
	}
	if st.ControlOwner == "human" {
		return st, mobile.Observation{}, mobile.Candidate{}, appium.RemoteElement{}, appium.Locator{}, mobile.NewError("control_locked", "run control belongs to the human", "Run mobile run resume before mutating actions.", 423)
	}
	if st.LatestObservationID == "" || mobile.RefObservationID(ref) != st.LatestObservationID {
		return st, mobile.Observation{}, mobile.Candidate{}, appium.RemoteElement{}, appium.Locator{}, mobile.RetryableError("stale_observation", "ref does not belong to the latest observation", "Run mobile observe again and use a fresh ref.", "observe", 409)
	}
	obs, err := svc.Store.LoadObservation(runID, st.LatestObservationID)
	if err != nil {
		return st, obs, mobile.Candidate{}, appium.RemoteElement{}, appium.Locator{}, err
	}
	candidate, ok := mobile.CandidateByRef(obs, ref)
	if !ok {
		return st, obs, mobile.Candidate{}, appium.RemoteElement{}, appium.Locator{}, mobile.NewError("element_not_found", "ref was not found in the observation", "Run mobile observe again.", 404)
	}
	var last appium.Locator
	for _, hint := range mobile.LocatorsForCandidate(st.Platform, candidate) {
		locator := appium.Locator{Using: hint.Using, Value: hint.Value}
		last = locator
		elements, err := svc.Appium.FindElements(ctx, st.SessionID, locator)
		if err != nil {
			return st, obs, candidate, appium.RemoteElement{}, locator, err
		}
		if len(elements) == 1 {
			return st, obs, candidate, elements[0], locator, nil
		}
		if len(elements) > 1 {
			return st, obs, candidate, appium.RemoteElement{}, locator, mobile.NewError("ambiguous_element", "locator matched multiple elements", "Observe again or locate with more specific semantic criteria.", 409)
		}
	}
	return st, obs, candidate, appium.RemoteElement{}, last, mobile.RetryableError("element_not_found", "no generated locator matched the element", "Run mobile observe again or use locate with stable criteria.", "observe", 404)
}

func scrollCmd(o *Opts) *cobra.Command {
	var runID, direction string
	var duration int
	c := &cobra.Command{Use: "scroll", RunE: func(cmd *cobra.Command, args []string) error {
		return swipeLike(cmd, o, runID, direction, duration, "scroll")
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&direction, "direction", "down", "")
	c.Flags().IntVar(&duration, "duration-ms", 500, "")
	return c
}

func swipeCmd(o *Opts) *cobra.Command {
	var runID, direction string
	var duration int
	c := &cobra.Command{Use: "swipe", RunE: func(cmd *cobra.Command, args []string) error {
		return swipeLike(cmd, o, runID, direction, duration, "swipe")
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&direction, "direction", "up", "")
	c.Flags().IntVar(&duration, "duration-ms", 500, "")
	return c
}

func swipeLike(cmd *cobra.Command, o *Opts, runID, direction string, duration int, action string) error {
	if runID == "" {
		return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
	}
	svc, err := newServices(o, true)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	err = svc.Store.WithRunLock(runID, func() error {
		st, err := svc.Store.LoadRun(runID)
		if err != nil {
			return err
		}
		if st.ControlOwner == "human" {
			return mobile.NewError("control_locked", "run control belongs to the human", "Run mobile run resume before mutating actions.", 423)
		}
		actions, err := swipeActions(direction, duration)
		if err != nil {
			return err
		}
		if err := svc.Appium.PerformActions(cmd.Context(), st.SessionID, actions); err != nil {
			return err
		}
		st.LatestObservationID = ""
		return svc.Store.SaveRun(st)
	})
	if err != nil {
		return renderErr(cmd, o, err)
	}
	return print(cmd, o, output.Success("", map[string]any{"action": action, "direction": direction, "observation_invalidated": true}))
}

func swipeActions(direction string, duration int) (appium.ActionsRequest, error) {
	if duration <= 0 {
		duration = 500
	}
	startX, startY, endX, endY := 500, 500, 500, 500
	switch strings.ToLower(direction) {
	case "up":
		startY, endY = 700, 300
	case "down":
		startY, endY = 300, 700
	case "left":
		startX, endX = 800, 200
	case "right":
		startX, endX = 200, 800
	default:
		return appium.ActionsRequest{}, mobile.NewError("invalid_args", "--direction must be up, down, left, or right", "Use a bounded direction.", 400)
	}
	return appium.ActionsRequest{Actions: []map[string]any{{
		"type": "pointer", "id": "finger1", "parameters": map[string]any{"pointerType": "touch"},
		"actions": []map[string]any{
			{"type": "pointerMove", "duration": 0, "x": startX, "y": startY},
			{"type": "pointerDown", "button": 0},
			{"type": "pointerMove", "duration": duration, "x": endX, "y": endY},
			{"type": "pointerUp", "button": 0},
		},
	}}}, nil
}

func backCmd(o *Opts) *cobra.Command {
	var runID string
	c := &cobra.Command{Use: "back", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		err = svc.Store.WithRunLock(runID, func() error {
			st, err := svc.Store.LoadRun(runID)
			if err != nil {
				return err
			}
			if st.ControlOwner == "human" {
				return mobile.NewError("control_locked", "run control belongs to the human", "Run mobile run resume before mutating actions.", 423)
			}
			if err := svc.Appium.Back(cmd.Context(), st.SessionID); err != nil {
				return err
			}
			st.LatestObservationID = ""
			return svc.Store.SaveRun(st)
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"action": "back", "observation_invalidated": true}))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	return c
}

func contextCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "context"}
	var runID string
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		svc, st, err := servicesAndRun(o, runID, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		contexts, err := svc.Appium.Contexts(cmd.Context(), st.SessionID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"contexts": contexts}))
	}}
	list.Flags().StringVar(&runID, "run-id", "", "")
	var name string
	sw := &cobra.Command{Use: "switch", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			return print(cmd, o, output.Failure("invalid_args", "--name is required", "Pass a context name from context list.", 400))
		}
		svc, st, err := servicesAndRun(o, runID, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if st.ControlOwner == "human" {
			return renderErr(cmd, o, mobile.NewError("control_locked", "run control belongs to the human", "Run mobile run resume first.", 423))
		}
		if err := svc.Appium.SwitchContext(cmd.Context(), st.SessionID, name); err != nil {
			return renderErr(cmd, o, err)
		}
		st.LatestObservationID = ""
		_ = svc.Store.SaveRun(st)
		return print(cmd, o, output.Success("", map[string]any{"context": name, "observation_invalidated": true}))
	}}
	sw.Flags().StringVar(&runID, "run-id", "", "")
	sw.Flags().StringVar(&name, "name", "", "")
	c.AddCommand(list, sw)
	return c
}

func servicesAndRun(o *Opts, runID string, auth bool) (*services, mobile.RunState, error) {
	var st mobile.RunState
	if runID == "" {
		return nil, st, mobile.NewError("invalid_args", "--run-id is required", "Pass the active run id.", 400)
	}
	svc, err := newServices(o, auth)
	if err != nil {
		return nil, st, err
	}
	st, err = svc.Store.LoadRun(runID)
	return svc, st, err
}
