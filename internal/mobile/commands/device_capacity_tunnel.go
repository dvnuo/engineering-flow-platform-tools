package commands

import (
	"context"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browserstack"
	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func deviceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "device"}
	c.AddCommand(deviceListCmd(o), deviceResolveCmd(o), deviceUsageCmd(o))
	return c
}

func deviceListCmd(o *Opts) *cobra.Command {
	var platform string
	var realOnly bool
	c := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		devices, err := svc.Control.ListDevices(cmd.Context())
		if err != nil {
			return renderErr(cmd, o, err)
		}
		filtered := devices[:0]
		for _, d := range devices {
			if platform != "" && !equalFold(d.OS, platform) {
				continue
			}
			if realOnly && !d.RealMobile {
				continue
			}
			filtered = append(filtered, d)
		}
		return print(cmd, o, output.Success("", map[string]any{"devices": filtered, "count": len(filtered)}))
	}}
	c.Flags().StringVar(&platform, "platform", "", "")
	c.Flags().BoolVar(&realOnly, "real-only", false, "")
	return c
}

func deviceResolveCmd(o *Opts) *cobra.Command {
	q := mobile.DeviceQuery{}
	c := &cobra.Command{Use: "resolve", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		devices, err := svc.Control.ListDevices(cmd.Context())
		if err != nil {
			return renderErr(cmd, o, err)
		}
		res, err := mobile.ResolveDevice(deviceInfos(devices), q)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", res))
	}}
	c.Flags().StringVar(&q.Platform, "platform", "", "")
	c.Flags().StringVar(&q.OSVersion, "os-version", "", "")
	c.Flags().StringVar(&q.MinOSVersion, "min-os-version", "", "")
	c.Flags().StringVar(&q.Manufacturer, "manufacturer", "", "")
	c.Flags().StringVar(&q.Name, "device", "", "")
	c.Flags().BoolVar(&q.RealOnly, "real-only", true, "")
	c.Flags().StringVar(&q.Tier, "tier", "", "")
	c.Flags().StringVar(&q.Strategy, "strategy", "latest-compatible", "")
	return c
}

func deviceUsageCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "usage", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		usage, err := svc.Control.DeviceTierUsage(cmd.Context())
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"usage": usage}))
	}}
}

func capacityCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "capacity"}
	c.AddCommand(capacityGetCmd(o), capacityWaitCmd(o))
	return c
}

func capacityGetCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		plan, err := svc.Control.GetPlan(cmd.Context())
		if err != nil {
			return renderErr(cmd, o, err)
		}
		usage, _ := svc.Control.CurrentParallelQueueUsage(cmd.Context())
		return print(cmd, o, output.Success("", map[string]any{"plan": plan, "current_usage": usage}))
	}}
}

func capacityWaitCmd(o *Opts) *cobra.Command {
	var required int
	var timeoutText, pollText string
	c := &cobra.Command{Use: "wait", RunE: func(cmd *cobra.Command, args []string) error {
		if required <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "--required must be greater than zero", "Pass --required N.", 400))
		}
		timeout, err := time.ParseDuration(timeoutText)
		if err != nil || timeout <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --timeout", "Use a Go duration such as 5m or 30s.", 400))
		}
		poll, err := time.ParseDuration(pollText)
		if err != nil || poll <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --poll-interval", "Use a Go duration such as 10s.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		ticker := time.NewTicker(poll)
		defer ticker.Stop()
		attempts := 0
		for {
			attempts++
			plan, err := svc.Control.GetPlan(ctx)
			if err != nil {
				return renderErr(cmd, o, err)
			}
			allowed := plan.TeamParallelSessionsMaxAllowed
			if allowed == 0 {
				allowed = plan.ParallelSessionsMaxAllowed
			}
			available := allowed - plan.ParallelSessionsRunning
			if available >= required {
				return print(cmd, o, output.Success("", map[string]any{"ready": true, "available": available, "required": required, "attempts": attempts, "plan": plan}))
			}
			select {
			case <-ctx.Done():
				err := mobile.RetryableError("capacity_wait_timeout", "timed out waiting for BrowserStack capacity", "Retry later or lower --required.", "retry", 408)
				return renderErr(cmd, o, err)
			case <-ticker.C:
			}
		}
	}}
	c.Flags().IntVar(&required, "required", 1, "")
	c.Flags().StringVar(&timeoutText, "timeout", "5m", "")
	c.Flags().StringVar(&pollText, "poll-interval", "15s", "")
	return c
}

func tunnelCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "tunnel"}
	c.AddCommand(tunnelStartCmd(o), tunnelEnsureCmd(o), tunnelStatusCmd(o), tunnelStopCmd(o), tunnelCleanupCmd(o))
	return c
}

func tunnelStartCmd(o *Opts) *cobra.Command {
	var runID, mode, localID, hold string
	c := &cobra.Command{Use: "start", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := time.ParseDuration(hold)
		if err != nil || d <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --hold-for", "Use a duration such as 10m.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err := svc.Tunnel.Start(mobile.TunnelStartRequest{RunID: runID, NetworkMode: mode, LocalIdentifier: localID, HoldFor: d})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", st))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&mode, "network", "private-managed", "")
	c.Flags().StringVar(&localID, "local-identifier", "", "")
	c.Flags().StringVar(&hold, "hold-for", "10m", "")
	return c
}

func tunnelEnsureCmd(o *Opts) *cobra.Command {
	var runID, mode, localID, hold string
	c := &cobra.Command{Use: "ensure", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := time.ParseDuration(hold)
		if err != nil || d <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "invalid --hold-for", "Use a duration such as 10m.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if runID != "" && localID != "" {
			if st, err := svc.Tunnel.Status(runID, localID); err == nil && mobile.TunnelReusable(st, time.Now().UTC()) {
				return print(cmd, o, output.Success("", st))
			}
		}
		st, err := svc.Tunnel.Start(mobile.TunnelStartRequest{RunID: runID, NetworkMode: mode, LocalIdentifier: localID, HoldFor: d})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", st))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&mode, "network", "private-managed", "")
	c.Flags().StringVar(&localID, "local-identifier", "", "")
	c.Flags().StringVar(&hold, "hold-for", "10m", "")
	return c
}

func tunnelStatusCmd(o *Opts) *cobra.Command {
	var runID, localID string
	c := &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" || localID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id and --local-identifier are required", "Pass the managed tunnel metadata returned by run start.", 400))
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err := svc.Tunnel.Status(runID, localID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", st))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&localID, "local-identifier", "", "")
	return c
}

func tunnelStopCmd(o *Opts) *cobra.Command {
	var runID, localID string
	var yes bool
	c := &cobra.Command{Use: "stop", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" || localID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id and --local-identifier are required", "Pass the managed tunnel metadata returned by tunnel start or run start.", 400))
		}
		if !yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes is required for tunnel stop", "Re-run with --yes after confirming.", 400))
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err := svc.Tunnel.Load(runID, localID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err = svc.Tunnel.Stop(st)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", st))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&localID, "local-identifier", "", "")
	c.Flags().BoolVar(&yes, "yes", false, "")
	return c
}

func tunnelCleanupCmd(o *Opts) *cobra.Command {
	var yes bool
	c := &cobra.Command{Use: "cleanup-orphans", RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes is required for orphan cleanup", "Re-run with --yes after confirming.", 400))
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		stopped, err := svc.Tunnel.CleanupOrphans()
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"stopped": stopped, "count": len(stopped)}))
	}}
	c.Flags().BoolVar(&yes, "yes", false, "")
	return c
}

func equalFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func deviceInfos(devices []browserstack.Device) []mobile.DeviceInfo {
	out := make([]mobile.DeviceInfo, 0, len(devices))
	for _, d := range devices {
		out = append(out, mobile.DeviceInfo{
			OS:         d.OS,
			OSVersion:  d.OSVersion,
			Name:       d.Name,
			RealMobile: d.RealMobile,
			DeviceTier: d.DeviceTier,
			GroupUsage: d.GroupUsage,
		})
	}
	return out
}
