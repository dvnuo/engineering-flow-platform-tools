package commands

import (
	"fmt"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

// accountSelection is the account a command acts on. Configured is false for
// an ad-hoc account id passed on the command line that is not part of the
// configured matrix (the pre-matrix behaviour of `login --account <id>`).
type accountSelection struct {
	Account    config.AWSAccountConfig
	Configured bool
}

func findAccount(accounts []config.AWSAccountConfig, key string) (config.AWSAccountConfig, bool) {
	for _, acct := range accounts {
		if strings.EqualFold(acct.Name, key) || acct.AccountID == key || strings.EqualFold(acct.EffectiveProfile(), key) {
			return acct, true
		}
	}
	return config.AWSAccountConfig{}, false
}

func accountCandidates(accounts []config.AWSAccountConfig) []string {
	out := make([]string, 0, len(accounts))
	for _, acct := range accounts {
		if acct.Name != "" {
			out = append(out, acct.Name)
			continue
		}
		out = append(out, acct.AccountID)
	}
	return out
}

// resolveAccount picks the account an aws-auth command should act on:
// an explicit --account (configured name/id, or an ad-hoc numeric id), else
// aws.default_account, else the only configured account. It returns an empty
// selection with no failure when nothing is configured and nothing was asked
// for, so callers can fall back to prompting.
func resolveAccount(aws config.AWSConfig, explicit string) (accountSelection, *output.Envelope) {
	enabled := aws.EnabledAccounts()
	explicit = singleLine(explicit)
	if explicit != "" {
		if acct, ok := findAccount(enabled, explicit); ok {
			return accountSelection{Account: acct, Configured: true}, nil
		}
		if len(enabled) == 0 || isDigits(explicit) {
			return accountSelection{Account: config.AWSAccountConfig{AccountID: explicit}}, nil
		}
		failure := output.Failure(
			"unknown_account",
			fmt.Sprintf("account %q is not in the configured account matrix", explicit),
			"Run aws-auth account list --json and pass a configured account name or id, or pass a numeric AWS account id.",
			400,
		)
		failure.Data = map[string]any{"candidates": accountCandidates(enabled)}
		return accountSelection{}, &failure
	}
	if aws.DefaultAccount != "" {
		if acct, ok := findAccount(enabled, aws.DefaultAccount); ok {
			return accountSelection{Account: acct, Configured: true}, nil
		}
		failure := output.Failure(
			"unknown_account",
			fmt.Sprintf("default_account %q is not in the configured account matrix", aws.DefaultAccount),
			"Fix aws.default_account or pass --account explicitly.",
			400,
		)
		failure.Data = map[string]any{"candidates": accountCandidates(enabled)}
		return accountSelection{}, &failure
	}
	switch len(enabled) {
	case 0:
		return accountSelection{}, nil
	case 1:
		return accountSelection{Account: enabled[0], Configured: true}, nil
	default:
		failure := output.Failure(
			"account_required",
			"several accounts are configured; pass --account with one of them",
			"Run aws-auth account list --json to see the configured accounts, or set aws.default_account.",
			400,
		)
		failure.Data = map[string]any{"candidates": accountCandidates(enabled)}
		return accountSelection{}, &failure
	}
}

func effectiveRoleARN(acct config.AWSAccountConfig) string {
	if acct.RoleARN != "" {
		return acct.RoleARN
	}
	if acct.AccountID != "" && acct.Role != "" {
		return fmt.Sprintf("arn:aws:iam::%s:role/%s", acct.AccountID, acct.Role)
	}
	return ""
}

func accountView(aws config.AWSConfig, acct config.AWSAccountConfig) map[string]any {
	regions := acct.Regions
	if regions == nil {
		regions = []string{}
	}
	isDefault := aws.DefaultAccount != "" && (strings.EqualFold(aws.DefaultAccount, acct.Name) || aws.DefaultAccount == acct.AccountID)
	return map[string]any{
		"name":       acct.Name,
		"account_id": acct.AccountID,
		"role":       acct.Role,
		"role_arn":   effectiveRoleARN(acct),
		"regions":    regions,
		"profile":    acct.EffectiveProfile(),
		"enabled":    acct.IsEnabled(),
		"default":    isDefault,
	}
}

func accountCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "account"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		path, cfg, err := loadAWSConfigForRead(o.Config)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", output.RedactString(err.Error()), "Configure the aws node (accounts, provider, credentials) or pass --config.", 404))
		}
		accounts := make([]map[string]any, 0, len(cfg.AWS.Accounts))
		for _, acct := range cfg.AWS.Accounts {
			accounts = append(accounts, accountView(cfg.AWS, acct))
		}
		data := map[string]any{
			"provider":                 cfg.AWS.EffectiveProvider(),
			"default_account":          cfg.AWS.DefaultAccount,
			"default_region":           cfg.AWS.DefaultRegion,
			"session_duration_seconds": cfg.AWS.EffectiveSessionDurationSeconds(),
			"kubeconfig_path":          cfg.AWS.EffectiveKubeconfigPath(),
			"accounts":                 accounts,
			"count":                    len(accounts),
			"enabled_count":            len(cfg.AWS.EnabledAccounts()),
		}
		if o.Verbose {
			data["config_path"] = path
		}
		return print(cmd, o, output.Success("", data))
	}}
	c.AddCommand(list)
	return c
}
