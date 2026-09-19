package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
)

type providerResult struct {
	command   string
	expiresAt string
}

// loginProvider exchanges the configured identity for AWS credentials in one
// AWS CLI profile. The exec providers shell out through the injected
// commandRunner; assume-role only writes ~/.aws/config.
type loginProvider interface {
	name() string
	binary() string
	describe(aws config.AWSConfig, target loginTarget) string
	login(ctx context.Context, runner commandRunner, aws config.AWSConfig, target loginTarget) (providerResult, *output.Envelope)
}

func knownProvider(name string) bool {
	switch name {
	case config.AWSProviderADFSAssume, config.AWSProviderSAML2AWS, config.AWSProviderAssumeRole:
		return true
	}
	return false
}

func providerNeedsDirectoryCredentials(name string) bool {
	return name != config.AWSProviderAssumeRole
}

func providerFor(name string) loginProvider {
	switch name {
	case config.AWSProviderSAML2AWS:
		return saml2awsProvider{}
	case config.AWSProviderAssumeRole:
		return assumeRoleProvider{}
	default:
		return adfsAssumeProvider{}
	}
}

// runExecProvider runs an external login binary and maps its outcome onto the
// stable envelope codes: provider_missing (binary absent), execution_failed
// (could not run), auth_failed (non-zero exit). The password never appears in
// argv and is scrubbed from any message.
func runExecProvider(ctx context.Context, runner commandRunner, command string, args, env []string, password, installHint string) (providerResult, *output.Envelope) {
	result, err := runner.Run(ctx, command, args, env)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			failure := output.Failure("provider_missing", fmt.Sprintf("%s is not installed or not on PATH", command), installHint, 500)
			return providerResult{}, &failure
		}
		failure := output.Failure("execution_failed", redactWithSecrets(err.Error(), password), installHint, 500)
		return providerResult{}, &failure
	}
	if result.ExitCode != 0 {
		message := firstNonEmpty(result.Stderr, result.Stdout)
		if message == "" {
			message = fmt.Sprintf("authorization provider exited with %d", result.ExitCode)
		}
		failure := output.Failure(
			"auth_failed",
			redactWithSecrets(message, password),
			"Verify the configured AWS domain, username, password, and the supplied account and role.",
			401,
		)
		return providerResult{}, &failure
	}
	return providerResult{command: formatCommand(command, args)}, nil
}

// ---- adfs-assume ------------------------------------------------------------

type adfsAssumeProvider struct{}

func (adfsAssumeProvider) name() string   { return config.AWSProviderADFSAssume }
func (adfsAssumeProvider) binary() string { return adfsAssumeCommand }

func (adfsAssumeProvider) args(aws config.AWSConfig, target loginTarget) []string {
	return []string{
		"--domain", singleLine(aws.Domain),
		"--username", singleLine(aws.Username),
		"--role", target.Role,
		"--account", target.AccountID,
		"--profile", target.Profile,
		"--no-warning", "--display-token", "--jenkins",
	}
}

func (p adfsAssumeProvider) describe(aws config.AWSConfig, target loginTarget) string {
	return formatCommand(adfsAssumeCommand, p.args(aws, target))
}

func (p adfsAssumeProvider) login(ctx context.Context, runner commandRunner, aws config.AWSConfig, target loginTarget) (providerResult, *output.Envelope) {
	if target.AccountID == "" || target.Role == "" {
		failure := output.Failure("invalid_args", "account id and role are required for adfs-assume", "Pass --account and --role, or configure account_id and role for the account.", 400)
		return providerResult{}, &failure
	}
	password := cleanSecret(aws.Password)
	return runExecProvider(ctx, runner, adfsAssumeCommand, p.args(aws, target), withSecretEnv(os.Environ(), "AD_PASS", password), password, "Ensure adfs-assume is installed and available on PATH.")
}

// ---- saml2aws ---------------------------------------------------------------

type saml2awsProvider struct{}

func (saml2awsProvider) name() string   { return config.AWSProviderSAML2AWS }
func (saml2awsProvider) binary() string { return saml2awsCommand }

func saml2awsUsername(aws config.AWSConfig) string {
	username := singleLine(aws.Username)
	domain := singleLine(aws.Domain)
	if domain != "" && !strings.ContainsAny(username, `\@`) {
		return domain + `\` + username
	}
	return username
}

func (saml2awsProvider) args(aws config.AWSConfig, target loginTarget) []string {
	return []string{
		"login",
		"--idp-provider", "ADFS",
		"--url", aws.IdpURL,
		"--username", saml2awsUsername(aws),
		"--role", target.roleARN(),
		"--profile", target.Profile,
		"--skip-prompt",
		"--session-duration", strconv.Itoa(aws.EffectiveSessionDurationSeconds()),
		"--disable-keychain",
	}
}

func (p saml2awsProvider) describe(aws config.AWSConfig, target loginTarget) string {
	return formatCommand(saml2awsCommand, p.args(aws, target))
}

func (p saml2awsProvider) login(ctx context.Context, runner commandRunner, aws config.AWSConfig, target loginTarget) (providerResult, *output.Envelope) {
	if aws.IdpURL == "" {
		failure := output.Failure("config_missing", "aws.idp_url is required for the saml2aws provider", "Set aws.idp_url to the ADFS IdP-initiated sign-on URL (EFP_AWS_IDP_URL in managed runtimes).", 400)
		return providerResult{}, &failure
	}
	if target.roleARN() == "" {
		failure := output.Failure("invalid_args", "account id and role are required to build the role ARN", "Pass --account and --role, or configure account_id and role (or role_arn) for the account.", 400)
		return providerResult{}, &failure
	}
	password := cleanSecret(aws.Password)
	result, failure := runExecProvider(ctx, runner, saml2awsCommand, p.args(aws, target), withSecretEnv(os.Environ(), "SAML2AWS_PASSWORD", password), password, "Ensure saml2aws is installed and available on PATH.")
	if failure != nil {
		return providerResult{}, failure
	}
	if expires, ok := profileExpiryFromFile(awsCredentialsFilePath(), target.Profile); ok {
		result.expiresAt = expires.UTC().Format(time.RFC3339)
	}
	return result, nil
}

// ---- assume-role ------------------------------------------------------------

type assumeRoleProvider struct{}

func (assumeRoleProvider) name() string   { return config.AWSProviderAssumeRole }
func (assumeRoleProvider) binary() string { return "" }

func sourceProfileName(aws config.AWSConfig) string {
	if aws.SourceProfile != "" {
		return aws.SourceProfile
	}
	return "default"
}

func (assumeRoleProvider) describe(aws config.AWSConfig, target loginTarget) string {
	return fmt.Sprintf("write [profile %s] role_arn=%s source_profile=%s to %s", target.Profile, target.roleARN(), sourceProfileName(aws), awsConfigFilePath())
}

func (p assumeRoleProvider) login(ctx context.Context, runner commandRunner, aws config.AWSConfig, target loginTarget) (providerResult, *output.Envelope) {
	arn := target.roleARN()
	if arn == "" {
		failure := output.Failure("invalid_args", "account id and role (or role_arn) are required for assume-role", "Configure account_id and role, or role_arn, for the account.", 400)
		return providerResult{}, &failure
	}
	keys := []string{"role_arn", "source_profile"}
	values := map[string]string{"role_arn": arn, "source_profile": sourceProfileName(aws)}
	if target.Region != "" {
		keys = append(keys, "region")
		values["region"] = target.Region
	}
	if err := upsertAWSConfigProfile(awsConfigFilePath(), target.Profile, keys, values); err != nil {
		failure := output.Failure("config_error", output.RedactString(err.Error()), "Check that the AWS config file directory is writable.", 500)
		return providerResult{}, &failure
	}
	return providerResult{command: p.describe(aws, target)}, nil
}
