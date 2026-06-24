package commands

import (
	"context"

	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func permissionsCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "permissions"}
	c.AddCommand(permissionAcceptCmd(o), permissionDenyCmd(o))
	return c
}

func permissionAcceptCmd(o *Opts) *cobra.Command {
	var runID string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "accept", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, runID, "permissions_accept", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return nil, svc.Appium.AcceptAlert(ctx, st.SessionID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}

func permissionDenyCmd(o *Opts) *cobra.Command {
	var runID string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "deny", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, runID, "permissions_deny", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return nil, svc.Appium.DismissAlert(ctx, st.SessionID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}
