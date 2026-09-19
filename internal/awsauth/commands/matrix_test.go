package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedRunner answers each external command through a handler so a test can
// make the provider fail for one account while STS succeeds for another.
type scriptedRunner struct {
	calls   []fakeCall
	handler func(command string, args []string) (commandResult, error)
}

func (r *scriptedRunner) Run(ctx context.Context, command string, args []string, env []string) (commandResult, error) {
	r.calls = append(r.calls, fakeCall{command: command, args: append([]string{}, args...), env: append([]string{}, env...)})
	if r.handler != nil {
		return r.handler(command, args)
	}
	return commandResult{}, nil
}

func identityJSON(account string) string {
	return `{"UserId":"AROAEXAMPLE:session","Account":"` + account + `","Arn":"arn:aws:sts::` + account + `:assumed-role/ADFS-ReadOnly/session"}`
}

func stsHandler(account string) func(string, []string) (commandResult, error) {
	return func(command string, args []string) (commandResult, error) {
		if command == "aws" && len(args) >= 2 && args[0] == "sts" {
			return commandResult{Stdout: identityJSON(account)}, nil
		}
		return commandResult{}, nil
	}
}

func runJSONWith(t *testing.T, runner commandRunner, args ...string) map[string]any {
	t.Helper()
	cmd := NewRootWithRunner(runner)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\n%s", err, out.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	return obj
}

func isolateAWSFiles(t *testing.T) (credentialsPath, configPath string) {
	t.Helper()
	dir := t.TempDir()
	credentialsPath = filepath.Join(dir, "credentials")
	configPath = filepath.Join(dir, "config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsPath)
	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("KUBECONFIG", "")
	return credentialsPath, configPath
}

const matrixConfig = `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
  default_region: eu-west-1
  accounts:
    - name: cps-dev
      account_id: "111111111111"
      role: ADFS-ReadOnly
      regions: [ap-east-1, eu-west-1]
    - name: dcc-dev
      account_id: "222222222222"
      role: ADFS-ReadOnly
    - name: retired
      account_id: "333333333333"
      role: ADFS-ReadOnly
      enabled: false
`

func argsText(call fakeCall) string {
	return call.command + " " + strings.Join(call.args, " ")
}

func errorCode(t *testing.T, obj map[string]any) string {
	t.Helper()
	if obj["ok"] == true {
		t.Fatalf("expected failure: %#v", obj)
	}
	errObj, _ := obj["error"].(map[string]any)
	if errObj == nil {
		t.Fatalf("missing error: %#v", obj)
	}
	code, _ := errObj["code"].(string)
	return code
}

func TestLoginResolvesConfiguredAccountByName(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: stsHandler("111111111111")}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--account", "cps-dev", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected provider + sts calls, got %#v", runner.calls)
	}
	provider := argsText(runner.calls[0])
	for _, token := range []string{"adfs-assume ", "--account 111111111111", "--role ADFS-ReadOnly", "--profile cps-dev", "--domain HBEU", "--username aws-user"} {
		if !strings.Contains(provider, token) {
			t.Fatalf("missing %q in provider call %q", token, provider)
		}
	}
	if got := envValue(runner.calls[0].env, "AD_PASS"); got != "aws-password" {
		t.Fatalf("AD_PASS not passed: %q", got)
	}
	sts := argsText(runner.calls[1])
	if sts != "aws sts get-caller-identity --profile cps-dev --output json --region ap-east-1" {
		t.Fatalf("unexpected verification call %q", sts)
	}
	if envValue(runner.calls[1].env, "AD_PASS") != "" {
		t.Fatalf("password must not reach the aws CLI: %#v", runner.calls[1].env)
	}
	data := obj["data"].(map[string]any)
	if data["profile"] != "cps-dev" || data["account_id"] != "111111111111" || data["region"] != "ap-east-1" || data["verified"] != true {
		t.Fatalf("unexpected login data: %#v", data)
	}
	identity := data["identity"].(map[string]any)
	if identity["account"] != "111111111111" || !strings.HasPrefix(identity["arn"].(string), "arn:aws:sts::111111111111:") {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestLoginUsesDefaultAccountWhenOmitted(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig+"  default_account: dcc-dev\n")
	runner := &scriptedRunner{handler: stsHandler("222222222222")}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	provider := argsText(runner.calls[0])
	if !strings.Contains(provider, "--account 222222222222") || !strings.Contains(provider, "--profile dcc-dev") {
		t.Fatalf("default account not used: %q", provider)
	}
	data := obj["data"].(map[string]any)
	if data["region"] != "eu-west-1" {
		t.Fatalf("account without regions must fall back to default_region: %#v", data)
	}
}

func TestLoginAccountRequiredWhenSeveralConfigured(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--json")
	if code := errorCode(t, obj); code != "account_required" {
		t.Fatalf("expected account_required, got %s: %#v", code, obj)
	}
	candidates := obj["data"].(map[string]any)["candidates"].([]any)
	if len(candidates) != 2 || candidates[0] != "cps-dev" || candidates[1] != "dcc-dev" {
		t.Fatalf("unexpected candidates: %#v", candidates)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("provider must not run: %#v", runner.calls)
	}
}

func TestLoginRejectsUnknownAccountNameButAllowsAdHocID(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: stsHandler("999999999999")}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--account", "nope", "--json")
	if code := errorCode(t, obj); code != "unknown_account" {
		t.Fatalf("expected unknown_account, got %s", code)
	}
	obj = runJSONWith(t, runner, "--config", cfg, "login", "--account", "999999999999", "--json")
	if code := errorCode(t, obj); code != "invalid_args" {
		t.Fatalf("ad-hoc account still needs --role: %#v", obj)
	}
	obj = runJSONWith(t, runner, "--config", cfg, "login", "--account", "999999999999", "--role", "Ops", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	provider := argsText(runner.calls[len(runner.calls)-2])
	if !strings.Contains(provider, "--account 999999999999 --profile saml") || !strings.Contains(provider, "--role Ops") {
		t.Fatalf("ad-hoc account must use the saml profile: %q", provider)
	}
}

func TestLoginVerifyMismatchIsReported(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: stsHandler("222222222222")}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--account", "cps-dev", "--json")
	if obj["ok"] != true {
		t.Fatalf("login itself succeeded, expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["verified"] != false || !strings.Contains(data["verify_error"].(string), "expected 111111111111") {
		t.Fatalf("mismatch not reported: %#v", data)
	}
}

func TestLoginVerifyCanBeDisabled(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--account", "cps-dev", "--verify=false", "--json")
	if obj["ok"] != true || len(runner.calls) != 1 {
		t.Fatalf("expected a single provider call: %#v %#v", obj, runner.calls)
	}
	if _, ok := obj["data"].(map[string]any)["verified"]; ok {
		t.Fatalf("verified must be absent when verification is disabled: %#v", obj)
	}
}

func TestLoginAllReportsPartialFailure(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: func(command string, args []string) (commandResult, error) {
		joined := strings.Join(args, " ")
		if command == "adfs-assume" && strings.Contains(joined, "--account 222222222222") {
			return commandResult{ExitCode: 1, Stderr: "access denied for aws-password"}, nil
		}
		if command == "aws" {
			return commandResult{Stdout: identityJSON("111111111111")}, nil
		}
		return commandResult{}, nil
	}}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--all", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok with partial results: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["partial"] != true || data["succeeded"] != float64(1) || data["failed"] != float64(1) {
		t.Fatalf("unexpected summary: %#v", data)
	}
	results := data["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("expected two results (disabled account skipped): %#v", results)
	}
	first := results[0].(map[string]any)
	second := results[1].(map[string]any)
	if first["ok"] != true || first["account"] != "cps-dev" || first["verified"] != true {
		t.Fatalf("first result: %#v", first)
	}
	if second["ok"] != false || second["error"].(map[string]any)["code"] != "auth_failed" {
		t.Fatalf("second result: %#v", second)
	}
	raw, _ := json.Marshal(obj)
	if strings.Contains(string(raw), "aws-password") {
		t.Fatalf("password leaked: %s", raw)
	}
}

func TestLoginAllAbortsWhenProviderMissing(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: func(command string, args []string) (commandResult, error) {
		return commandResult{ExitCode: 1}, &exec.Error{Name: command, Err: exec.ErrNotFound}
	}}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--all", "--json")
	if code := errorCode(t, obj); code != "provider_missing" {
		t.Fatalf("expected provider_missing, got %s: %#v", code, obj)
	}
	data := obj["data"].(map[string]any)
	if data["aborted"] != "provider_missing" || len(data["results"].([]any)) != 1 {
		t.Fatalf("expected abort after the first account: %#v", data)
	}
}

func TestLoginDryRunDescribesProviderWithoutRunning(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{}
	obj := runJSONWith(t, runner, "--config", cfg, "--dry-run", "login", "--account", "cps-dev", "--json")
	if obj["ok"] != true || len(runner.calls) != 0 {
		t.Fatalf("dry-run must not run anything: %#v %#v", obj, runner.calls)
	}
	data := obj["data"].(map[string]any)
	if data["dry_run"] != true || data["authenticated"] != false || !strings.Contains(data["command"].(string), "adfs-assume") {
		t.Fatalf("unexpected dry-run data: %#v", data)
	}
}

func TestSaml2awsProviderBuildsRoleARNAndPassesPasswordByEnv(t *testing.T) {
	credentialsPath, _ := isolateAWSFiles(t)
	writeFile(t, credentialsPath, "[cps-dev]\naws_access_key_id = AKIAEXAMPLE\nx_security_token_expires = 2030-01-01T00:00:00Z\n")
	cfg := writeConfig(t, strings.Replace(matrixConfig, "  enabled: true\n", "  enabled: true\n  provider: saml2aws\n  idp_url: https://adfs.example.test/adfs/ls/IdpInitiatedSignOn.aspx?loginToRp=urn:amazon:webservices\n", 1))
	runner := &scriptedRunner{}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--account", "cps-dev", "--verify=false", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	call := runner.calls[0]
	if call.command != "saml2aws" {
		t.Fatalf("expected saml2aws, got %s", call.command)
	}
	joined := strings.Join(call.args, " ")
	for _, token := range []string{
		"login --idp-provider ADFS",
		"--url https://adfs.example.test/adfs/ls/IdpInitiatedSignOn.aspx?loginToRp=urn:amazon:webservices",
		`--username HBEU\aws-user`,
		"--role arn:aws:iam::111111111111:role/ADFS-ReadOnly",
		"--profile cps-dev",
		"--skip-prompt",
		"--session-duration 3600",
		"--disable-keychain",
	} {
		if !strings.Contains(joined, token) {
			t.Fatalf("missing %q in %q", token, joined)
		}
	}
	if strings.Contains(joined, "aws-password") {
		t.Fatalf("password leaked into args: %q", joined)
	}
	if envValue(call.env, "SAML2AWS_PASSWORD") != "aws-password" || envValue(call.env, "AD_PASS") != "" {
		t.Fatalf("password must reach saml2aws only through SAML2AWS_PASSWORD: %#v", call.env)
	}
	data := obj["data"].(map[string]any)
	if data["expires_at"] != "2030-01-01T00:00:00Z" || data["provider"] != "saml2aws" {
		t.Fatalf("expiry not read from the credentials file: %#v", data)
	}
}

func TestSaml2awsProviderRequiresIdpURL(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, strings.Replace(matrixConfig, "  enabled: true\n", "  enabled: true\n  provider: saml2aws\n", 1))
	runner := &scriptedRunner{}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--account", "cps-dev", "--json")
	if code := errorCode(t, obj); code != "config_missing" || len(runner.calls) != 0 {
		t.Fatalf("expected config_missing without running anything: %s %#v", code, runner.calls)
	}
}

func TestAssumeRoleProviderWritesConfigProfile(t *testing.T) {
	_, configPath := isolateAWSFiles(t)
	writeFile(t, configPath, "[default]\nregion = us-east-1\n")
	cfg := writeConfig(t, `
version: 1
aws:
  enabled: true
  provider: assume-role
  source_profile: base
  accounts:
    - name: cps-dev
      account_id: "111111111111"
      role: ADFS-ReadOnly
      regions: [ap-east-1]
`)
	runner := &scriptedRunner{handler: stsHandler("111111111111")}
	obj := runJSONWith(t, runner, "--config", cfg, "login", "--account", "cps-dev", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	if len(runner.calls) != 1 || runner.calls[0].command != "aws" {
		t.Fatalf("assume-role must only call sts for verification: %#v", runner.calls)
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, token := range []string{"[default]\nregion = us-east-1", "[profile cps-dev]", "role_arn = arn:aws:iam::111111111111:role/ADFS-ReadOnly", "source_profile = base", "region = ap-east-1"} {
		if !strings.Contains(text, token) {
			t.Fatalf("missing %q in config file:\n%s", token, text)
		}
	}
	data := obj["data"].(map[string]any)
	if data["verified"] != true || data["provider"] != "assume-role" {
		t.Fatalf("unexpected data: %#v", data)
	}
}

func TestStatusReportsProfilesAndExpiry(t *testing.T) {
	credentialsPath, configPath := isolateAWSFiles(t)
	writeFile(t, credentialsPath, strings.Join([]string{
		"[cps-dev]", "aws_access_key_id = AKIAONE", "x_security_token_expires = 2000-01-01T00:00:00Z", "x_principal_arn = arn:aws:iam::111111111111:role/ADFS-ReadOnly",
		"[dcc-dev]", "aws_access_key_id = AKIATWO", "x_security_token_expires = 2999-01-01T00:00:00Z",
		"[other]", "aws_access_key_id = AKIATHREE", "",
	}, "\n"))
	writeFile(t, configPath, "[profile chained]\nrole_arn = arn:aws:iam::444444444444:role/Ops\nsource_profile = default\n")
	cfg := writeConfig(t, matrixConfig)
	obj := runJSONWith(t, &scriptedRunner{}, "--config", cfg, "status", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["present_count"] != float64(3) || data["expired_count"] != float64(1) {
		t.Fatalf("unexpected counts: %#v", data)
	}
	byProfile := map[string]map[string]any{}
	for _, item := range data["profiles"].([]any) {
		entry := item.(map[string]any)
		byProfile[entry["profile"].(string)] = entry
	}
	if byProfile["cps-dev"]["expired"] != true || byProfile["cps-dev"]["account_id"] != "111111111111" || byProfile["cps-dev"]["principal_arn"] == nil {
		t.Fatalf("cps-dev entry: %#v", byProfile["cps-dev"])
	}
	if byProfile["dcc-dev"]["expired"] != false || byProfile["dcc-dev"]["region"] != "eu-west-1" {
		t.Fatalf("dcc-dev entry: %#v", byProfile["dcc-dev"])
	}
	if byProfile["other"]["source"] != "credentials" || byProfile["other"]["present"] != true {
		t.Fatalf("unmapped profile must still be listed: %#v", byProfile["other"])
	}
	if _, listed := byProfile["retired"]; listed {
		t.Fatalf("disabled accounts must not be listed: %#v", byProfile)
	}
	raw, _ := json.Marshal(obj)
	if strings.Contains(string(raw), "AKIAONE") {
		t.Fatalf("access key ids must not be echoed: %s", raw)
	}
}

func TestStatusVerifyCallsSTSForPresentProfiles(t *testing.T) {
	credentialsPath, _ := isolateAWSFiles(t)
	writeFile(t, credentialsPath, "[cps-dev]\naws_access_key_id = AKIAONE\n")
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: stsHandler("111111111111")}
	obj := runJSONWith(t, runner, "--config", cfg, "status", "--verify", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	if len(runner.calls) != 1 || argsText(runner.calls[0]) != "aws sts get-caller-identity --profile cps-dev --output json --region ap-east-1" {
		t.Fatalf("expected one verification call for the present profile: %#v", runner.calls)
	}
	for _, item := range obj["data"].(map[string]any)["profiles"].([]any) {
		entry := item.(map[string]any)
		if entry["profile"] == "cps-dev" && entry["verified"] != true {
			t.Fatalf("cps-dev must verify: %#v", entry)
		}
		if entry["profile"] == "dcc-dev" {
			if _, ok := entry["verified"]; ok {
				t.Fatalf("absent profile must not be verified: %#v", entry)
			}
		}
	}
}

func TestEksKubeconfigWritesContextAndChecksAccess(t *testing.T) {
	isolateAWSFiles(t)
	kubeconfig := filepath.Join(t.TempDir(), "kube", "config")
	t.Setenv("KUBECONFIG", kubeconfig)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: func(command string, args []string) (commandResult, error) {
		if command == "kubectl" {
			return commandResult{Stdout: "yes\n"}, nil
		}
		return commandResult{}, nil
	}}
	obj := runJSONWith(t, runner, "--config", cfg, "eks", "kubeconfig", "--account", "cps-dev", "--cluster", "cps-dev-eks", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected update-kubeconfig + kubectl calls: %#v", runner.calls)
	}
	update := argsText(runner.calls[0])
	if update != "aws eks update-kubeconfig --name cps-dev-eks --region ap-east-1 --profile cps-dev --alias cps-dev/cps-dev-eks --kubeconfig "+kubeconfig {
		t.Fatalf("unexpected update call %q", update)
	}
	verify := argsText(runner.calls[1])
	if verify != "kubectl --kubeconfig "+kubeconfig+" --context cps-dev/cps-dev-eks auth can-i list pods --all-namespaces" {
		t.Fatalf("unexpected verify call %q", verify)
	}
	data := obj["data"].(map[string]any)
	if data["context"] != "cps-dev/cps-dev-eks" || data["can_list_pods"] != true || data["kubectl_missing"] != false || data["kubeconfig"] != kubeconfig {
		t.Fatalf("unexpected data: %#v", data)
	}
	if _, err := os.Stat(filepath.Dir(kubeconfig)); err != nil {
		t.Fatalf("kubeconfig directory must be created: %v", err)
	}
}

func TestEksKubeconfigToleratesMissingKubectl(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: func(command string, args []string) (commandResult, error) {
		if command == "kubectl" {
			return commandResult{ExitCode: 1}, &exec.Error{Name: command, Err: exec.ErrNotFound}
		}
		return commandResult{}, nil
	}}
	obj := runJSONWith(t, runner, "--config", cfg, "eks", "kubeconfig", "--account", "cps-dev", "--cluster", "c1", "--region", "eu-west-1", "--kubeconfig", filepath.Join(t.TempDir(), "config"), "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["kubectl_missing"] != true || data["can_list_pods"] != nil || data["region"] != "eu-west-1" {
		t.Fatalf("unexpected data: %#v", data)
	}
}

func TestEksKubeconfigRequiresClusterAndRegion(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{}
	if code := errorCode(t, runJSONWith(t, runner, "--config", cfg, "eks", "kubeconfig", "--account", "cps-dev", "--json")); code != "invalid_args" {
		t.Fatalf("expected invalid_args for missing cluster, got %s", code)
	}
	noRegion := writeConfig(t, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
  accounts:
    - name: solo
      account_id: "555555555555"
      role: ADFS-ReadOnly
`)
	if code := errorCode(t, runJSONWith(t, runner, "--config", noRegion, "eks", "kubeconfig", "--cluster", "c1", "--json")); code != "invalid_args" {
		t.Fatalf("expected invalid_args for missing region, got %s", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("nothing must run: %#v", runner.calls)
	}
}

func TestEksListParsesClustersAndMapsExpiredSessions(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	runner := &scriptedRunner{handler: func(command string, args []string) (commandResult, error) {
		return commandResult{Stdout: `{"clusters":["cps-dev-eks","cps-dev-batch"]}`}, nil
	}}
	obj := runJSONWith(t, runner, "--config", cfg, "eks", "list", "--account", "cps-dev", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["count"] != float64(2) || data["profile"] != "cps-dev" || argsText(runner.calls[0]) != "aws eks list-clusters --profile cps-dev --region ap-east-1 --output json" {
		t.Fatalf("unexpected list data: %#v %#v", data, runner.calls)
	}

	expired := &scriptedRunner{handler: func(command string, args []string) (commandResult, error) {
		return commandResult{ExitCode: 255, Stderr: "An error occurred (ExpiredToken) when calling the ListClusters operation: The security token included in the request is expired"}, nil
	}}
	obj = runJSONWith(t, expired, "--config", cfg, "eks", "list", "--account", "cps-dev", "--json")
	if code := errorCode(t, obj); code != "session_expired" {
		t.Fatalf("expected session_expired, got %s: %#v", code, obj)
	}
}

func TestAccountListShowsMatrix(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig+"  default_account: cps-dev\n")
	obj := runJSONWith(t, &scriptedRunner{}, "--config", cfg, "account", "list", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["count"] != float64(3) || data["enabled_count"] != float64(2) || data["provider"] != "adfs-assume" || data["default_account"] != "cps-dev" {
		t.Fatalf("unexpected summary: %#v", data)
	}
	first := data["accounts"].([]any)[0].(map[string]any)
	if first["profile"] != "cps-dev" || first["default"] != true || first["role_arn"] != "arn:aws:iam::111111111111:role/ADFS-ReadOnly" || len(first["regions"].([]any)) != 2 {
		t.Fatalf("unexpected first account: %#v", first)
	}
	third := data["accounts"].([]any)[2].(map[string]any)
	if third["enabled"] != false {
		t.Fatalf("disabled account must be reported as disabled: %#v", third)
	}
	raw, _ := json.Marshal(obj)
	if strings.Contains(string(raw), "aws-password") {
		t.Fatalf("password leaked: %s", raw)
	}
}

func TestAuthLoginPreservesAccountMatrix(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	obj := runJSONInput(t, &fakeRunner{}, "new-password\n", "--config", cfg, "auth", "login", "--domain", "HBEU", "--username", "new-user", "--password-stdin", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, token := range []string{"new-user", "cps-dev", "111111111111", "ADFS-ReadOnly", "ap-east-1"} {
		if !strings.Contains(text, token) {
			t.Fatalf("auth login must keep the account matrix; missing %q:\n%s", token, text)
		}
	}
}

func TestAuthStatusReportsProviderAndAccounts(t *testing.T) {
	isolateAWSFiles(t)
	cfg := writeConfig(t, matrixConfig)
	obj := runJSONWith(t, &scriptedRunner{}, "--config", cfg, "auth", "status", "--json")
	data := obj["data"].(map[string]any)
	if data["configured"] != true || data["provider"] != "adfs-assume" || data["account_count"] != float64(2) {
		t.Fatalf("unexpected auth status: %#v", data)
	}
	raw, _ := json.Marshal(obj)
	if strings.Contains(string(raw), "aws-password") {
		t.Fatalf("password leaked: %s", raw)
	}
}

func TestCommandsExposeMatrixCommands(t *testing.T) {
	obj := runJSONWith(t, &scriptedRunner{}, "commands", "--json")
	names := map[string]bool{}
	for _, item := range obj["data"].(map[string]any)["commands"].([]any) {
		names[item.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"login", "account.list", "status", "eks.list", "eks.kubeconfig", "auth.login", "auth.status"} {
		if !names[want] {
			t.Fatalf("missing %s in commands: %#v", want, names)
		}
	}
	schema := runJSONWith(t, &scriptedRunner{}, "schema", "eks.kubeconfig", "--json")
	if schema["ok"] != true {
		t.Fatalf("schema failed: %#v", schema)
	}
}

func TestParseExpiryAcceptsCommonLayouts(t *testing.T) {
	for _, raw := range []string{"2030-01-01T00:00:00Z", "2030-01-01T00:00:00+08:00", "2030-01-01 00:00:00", "1893456000"} {
		if _, ok := parseExpiry(raw); !ok {
			t.Fatalf("expected %q to parse", raw)
		}
	}
	if _, ok := parseExpiry("soon"); ok {
		t.Fatal("garbage must not parse")
	}
}
