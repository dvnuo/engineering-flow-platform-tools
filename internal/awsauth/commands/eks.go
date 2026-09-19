package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type eksTarget struct {
	AccountName string
	AccountID   string
	Profile     string
	Region      string
}

func (t eksTarget) label() string {
	if t.AccountName != "" {
		return t.AccountName
	}
	return t.AccountID
}

func resolveEKSTarget(aws config.AWSConfig, account, region string) (eksTarget, *output.Envelope) {
	sel, failure := resolveAccount(aws, account)
	if failure != nil {
		return eksTarget{}, failure
	}
	target := eksTarget{}
	switch {
	case sel.Configured:
		target.AccountName = sel.Account.Name
		target.AccountID = sel.Account.AccountID
		target.Profile = sel.Account.EffectiveProfile()
		if len(sel.Account.Regions) > 0 {
			target.Region = sel.Account.Regions[0]
		}
	case sel.Account.AccountID != "":
		target.AccountID = sel.Account.AccountID
		target.Profile = defaultAWSAuthProfile
	default:
		failure := output.Failure("invalid_args", "account is required", "Pass --account with a configured account name, or configure aws.default_account.", 400)
		return eksTarget{}, &failure
	}
	if region := singleLine(region); region != "" {
		target.Region = region
	} else if target.Region == "" {
		target.Region = aws.DefaultRegion
	}
	if target.Region == "" {
		failure := output.Failure("invalid_args", "region is required", "Pass --region, or configure regions for the account or aws.default_region.", 400)
		return eksTarget{}, &failure
	}
	return target, nil
}

// runAWSCLI executes the AWS CLI with the profile the provider wrote and maps
// failures onto stable codes so agents can react (session_expired -> re-login).
func runAWSCLI(ctx context.Context, runner commandRunner, args []string) (string, *output.Envelope) {
	result, err := runner.Run(ctx, awsCLICommand, args, stripSecretEnv(os.Environ()))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			failure := output.Failure("aws_cli_missing", "aws CLI is not installed or not on PATH", "Install AWS CLI v2 in the runtime image.", 500)
			return "", &failure
		}
		failure := output.Failure("execution_failed", output.RedactString(err.Error()), "", 500)
		return "", &failure
	}
	if result.ExitCode != 0 {
		message := firstNonEmpty(result.Stderr, result.Stdout, fmt.Sprintf("aws exited with %d", result.ExitCode))
		code, hint := "aws_cli_failed", "Inspect the message; the AWS CLI reported the failure."
		lower := strings.ToLower(message)
		switch {
		case strings.Contains(lower, "expiredtoken") || strings.Contains(lower, "token included in the request is expired") || strings.Contains(lower, "expired"):
			code, hint = "session_expired", "Run aws-auth login for this account again, then retry."
		case strings.Contains(lower, "unable to locate credentials") || strings.Contains(lower, "could not be found") || strings.Contains(lower, "profile") && strings.Contains(lower, "not found"):
			code, hint = "credentials_missing", "Run aws-auth login for this account first; it writes the profile the command uses."
		case strings.Contains(lower, "accessdenied") || strings.Contains(lower, "not authorized"):
			code, hint = "permission_denied", "The assumed role lacks permission for this call."
		}
		failure := output.Failure(code, truncateText(output.RedactString(message), 1000), hint, 500)
		return "", &failure
	}
	return result.Stdout, nil
}

func resolveKubeconfigPath(flag string, aws config.AWSConfig) string {
	if p := strings.TrimSpace(flag); p != "" {
		return expandHome(p)
	}
	if env := strings.TrimSpace(os.Getenv("KUBECONFIG")); env != "" {
		first := strings.Split(env, string(os.PathListSeparator))[0]
		if strings.TrimSpace(first) != "" {
			return expandHome(first)
		}
	}
	return expandHome(aws.EffectiveKubeconfigPath())
}

func eksCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "eks"}
	c.AddCommand(eksListCmd(o), eksKubeconfigCmd(o))
	return c
}

func eksListCmd(o *Opts) *cobra.Command {
	var account, region string
	c := &cobra.Command{
		Use:   "list",
		Short: "List the EKS clusters visible to an account's credentials in one region.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadAWSConfigForRead(o.Config)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check EFP_CONFIG or pass --config.", 400))
			}
			target, failure := resolveEKSTarget(cfg.AWS, account, region)
			if failure != nil {
				return print(cmd, o, *failure)
			}
			cliArgs := []string{"eks", "list-clusters", "--profile", target.Profile, "--region", target.Region, "--output", "json"}
			data := map[string]any{
				"account":    target.AccountName,
				"account_id": target.AccountID,
				"profile":    target.Profile,
				"region":     target.Region,
				"command":    formatCommand(awsCLICommand, cliArgs),
			}
			if o.DryRun {
				data["dry_run"] = true
				return print(cmd, o, output.Success("", data))
			}
			stdout, failure := runAWSCLI(cmd.Context(), o.runner, cliArgs)
			if failure != nil {
				return print(cmd, o, *failure)
			}
			var parsed struct {
				Clusters []string `json:"clusters"`
			}
			if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
				return print(cmd, o, output.Failure("aws_cli_failed", "aws eks list-clusters returned unexpected output", "Retry with --verbose or run the command manually.", 500))
			}
			if parsed.Clusters == nil {
				parsed.Clusters = []string{}
			}
			data["clusters"] = parsed.Clusters
			data["count"] = len(parsed.Clusters)
			return print(cmd, o, output.Success("", data))
		},
	}
	c.Flags().StringVar(&account, "account", "", "Configured account name or id; defaults to aws.default_account.")
	c.Flags().StringVar(&region, "region", "", "AWS region; defaults to the account's first region.")
	return c
}

func eksKubeconfigCmd(o *Opts) *cobra.Command {
	var account, cluster, region, alias, kubeconfig string
	c := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Write a kubectl context named <account>/<cluster> that authenticates through the account's AWS profile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadAWSConfigForRead(o.Config)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check EFP_CONFIG or pass --config.", 400))
			}
			cluster = singleLine(cluster)
			if cluster == "" {
				return print(cmd, o, output.Failure("invalid_args", "cluster is required", "Pass --cluster with the EKS cluster name; aws-auth eks list --json shows the names.", 400))
			}
			target, failure := resolveEKSTarget(cfg.AWS, account, region)
			if failure != nil {
				return print(cmd, o, *failure)
			}
			contextName := singleLine(alias)
			if contextName == "" {
				contextName = target.label() + "/" + cluster
			}
			path := resolveKubeconfigPath(kubeconfig, cfg.AWS)
			updateArgs := []string{"eks", "update-kubeconfig", "--name", cluster, "--region", target.Region, "--profile", target.Profile, "--alias", contextName, "--kubeconfig", path}
			verifyArgs := []string{"--kubeconfig", path, "--context", contextName, "auth", "can-i", "list", "pods", "--all-namespaces"}
			data := map[string]any{
				"account":    target.AccountName,
				"account_id": target.AccountID,
				"profile":    target.Profile,
				"region":     target.Region,
				"cluster":    cluster,
				"context":    contextName,
				"kubeconfig": path,
				"commands":   []string{formatCommand(awsCLICommand, updateArgs), formatCommand(kubectlCommand, verifyArgs)},
			}
			if o.DryRun {
				data["dry_run"] = true
				return print(cmd, o, output.Success("", data))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check that the kubeconfig directory is writable.", 500))
			}
			if _, failure := runAWSCLI(cmd.Context(), o.runner, updateArgs); failure != nil {
				return print(cmd, o, *failure)
			}
			data["kubectl_missing"] = false
			result, err := o.runner.Run(cmd.Context(), kubectlCommand, verifyArgs, stripSecretEnv(os.Environ()))
			switch {
			case err != nil && errors.Is(err, exec.ErrNotFound):
				data["kubectl_missing"] = true
				data["can_list_pods"] = nil
			case err != nil:
				data["can_list_pods"] = nil
				data["verify_error"] = output.RedactString(err.Error())
			default:
				answer := strings.ToLower(strings.TrimSpace(result.Stdout))
				switch {
				case answer == "yes":
					data["can_list_pods"] = true
				case answer == "no":
					data["can_list_pods"] = false
				default:
					data["can_list_pods"] = nil
					data["verify_error"] = truncateText(output.RedactString(firstNonEmpty(result.Stderr, result.Stdout, fmt.Sprintf("kubectl exited with %d", result.ExitCode))), 500)
				}
			}
			return print(cmd, o, output.Success("", data))
		},
	}
	c.Flags().StringVar(&account, "account", "", "Configured account name or id; defaults to aws.default_account.")
	c.Flags().StringVar(&cluster, "cluster", "", "EKS cluster name.")
	c.Flags().StringVar(&region, "region", "", "AWS region; defaults to the account's first region.")
	c.Flags().StringVar(&alias, "alias", "", "kubectl context name; defaults to <account>/<cluster>.")
	c.Flags().StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig file to update; defaults to KUBECONFIG or aws.kubeconfig_path.")
	return c
}
