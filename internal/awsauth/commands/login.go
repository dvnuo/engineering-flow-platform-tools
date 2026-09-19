package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	Account string
	Role    string
	Profile string
	Region  string
	Prompt  bool
}

// loginTarget is a fully resolved login request: which provider, which account
// and role, and which AWS CLI profile receives the credentials.
type loginTarget struct {
	Provider    string
	AccountName string
	AccountID   string
	Role        string
	RoleARN     string
	Profile     string
	Region      string
	Configured  bool
}

func (t loginTarget) roleARN() string {
	if t.RoleARN != "" {
		return t.RoleARN
	}
	if t.AccountID != "" && t.Role != "" {
		return fmt.Sprintf("arn:aws:iam::%s:role/%s", t.AccountID, t.Role)
	}
	return ""
}

func roleNameFromARN(arn string) string {
	if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx+1 < len(arn) {
		return arn[idx+1:]
	}
	return ""
}

func accountIDFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 5 && isDigits(parts[4]) {
		return parts[4]
	}
	return ""
}

var notConfiguredFailure = output.Failure("config_missing", "AWS authorization is not configured.", "Set AWS domain, username, and password with aws-auth auth login.", 400)

// checkProviderPrerequisites validates the aws node for the selected provider
// before any account is resolved, so a misconfigured node fails the same way
// regardless of which account was asked for.
func checkProviderPrerequisites(aws config.AWSConfig, provider string) *output.Envelope {
	if !knownProvider(provider) {
		failure := output.Failure("config_error", fmt.Sprintf("unknown aws provider %q", provider), "Set aws.provider to adfs-assume, saml2aws, or assume-role.", 400)
		return &failure
	}
	if aws.Enabled != nil && !*aws.Enabled {
		failure := notConfiguredFailure
		return &failure
	}
	if providerNeedsDirectoryCredentials(provider) {
		if singleLine(aws.Domain) == "" || singleLine(aws.Username) == "" || cleanSecret(aws.Password) == "" {
			failure := notConfiguredFailure
			return &failure
		}
	}
	return nil
}

func targetForAccount(provider string, aws config.AWSConfig, sel accountSelection) loginTarget {
	target := loginTarget{Provider: provider, Configured: sel.Configured}
	if sel.Configured {
		target.AccountName = sel.Account.Name
		target.AccountID = sel.Account.AccountID
		target.Role = sel.Account.Role
		target.RoleARN = sel.Account.RoleARN
		target.Profile = sel.Account.EffectiveProfile()
		if len(sel.Account.Regions) > 0 {
			target.Region = sel.Account.Regions[0]
		}
	} else {
		target.AccountID = sel.Account.AccountID
		target.Profile = defaultAWSAuthProfile
	}
	if target.Region == "" {
		target.Region = aws.DefaultRegion
	}
	// A configured role_arn also fills the fields the exec providers need.
	if target.RoleARN != "" {
		if target.Role == "" {
			target.Role = roleNameFromARN(target.RoleARN)
		}
		if target.AccountID == "" {
			target.AccountID = accountIDFromARN(target.RoleARN)
		}
	}
	return target
}

func resolveLogin(cmd *cobra.Command, aws config.AWSConfig, opts loginOptions) (loginTarget, *output.Envelope) {
	provider := aws.EffectiveProvider()
	if failure := checkProviderPrerequisites(aws, provider); failure != nil {
		return loginTarget{}, failure
	}
	sel, failure := resolveAccount(aws, opts.Account)
	if failure != nil {
		return loginTarget{}, failure
	}
	target := targetForAccount(provider, aws, sel)
	if role := singleLine(opts.Role); role != "" {
		target.Role = role
		target.RoleARN = ""
	}
	if profile := singleLine(opts.Profile); profile != "" {
		target.Profile = profile
	}
	if region := singleLine(opts.Region); region != "" {
		target.Region = region
	}
	if opts.Prompt && !target.Configured && (target.AccountID == "" || target.Role == "") {
		account, role, err := promptAccountRole(cmd, target.AccountID, target.Role)
		if err != nil {
			failure := output.Failure("invalid_args", output.RedactString(err.Error()), "Pass --account and --role when running aws-auth login.", 400)
			return loginTarget{}, &failure
		}
		target.AccountID, target.Role = account, role
	}
	if target.roleARN() == "" {
		failure := output.Failure("invalid_args", "account and role are required for AWS login.", "Pass --account and --role when running aws-auth login.", 400)
		return loginTarget{}, &failure
	}
	if target.Profile == "" {
		target.Profile = defaultAWSAuthProfile
	}
	return target, nil
}

func loginCmd(o *Opts) *cobra.Command {
	var account, role, profile, region string
	var all bool
	verify := true
	c := &cobra.Command{
		Use:   "login",
		Short: "Authorize AWS credentials for a configured account, or for an explicit account id and role.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, cfg, err := loadAWSConfigForRead(o.Config)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check EFP_CONFIG or pass --config.", 400))
			}
			if all {
				return print(cmd, o, runLoginAll(cmd.Context(), o, cfg.AWS, role, region, verify))
			}
			target, failure := resolveLogin(cmd, cfg.AWS, loginOptions{Account: account, Role: role, Profile: profile, Region: region, Prompt: !o.JSON})
			if failure != nil {
				return print(cmd, o, *failure)
			}
			data, failure := runLogin(cmd.Context(), o, cfg.AWS, target, verify)
			if failure != nil {
				return print(cmd, o, *failure)
			}
			if o.Verbose {
				data["config_path"] = path
			}
			return print(cmd, o, output.Success("", data))
		},
	}
	c.Flags().StringVar(&account, "account", "", "Configured account name, or an AWS account id.")
	c.Flags().StringVar(&role, "role", "", "AWS role name; defaults to the configured account's role.")
	c.Flags().StringVar(&profile, "profile", "", "AWS CLI profile to write; defaults to the account name (saml for an ad-hoc account id).")
	c.Flags().StringVar(&region, "region", "", "Region used when verifying the credentials; defaults to the account's first region.")
	c.Flags().BoolVar(&verify, "verify", true, "Verify the credentials with aws sts get-caller-identity after login.")
	c.Flags().BoolVar(&all, "all", false, "Log in to every enabled configured account.")
	return c
}

func runLogin(ctx context.Context, o *Opts, aws config.AWSConfig, target loginTarget, verify bool) (map[string]any, *output.Envelope) {
	prov := providerFor(target.Provider)
	data := map[string]any{
		"provider":   target.Provider,
		"profile":    target.Profile,
		"account":    target.AccountName,
		"account_id": target.AccountID,
		"role":       target.Role,
		"role_arn":   target.roleARN(),
		"region":     target.Region,
	}
	if o.DryRun {
		data["authenticated"] = false
		data["dry_run"] = true
		data["command"] = prov.describe(aws, target)
		return data, nil
	}
	result, failure := prov.login(ctx, o.runner, aws, target)
	if failure != nil {
		return nil, failure
	}
	data["authenticated"] = true
	data["command"] = result.command
	if result.expiresAt != "" {
		data["expires_at"] = result.expiresAt
	}
	if o.Verbose {
		data["provider_binary"] = prov.binary()
	}
	if verify {
		identity, err := verifyIdentity(ctx, o.runner, target.Profile, target.Region)
		switch {
		case err != nil:
			data["verified"] = false
			data["verify_error"] = redactWithSecrets(err.Error(), aws.Password)
		case target.Configured && target.AccountID != "" && identity.Account != target.AccountID:
			data["identity"] = identity.view()
			data["verified"] = false
			data["verify_error"] = fmt.Sprintf("credentials belong to account %s, expected %s", identity.Account, target.AccountID)
		default:
			data["identity"] = identity.view()
			data["verified"] = true
		}
	}
	return data, nil
}

func runLoginAll(ctx context.Context, o *Opts, aws config.AWSConfig, roleOverride, regionOverride string, verify bool) output.Envelope {
	provider := aws.EffectiveProvider()
	if failure := checkProviderPrerequisites(aws, provider); failure != nil {
		return *failure
	}
	enabled := aws.EnabledAccounts()
	if len(enabled) == 0 {
		return output.Failure("config_missing", "no accounts are configured under aws.accounts", "Add accounts to the aws node, or pass --account and --role for a single login.", 400)
	}
	results := make([]map[string]any, 0, len(enabled))
	succeeded, failed := 0, 0
	aborted := ""
	for _, acct := range enabled {
		target := targetForAccount(provider, aws, accountSelection{Account: acct, Configured: true})
		if role := singleLine(roleOverride); role != "" {
			target.Role = role
			target.RoleARN = ""
		}
		if region := singleLine(regionOverride); region != "" {
			target.Region = region
		}
		entry := map[string]any{"account": acct.Name, "account_id": acct.AccountID, "profile": target.Profile}
		if target.roleARN() == "" {
			entry["ok"] = false
			entry["error"] = map[string]any{"code": "invalid_args", "message": "role is not configured for this account"}
			failed++
			results = append(results, entry)
			continue
		}
		data, failure := runLogin(ctx, o, aws, target, verify)
		if failure != nil {
			entry["ok"] = false
			errView := map[string]any{"code": failure.Error.Code, "message": failure.Error.Message}
			if failure.Error.Hint != "" {
				errView["hint"] = failure.Error.Hint
			}
			entry["error"] = errView
			failed++
			results = append(results, entry)
			if failure.Error.Code == "provider_missing" || failure.Error.Code == "config_missing" {
				aborted = failure.Error.Code
				break
			}
			continue
		}
		entry["ok"] = true
		for _, key := range []string{"authenticated", "dry_run", "command", "expires_at", "identity", "verified", "verify_error", "role", "role_arn", "region"} {
			if value, ok := data[key]; ok {
				entry[key] = value
			}
		}
		succeeded++
		results = append(results, entry)
	}
	summary := map[string]any{
		"provider":  provider,
		"dry_run":   o.DryRun,
		"results":   results,
		"succeeded": succeeded,
		"failed":    failed,
		"partial":   succeeded > 0 && failed > 0,
	}
	if aborted != "" {
		summary["aborted"] = aborted
	}
	if succeeded == 0 {
		failure := output.Failure("auth_failed", fmt.Sprintf("all %d accounts failed to authenticate", failed), "Inspect results[].error for each account.", 401)
		if aborted != "" {
			failure = output.Failure(aborted, "authorization aborted before any account succeeded", "Inspect results[].error; the provider or its configuration is unavailable.", 500)
		}
		failure.Data = summary
		return failure
	}
	return output.Success("", summary)
}

type callerIdentity struct {
	UserID  string `json:"UserId"`
	Account string `json:"Account"`
	Arn     string `json:"Arn"`
}

func (id callerIdentity) view() map[string]any {
	return map[string]any{"arn": id.Arn, "account": id.Account, "user_id": id.UserID}
}

// verifyIdentity calls aws sts get-caller-identity for the profile the
// provider just wrote. It never receives the directory password.
func verifyIdentity(ctx context.Context, runner commandRunner, profile, region string) (callerIdentity, error) {
	args := []string{"sts", "get-caller-identity", "--profile", profile, "--output", "json"}
	if region != "" {
		args = append(args, "--region", region)
	}
	result, err := runner.Run(ctx, awsCLICommand, args, stripSecretEnv(os.Environ()))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return callerIdentity{}, errors.New("aws CLI is not installed or not on PATH")
		}
		return callerIdentity{}, fmt.Errorf("aws sts get-caller-identity failed: %s", output.RedactString(err.Error()))
	}
	if result.ExitCode != 0 {
		message := firstNonEmpty(result.Stderr, result.Stdout, fmt.Sprintf("exit %d", result.ExitCode))
		return callerIdentity{}, fmt.Errorf("aws sts get-caller-identity failed: %s", truncateText(output.RedactString(message), 500))
	}
	var identity callerIdentity
	if err := json.Unmarshal([]byte(result.Stdout), &identity); err != nil || identity.Arn == "" {
		return callerIdentity{}, errors.New("aws sts get-caller-identity returned no identity")
	}
	return identity, nil
}
