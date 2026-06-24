package commands

import (
	"context"
	"fmt"
	"io"
	"math"
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
		markRunLostIfSessionGone(svc, st, err)
		return mobile.Observation{}, err
	}
	screen, err := svc.Appium.Screenshot(ctx, st.SessionID)
	if err != nil {
		markRunLostIfSessionGone(svc, st, err)
		return mobile.Observation{}, err
	}
	st.ObservationVersion++
	obsID := mobile.NewObservationID(st.ObservationVersion)
	obs, err := mobile.BuildObservationStrict(st.RunID, st.SessionID, obsID, source, screen)
	if err != nil {
		return mobile.Observation{}, err
	}
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
	return mobile.LimitObservationCandidates(obs, limit), nil
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
		value, ok := os.LookupEnv(textEnv)
		if !ok {
			return "", "", fmt.Errorf("--text-env %s is not set", textEnv)
		}
		return value, "env", nil
	default:
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(b), "\r\n"), "stdin", nil
	}
}

type pointTargetOptions struct {
	RunID    string
	Ref      string
	X        int
	Y        int
	XPercent float64
	YPercent float64
}

func tapPointCmd(o *Opts) *cobra.Command {
	opts := pointTargetOptions{X: -1, Y: -1, XPercent: -1, YPercent: -1}
	c := &cobra.Command{Use: "tap-point", RunE: func(cmd *cobra.Command, args []string) error {
		if opts.RunID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, opts.RunID, "tap_point", func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			target, viewport, err := resolvePointTarget(ctx, svc, st, opts.RunID, opts)
			if err != nil {
				return nil, err
			}
			if err := svc.Appium.PerformActions(ctx, st.SessionID, pointerTapActions(target.Point)); err != nil {
				return nil, err
			}
			return map[string]any{"target": target, "viewport": viewport}, nil
		})
	}}
	bindPointTargetFlags(c, &opts, false)
	return c
}

func longPressCmd(o *Opts) *cobra.Command {
	opts := pointTargetOptions{X: -1, Y: -1, XPercent: -1, YPercent: -1}
	var duration int
	c := &cobra.Command{Use: "long-press", RunE: func(cmd *cobra.Command, args []string) error {
		if opts.RunID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, opts.RunID, "long_press", func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			target, viewport, err := resolvePointTarget(ctx, svc, st, opts.RunID, opts)
			if err != nil {
				return nil, err
			}
			if err := svc.Appium.PerformActions(ctx, st.SessionID, pointerLongPressActions(target.Point, duration)); err != nil {
				return nil, err
			}
			return map[string]any{"target": target, "viewport": viewport, "duration_ms": normalizeDuration(duration, 800)}, nil
		})
	}}
	bindPointTargetFlags(c, &opts, true)
	c.Flags().IntVar(&duration, "duration-ms", 800, "")
	return c
}

func doubleTapCmd(o *Opts) *cobra.Command {
	opts := pointTargetOptions{X: -1, Y: -1, XPercent: -1, YPercent: -1}
	c := &cobra.Command{Use: "double-tap", RunE: func(cmd *cobra.Command, args []string) error {
		if opts.RunID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, opts.RunID, "double_tap", func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			target, viewport, err := resolvePointTarget(ctx, svc, st, opts.RunID, opts)
			if err != nil {
				return nil, err
			}
			if err := svc.Appium.PerformActions(ctx, st.SessionID, pointerDoubleTapActions(target.Point)); err != nil {
				return nil, err
			}
			return map[string]any{"target": target, "viewport": viewport}, nil
		})
	}}
	bindPointTargetFlags(c, &opts, true)
	return c
}

type dragCommandOptions struct {
	RunID        string
	FromRef      string
	ToRef        string
	FromX        int
	FromY        int
	ToX          int
	ToY          int
	FromXPercent float64
	FromYPercent float64
	ToXPercent   float64
	ToYPercent   float64
	DurationMS   int
}

func dragCmd(o *Opts) *cobra.Command {
	opts := dragCommandOptions{FromX: -1, FromY: -1, ToX: -1, ToY: -1, FromXPercent: -1, FromYPercent: -1, ToXPercent: -1, ToYPercent: -1, DurationMS: 700}
	c := &cobra.Command{Use: "drag", RunE: func(cmd *cobra.Command, args []string) error {
		if opts.RunID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, opts.RunID, "drag", func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			from, to, viewport, err := resolveDragTargets(ctx, svc, st, opts)
			if err != nil {
				return nil, err
			}
			if err := svc.Appium.PerformActions(ctx, st.SessionID, pointerSwipeActions(from.Point, to.Point, opts.DurationMS)); err != nil {
				return nil, err
			}
			return map[string]any{"from": from, "to": to, "viewport": viewport, "duration_ms": normalizeDuration(opts.DurationMS, 700)}, nil
		})
	}}
	c.Flags().StringVar(&opts.RunID, "run-id", "", "")
	c.Flags().StringVar(&opts.FromRef, "from-ref", "", "")
	c.Flags().StringVar(&opts.ToRef, "to-ref", "", "")
	c.Flags().IntVar(&opts.FromX, "from-x", -1, "")
	c.Flags().IntVar(&opts.FromY, "from-y", -1, "")
	c.Flags().IntVar(&opts.ToX, "to-x", -1, "")
	c.Flags().IntVar(&opts.ToY, "to-y", -1, "")
	c.Flags().Float64Var(&opts.FromXPercent, "from-x-percent", -1, "")
	c.Flags().Float64Var(&opts.FromYPercent, "from-y-percent", -1, "")
	c.Flags().Float64Var(&opts.ToXPercent, "to-x-percent", -1, "")
	c.Flags().Float64Var(&opts.ToYPercent, "to-y-percent", -1, "")
	c.Flags().IntVar(&opts.DurationMS, "duration-ms", 700, "")
	return c
}

func bindPointTargetFlags(c *cobra.Command, opts *pointTargetOptions, allowRef bool) {
	c.Flags().StringVar(&opts.RunID, "run-id", "", "")
	if allowRef {
		c.Flags().StringVar(&opts.Ref, "ref", "", "")
	}
	c.Flags().IntVar(&opts.X, "x", -1, "")
	c.Flags().IntVar(&opts.Y, "y", -1, "")
	c.Flags().Float64Var(&opts.XPercent, "x-percent", -1, "")
	c.Flags().Float64Var(&opts.YPercent, "y-percent", -1, "")
}

func resolvePointTarget(ctx context.Context, svc *services, st *mobile.RunState, runID string, opts pointTargetOptions) (gestureTarget, *appium.Rect, error) {
	if opts.Ref != "" {
		_, _, _, element, _, err := resolveRefElement(ctx, svc, runID, opts.Ref)
		if err != nil {
			return gestureTarget{}, nil, err
		}
		rect, err := svc.Appium.ElementRect(ctx, st.SessionID, element.ID)
		if err != nil {
			return gestureTarget{}, nil, err
		}
		target := gestureTarget{Point: rectCenter(rect), Source: "ref", ElementID: element.ID, Rect: &rect}
		return target, nil, nil
	}
	return resolveCoordinateTarget(ctx, svc, st, opts.X, opts.Y, opts.XPercent, opts.YPercent, "point")
}

func resolveDragTargets(ctx context.Context, svc *services, st *mobile.RunState, opts dragCommandOptions) (gestureTarget, gestureTarget, *appium.Rect, error) {
	fromOpts := pointTargetOptions{RunID: opts.RunID, Ref: opts.FromRef, X: opts.FromX, Y: opts.FromY, XPercent: opts.FromXPercent, YPercent: opts.FromYPercent}
	toOpts := pointTargetOptions{RunID: opts.RunID, Ref: opts.ToRef, X: opts.ToX, Y: opts.ToY, XPercent: opts.ToXPercent, YPercent: opts.ToYPercent}
	from, viewport, err := resolvePointTarget(ctx, svc, st, opts.RunID, fromOpts)
	if err != nil {
		return gestureTarget{}, gestureTarget{}, nil, err
	}
	to, toViewport, err := resolvePointTarget(ctx, svc, st, opts.RunID, toOpts)
	if err != nil {
		return gestureTarget{}, gestureTarget{}, nil, err
	}
	if viewport == nil {
		viewport = toViewport
	}
	return from, to, viewport, nil
}

func resolveCoordinateTarget(ctx context.Context, svc *services, st *mobile.RunState, x, y int, xPercent, yPercent float64, source string) (gestureTarget, *appium.Rect, error) {
	hasAbs := x >= 0 || y >= 0
	hasPercent := xPercent >= 0 || yPercent >= 0
	if hasAbs && hasPercent {
		return gestureTarget{}, nil, mobile.NewError("invalid_args", "use either absolute coordinates or percent coordinates", "Choose --x/--y or --x-percent/--y-percent.", 400)
	}
	if hasAbs {
		if x < 0 || y < 0 {
			return gestureTarget{}, nil, mobile.NewError("invalid_args", "absolute point requires both x and y", "Pass both --x and --y.", 400)
		}
		return gestureTarget{Point: gesturePoint{X: x, Y: y}, Source: source + "_absolute"}, nil, nil
	}
	if hasPercent {
		if xPercent < 0 || xPercent > 100 || yPercent < 0 || yPercent > 100 {
			return gestureTarget{}, nil, mobile.NewError("invalid_args", "percent point requires x/y percent between 0 and 100", "Pass both --x-percent and --y-percent.", 400)
		}
		rect, err := svc.Appium.WindowRect(ctx, st.SessionID)
		if err != nil {
			return gestureTarget{}, nil, err
		}
		if err := validateViewport(rect); err != nil {
			return gestureTarget{}, nil, err
		}
		return gestureTarget{Point: percentPoint(rect, xPercent, yPercent), Source: source + "_percent"}, &rect, nil
	}
	return gestureTarget{}, nil, mobile.NewError("invalid_args", "a gesture target is required", "Pass --ref, --x/--y, or --x-percent/--y-percent.", 400)
}

func rectCenter(rect appium.Rect) gesturePoint {
	return gesturePoint{
		X: int(math.Round(rect.X + rect.Width/2)),
		Y: int(math.Round(rect.Y + rect.Height/2)),
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
			markRunLostIfSessionGone(svc, &st, err)
			return err
		}
		extra, err := fn(cmd.Context(), svc, st, element)
		if err != nil {
			markRunLostIfSessionGone(svc, &st, err)
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

type swipeCommandOptions struct {
	RunID         string
	Direction     string
	DurationMS    int
	StartXPercent float64
	StartYPercent float64
	EndXPercent   float64
	EndYPercent   float64
}

type gesturePoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type gestureTarget struct {
	Point     gesturePoint `json:"point"`
	Source    string       `json:"source"`
	ElementID string       `json:"element_id,omitempty"`
	Rect      *appium.Rect `json:"rect,omitempty"`
}

func scrollCmd(o *Opts) *cobra.Command {
	opts := swipeCommandOptions{Direction: "down", DurationMS: 500, StartXPercent: -1, StartYPercent: -1, EndXPercent: -1, EndYPercent: -1}
	c := &cobra.Command{Use: "scroll", RunE: func(cmd *cobra.Command, args []string) error {
		return swipeLike(cmd, o, opts, "scroll")
	}}
	bindSwipeCommandFlags(c, &opts)
	return c
}

func swipeCmd(o *Opts) *cobra.Command {
	opts := swipeCommandOptions{Direction: "up", DurationMS: 500, StartXPercent: -1, StartYPercent: -1, EndXPercent: -1, EndYPercent: -1}
	c := &cobra.Command{Use: "swipe", RunE: func(cmd *cobra.Command, args []string) error {
		return swipeLike(cmd, o, opts, "swipe")
	}}
	bindSwipeCommandFlags(c, &opts)
	return c
}

func scrollToCmd(o *Opts) *cobra.Command {
	opts := swipeCommandOptions{Direction: "down", DurationMS: 500, StartXPercent: -1, StartYPercent: -1, EndXPercent: -1, EndYPercent: -1}
	q := mobile.LocateQuery{}
	var limit, maxScrolls int
	var visible, enabled bool
	var useVisible, useEnabled bool
	c := &cobra.Command{Use: "scroll-to", RunE: func(cmd *cobra.Command, args []string) error {
		if opts.RunID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		if !locateQueryHasCriteria(q) {
			return print(cmd, o, output.Failure("invalid_args", "at least one locate criterion is required", "Pass --text, --name, --role, --resource-id, --accessibility-id, or --parent-text.", 400))
		}
		if maxScrolls < 0 {
			return print(cmd, o, output.Failure("invalid_args", "--max-scrolls cannot be negative", "Use 0 to only observe and locate without scrolling.", 400))
		}
		if useVisible {
			q.Visible = &visible
		}
		if useEnabled {
			q.Enabled = &enabled
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var data map[string]any
		err = svc.Store.WithRunLock(opts.RunID, func() error {
			st, err := svc.Store.LoadRun(opts.RunID)
			if err != nil {
				return err
			}
			if st.ControlOwner == "human" {
				return mobile.NewError("control_locked", "run control belongs to the human", "Run mobile run resume before mutating actions.", 423)
			}
			for scrolls := 0; scrolls <= maxScrolls; scrolls++ {
				obs, err := captureObservation(cmd.Context(), svc, &st, limit)
				if err != nil {
					return err
				}
				located := mobile.Locate(obs, q)
				if ref := scrollToResolvedRef(located); ref != "" {
					data = map[string]any{"found": true, "run_id": opts.RunID, "scrolls": scrolls, "recommended_ref": ref, "locate": located, "observation": obs}
					return nil
				}
				if scrolls == maxScrolls {
					return mobile.RetryableError("element_not_found", "target was not found after scrolling", "Try another direction, increase --max-scrolls, or broaden the locate query.", "observe", 404)
				}
				rect, err := svc.Appium.WindowRect(cmd.Context(), st.SessionID)
				if err != nil {
					markRunLostIfSessionGone(svc, &st, err)
					return err
				}
				start, end, err := swipePointsForViewport(rect, opts)
				if err != nil {
					return err
				}
				if err := svc.Appium.PerformActions(cmd.Context(), st.SessionID, pointerSwipeActions(start, end, opts.DurationMS)); err != nil {
					markRunLostIfSessionGone(svc, &st, err)
					return err
				}
				st.LatestObservationID = ""
				if err := svc.Store.SaveRun(st); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", data))
	}}
	c.Flags().IntVar(&limit, "limit", 100, "")
	c.Flags().IntVar(&maxScrolls, "max-scrolls", 8, "")
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
	bindSwipeCommandFlags(c, &opts)
	return c
}

func locateQueryHasCriteria(q mobile.LocateQuery) bool {
	return strings.TrimSpace(q.Name) != "" || strings.TrimSpace(q.Text) != "" || strings.TrimSpace(q.Role) != "" ||
		strings.TrimSpace(q.ResourceID) != "" || strings.TrimSpace(q.AccessibilityID) != "" ||
		strings.TrimSpace(q.ParentText) != "" || strings.TrimSpace(q.NearbyText) != "" || q.Actionable
}

func scrollToResolvedRef(res mobile.LocateResult) string {
	if res.RecommendedRef != "" {
		return res.RecommendedRef
	}
	if len(res.Matches) == 1 {
		return res.Matches[0].Candidate.Ref
	}
	return ""
}

func bindSwipeCommandFlags(c *cobra.Command, opts *swipeCommandOptions) {
	c.Flags().StringVar(&opts.RunID, "run-id", "", "")
	c.Flags().StringVar(&opts.Direction, "direction", opts.Direction, "")
	c.Flags().IntVar(&opts.DurationMS, "duration-ms", opts.DurationMS, "")
	c.Flags().Float64Var(&opts.StartXPercent, "start-x-percent", -1, "")
	c.Flags().Float64Var(&opts.StartYPercent, "start-y-percent", -1, "")
	c.Flags().Float64Var(&opts.EndXPercent, "end-x-percent", -1, "")
	c.Flags().Float64Var(&opts.EndYPercent, "end-y-percent", -1, "")
}

func swipeLike(cmd *cobra.Command, o *Opts, opts swipeCommandOptions, action string) error {
	if opts.RunID == "" {
		return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
	}
	err := runGesture(cmd, o, opts.RunID, action, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
		rect, err := svc.Appium.WindowRect(ctx, st.SessionID)
		if err != nil {
			return nil, err
		}
		start, end, err := swipePointsForViewport(rect, opts)
		if err != nil {
			return nil, err
		}
		actions := pointerSwipeActions(start, end, opts.DurationMS)
		if err := svc.Appium.PerformActions(ctx, st.SessionID, actions); err != nil {
			return nil, err
		}
		return map[string]any{"direction": opts.Direction, "duration_ms": normalizeDuration(opts.DurationMS, 500), "viewport": rect, "start": start, "end": end}, nil
	})
	return err
}

func runGesture(cmd *cobra.Command, o *Opts, runID, action string, fn func(context.Context, *services, *mobile.RunState) (map[string]any, error)) error {
	svc, err := newServices(o, true)
	if err != nil {
		return renderErr(cmd, o, err)
	}
	var data map[string]any
	err = svc.Store.WithRunLock(runID, func() error {
		st, err := svc.Store.LoadRun(runID)
		if err != nil {
			return err
		}
		if st.ControlOwner == "human" {
			return mobile.NewError("control_locked", "run control belongs to the human", "Run mobile run resume before mutating actions.", 423)
		}
		extra, err := fn(cmd.Context(), svc, &st)
		if err != nil {
			markRunLostIfSessionGone(svc, &st, err)
			return err
		}
		st.LatestObservationID = ""
		if err := svc.Store.SaveRun(st); err != nil {
			return err
		}
		data = map[string]any{"action": action, "run_id": runID, "observation_invalidated": true}
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

func swipePointsForViewport(rect appium.Rect, opts swipeCommandOptions) (gesturePoint, gesturePoint, error) {
	if err := validateViewport(rect); err != nil {
		return gesturePoint{}, gesturePoint{}, err
	}
	startX, startY, endX, endY, err := swipePercents(opts)
	if err != nil {
		return gesturePoint{}, gesturePoint{}, err
	}
	return percentPoint(rect, startX, startY), percentPoint(rect, endX, endY), nil
}

func swipePercents(opts swipeCommandOptions) (float64, float64, float64, float64, error) {
	custom := opts.StartXPercent >= 0 || opts.StartYPercent >= 0 || opts.EndXPercent >= 0 || opts.EndYPercent >= 0
	if custom {
		values := []float64{opts.StartXPercent, opts.StartYPercent, opts.EndXPercent, opts.EndYPercent}
		for _, v := range values {
			if v < 0 || v > 100 {
				return 0, 0, 0, 0, mobile.NewError("invalid_args", "custom swipe percentages must all be between 0 and 100", "Pass all of --start-x-percent, --start-y-percent, --end-x-percent, and --end-y-percent.", 400)
			}
		}
		return opts.StartXPercent, opts.StartYPercent, opts.EndXPercent, opts.EndYPercent, nil
	}
	switch strings.ToLower(strings.TrimSpace(opts.Direction)) {
	case "up":
		return 50, 80, 50, 20, nil
	case "down":
		return 50, 20, 50, 80, nil
	case "left":
		return 80, 50, 20, 50, nil
	case "right":
		return 20, 50, 80, 50, nil
	default:
		return 0, 0, 0, 0, mobile.NewError("invalid_args", "--direction must be up, down, left, or right", "Use a bounded direction or pass explicit start/end percentages.", 400)
	}
}

func pointerSwipeActions(start, end gesturePoint, duration int) appium.ActionsRequest {
	return appium.ActionsRequest{Actions: []map[string]any{{
		"type": "pointer", "id": "finger1", "parameters": map[string]any{"pointerType": "touch"},
		"actions": []map[string]any{
			{"type": "pointerMove", "duration": 0, "x": start.X, "y": start.Y},
			{"type": "pointerDown", "button": 0},
			{"type": "pointerMove", "duration": normalizeDuration(duration, 500), "x": end.X, "y": end.Y},
			{"type": "pointerUp", "button": 0},
		},
	}}}
}

func pointerTapActions(point gesturePoint) appium.ActionsRequest {
	return appium.ActionsRequest{Actions: []map[string]any{{
		"type": "pointer", "id": "finger1", "parameters": map[string]any{"pointerType": "touch"},
		"actions": []map[string]any{
			{"type": "pointerMove", "duration": 0, "x": point.X, "y": point.Y},
			{"type": "pointerDown", "button": 0},
			{"type": "pointerUp", "button": 0},
		},
	}}}
}

func pointerLongPressActions(point gesturePoint, duration int) appium.ActionsRequest {
	return appium.ActionsRequest{Actions: []map[string]any{{
		"type": "pointer", "id": "finger1", "parameters": map[string]any{"pointerType": "touch"},
		"actions": []map[string]any{
			{"type": "pointerMove", "duration": 0, "x": point.X, "y": point.Y},
			{"type": "pointerDown", "button": 0},
			{"type": "pause", "duration": normalizeDuration(duration, 800)},
			{"type": "pointerUp", "button": 0},
		},
	}}}
}

func pointerDoubleTapActions(point gesturePoint) appium.ActionsRequest {
	return appium.ActionsRequest{Actions: []map[string]any{{
		"type": "pointer", "id": "finger1", "parameters": map[string]any{"pointerType": "touch"},
		"actions": []map[string]any{
			{"type": "pointerMove", "duration": 0, "x": point.X, "y": point.Y},
			{"type": "pointerDown", "button": 0},
			{"type": "pointerUp", "button": 0},
			{"type": "pause", "duration": 100},
			{"type": "pointerDown", "button": 0},
			{"type": "pointerUp", "button": 0},
		},
	}}}
}

func validateViewport(rect appium.Rect) error {
	if rect.Width <= 0 || rect.Height <= 0 {
		return mobile.NewError("server_error", "Appium window rect did not include a usable viewport", "Retry the command after the session is fully ready.", 502)
	}
	return nil
}

func percentPoint(rect appium.Rect, xPercent, yPercent float64) gesturePoint {
	return gesturePoint{
		X: int(math.Round(rect.X + rect.Width*xPercent/100)),
		Y: int(math.Round(rect.Y + rect.Height*yPercent/100)),
	}
}

func normalizeDuration(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
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
				markRunLostIfSessionGone(svc, &st, err)
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
			markRunLostIfSessionGone(svc, &st, err)
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
		svc, _, err := servicesAndRun(o, runID, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		err = svc.Store.WithRunLock(runID, func() error {
			st, err := svc.Store.LoadRun(runID)
			if err != nil {
				return err
			}
			if st.ControlOwner == "human" {
				return mobile.NewError("control_locked", "run control belongs to the human", "Run mobile run resume first.", 423)
			}
			if err := svc.Appium.SwitchContext(cmd.Context(), st.SessionID, name); err != nil {
				markRunLostIfSessionGone(svc, &st, err)
				return err
			}
			st.LatestObservationID = ""
			return svc.Store.SaveRun(st)
		})
		if err != nil {
			return renderErr(cmd, o, err)
		}
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
