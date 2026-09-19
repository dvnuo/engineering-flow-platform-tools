package commands

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/nexus"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

const product = "nexus"

type Opts struct {
	Instance, Config, Format   string
	JSON, Verbose, DryRun, Yes bool
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{Use: product, SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "Configured nexus instance name.")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview a request without sending it.")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "Confirm destructive or service-affecting operations.")
	c.AddCommand(instanceCmd(o), authCmd(o), repoCmd(o), componentCmd(o), assetCmd(o), apiCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: product,
		Binary:  product,
		Short:   "Search Nexus Repository 3 components and assets",
		Long: strings.TrimSpace(`nexus is a terminal-invoked CLI for agents that need read-only, JSON-first access to Sonatype Nexus Repository 3: repositories, component and asset search, component/asset metadata, and artifact downloads.

It never uploads, deletes, or administers anything on the repository manager; the only writes are instance and auth commands that edit the local EFP config. Search and list results are paged: follow continuation_token with --continuation or pass --all.

Configuration uses the shared EFP config from environment variables injected by managed runtimes (for example EFP_NEXUS_DEFAULT_INSTANCE, EFP_NEXUS_INSTANCES_0_BASE_URL) or the config file, normally ~/.efp/config.yaml (local), under the nexus node. An instance without an auth block is used anonymously.`),
		Examples: []string{
			`nexus repo list --json`,
			`nexus component search --repository maven-releases --maven-group-id com.example --maven-artifact-id app --version 1.4.2 --json`,
			`nexus asset search --repository docker-hosted --docker-image-name payments/api --docker-image-tag 2.3.0 --json`,
			`nexus asset download bWF2ZW4tcmVsZWFzZXM6MTVkYWJmZDA1MTIzYWM1MTIzNGY1NjEyMzQ1Njc4OTA --output app-1.4.2.jar --json`,
			`nexus schema component.search --json`,
			`nexus help llm --json`,
		},
		Instructions: "copy cmd/nexus/nexus-cli.instructions.md to ~/.copilot/instructions/nexus-cli.instructions.md.",
		Groups: map[string]string{
			"instance":  "Manage configured Nexus instances.",
			"auth":      "Manage Nexus credentials stored in the EFP config.",
			"repo":      "List and inspect Nexus repositories.",
			"component": "Search, list, and inspect Nexus components.",
			"asset":     "Search, list, inspect, and download Nexus assets.",
			"api":       "Call raw read-only Nexus REST API paths on the selected instance.",
		},
	})
	return c
}

func fmtOut(o *Opts) string {
	if o.JSON {
		return "json"
	}
	if o.Format != "" {
		return strings.ToLower(o.Format)
	}
	return "table"
}

func print(cmd *cobra.Command, o *Opts, e output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), e)
}

func loadCfg(o *Opts) (config.RootConfig, error) {
	cfg, _, err := config.LoadShared(o.Config)
	return cfg, err
}

func saveCfg(o *Opts, cfg config.RootConfig) error {
	return config.SaveShared(o.Config, cfg)
}

func configMissing(err error) output.Envelope {
	return output.Failure("config_missing", httpclient.SanitizeErrorText(err.Error()), "Create ~/.efp/config.yaml, pass --config <path>, or run "+product+" instance add.", 404)
}

func saveFailure(err error) output.Envelope {
	if errors.Is(err, config.ErrEnvManaged) {
		return output.Failure("config_env_managed", "config comes from environment variables; pass --config to write a file", "Managed runtimes inject EFP_NEXUS_* variables; instance and auth writes need an explicit --config <path>.", 400)
	}
	return output.Failure("config_error", httpclient.SanitizeErrorText(err.Error()), "", 500)
}

func loadCtx(cmd *cobra.Command, o *Opts) (*nexus.Context, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return nil, &configLoadError{err: err}
	}
	cx, err := nexus.NewContext(cfg, o.Instance, "")
	if err != nil {
		return nil, err
	}
	if o.Verbose {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s verbose: instance=%s base_url=%s rest_path=%s auth=%s\n", product, cx.Inst.Name, output.RedactString(cx.Inst.BaseURL), cx.Inst.RESTPath, nexus.AuthLabel(cx.Inst.Auth))
	}
	return cx, nil
}

// configLoadError marks a failure to read the config file so envelopeError can
// report config_missing instead of a generic server error.
type configLoadError struct{ err error }

func (e *configLoadError) Error() string { return e.err.Error() }

func envelopeError(err error, fallbackCode string) output.Envelope {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		return output.Failure(httpErr.Code, httpErr.Message, httpErr.Hint, httpErr.Status)
	}
	var loadErr *configLoadError
	if errors.As(err, &loadErr) {
		return configMissing(loadErr.err)
	}
	msg := httpclient.SanitizeErrorText(err.Error())
	if isStableErrorCode(msg) {
		message, hint := stableErrorText(msg)
		return output.Failure(msg, message, hint, 400)
	}
	if fallbackCode == "" {
		fallbackCode = "server_error"
	}
	return output.Failure(fallbackCode, msg, "", 500)
}

func isStableErrorCode(code string) bool {
	switch code {
	case "config_missing", "config_error", "no_instance_configured", "instance_required", "ambiguous_instance", "instance_url_mismatch", "invalid_args", "not_found", "not_supported", "auth_failed", "permission_denied", "rate_limited", "conflict", "network_error", "server_error":
		return true
	default:
		return false
	}
}

func stableErrorText(code string) (string, string) {
	switch code {
	case "no_instance_configured":
		return "no nexus instance is configured", "Run " + product + " instance add <name> --base-url <url> (add --username with --password-stdin unless anonymous reads are allowed), or pass --config <path>."
	case "instance_required":
		return "the nexus instance could not be selected", "Pass --instance <name> or set a default with " + product + " instance default <name>; " + product + " instance list --json shows the configured names."
	case "config_error":
		return "the selected nexus instance has an incomplete or invalid configuration", "Check auth.type, auth.username and the secret field (password, api_key, or token) and ca_cert; remove the auth block entirely for anonymous access."
	default:
		return code, ""
	}
}

func authFromFlags(cmd *cobra.Command) (config.AuthConfig, bool, error) {
	username, _ := cmd.Flags().GetString("username")
	authType, _ := cmd.Flags().GetString("auth-type")
	auth := config.AuthConfig{Type: authType, Username: username}
	provided := strings.TrimSpace(username) != "" || strings.TrimSpace(authType) != ""
	if mustB(cmd, "password-stdin") {
		provided = true
		secret, _ := io.ReadAll(cmd.InOrStdin())
		auth.Password = strings.TrimRight(string(secret), "\r\n")
	}
	if mustB(cmd, "api-key-stdin") {
		provided = true
		secret, _ := io.ReadAll(cmd.InOrStdin())
		auth.APIKey = strings.TrimRight(string(secret), "\r\n")
	}
	if mustB(cmd, "token-stdin") {
		provided = true
		secret, _ := io.ReadAll(cmd.InOrStdin())
		auth.Token = strings.TrimRight(string(secret), "\r\n")
	}
	if !provided {
		return config.AuthConfig{}, false, nil
	}
	auth.NormalizeType()
	switch auth.Type {
	case "basic_password":
		if auth.Username == "" || auth.Password == "" {
			return auth, true, fmt.Errorf("invalid_args")
		}
	case "basic_api_key":
		if auth.Username == "" || auth.APIKey == "" {
			return auth, true, fmt.Errorf("invalid_args")
		}
	case "bearer_token":
		if auth.Token == "" {
			return auth, true, fmt.Errorf("invalid_args")
		}
	default:
		return auth, true, fmt.Errorf("invalid_args")
	}
	return auth, true, nil
}

const authHint = "Use --username with --password-stdin (Nexus password or user token passcode), --username with --api-key-stdin (user token name and passcode as basic auth), or --token-stdin for a bearer token."

func addAuthFlags(cmd *cobra.Command) {
	cmd.Flags().String("username", "", "Nexus username, or the user token name code, for basic authentication.")
	cmd.Flags().String("auth-type", "", "Authentication type: basic_password, basic_api_key, bearer_token, or alias.")
	cmd.Flags().Bool("password-stdin", false, "Read the Nexus password from stdin.")
	cmd.Flags().Bool("api-key-stdin", false, "Read the Nexus user token passcode from stdin; it is sent as HTTP basic auth with --username.")
	cmd.Flags().Bool("token-stdin", false, "Read a bearer token from stdin.")
}

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, configMissing(err))
		}
		instances := make([]config.InstanceConfig, 0, len(cfg.Nexus.Instances))
		for _, in := range cfg.Nexus.Instances {
			in = nexus.WithDefaults(in)
			in.Auth = config.RedactAuth(in.Auth)
			instances = append(instances, in)
		}
		return print(cmd, o, output.Success("", map[string]any{"instances": instances, "default_instance": cfg.Nexus.DefaultInstance}))
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, configMissing(err))
		}
		for _, in := range cfg.Nexus.Instances {
			if in.Name == args[0] {
				in = nexus.WithDefaults(in)
				in.Auth = config.RedactAuth(in.Auth)
				return print(cmd, o, output.Success(in.Name, in))
			}
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "Run "+product+" instance list --json to see configured names.", 404))
	}})
	add := &cobra.Command{Use: "add <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadCfg(o)
		baseURL := strings.TrimSpace(mustS(cmd, "base-url"))
		if baseURL == "" {
			return print(cmd, o, output.Failure("invalid_args", "--base-url is required", "Pass the Nexus Repository base URL, for example https://nexus.example.test.", 400))
		}
		if u, err := url.Parse(baseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return print(cmd, o, output.Failure("invalid_args", "--base-url must be an absolute http or https URL", "", 400))
		}
		for _, in := range cfg.Nexus.Instances {
			if in.Name == args[0] {
				return print(cmd, o, output.Failure("conflict", "instance already exists", "Use "+product+" instance update or auth login to change it, or pick another name.", 409))
			}
		}
		auth, provided, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", authHint+" Omit every auth flag to register an anonymous-read instance.", 400))
		}
		in := config.InstanceConfig{Name: args[0], BaseURL: baseURL, RESTPath: strings.TrimSpace(mustS(cmd, "rest-path")), Auth: auth}
		cfg.Nexus.Instances = append(cfg.Nexus.Instances, in)
		if mustB(cmd, "default") {
			cfg.Nexus.DefaultInstance = args[0]
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, saveFailure(err))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"added": true, "anonymous": !provided, "default_instance": cfg.Nexus.DefaultInstance}))
	}}
	add.Flags().String("base-url", "", "Nexus Repository base URL, for example https://nexus.example.test.")
	add.Flags().String("rest-path", "", "REST API prefix override; empty means "+nexus.DefaultRESTPath+".")
	addAuthFlags(add)
	add.Flags().Bool("default", false, "Make the added Nexus instance the default instance.")
	c.AddCommand(add)
	update := &cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, configMissing(err))
		}
		baseURL := strings.TrimSpace(mustS(cmd, "base-url"))
		restPath := strings.TrimSpace(mustS(cmd, "rest-path"))
		if baseURL == "" && restPath == "" {
			return print(cmd, o, output.Failure("invalid_args", "nothing to update", "Pass --base-url and/or --rest-path; use auth login to change credentials.", 400))
		}
		if baseURL != "" {
			if u, err := url.Parse(baseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return print(cmd, o, output.Failure("invalid_args", "--base-url must be an absolute http or https URL", "", 400))
			}
		}
		for i := range cfg.Nexus.Instances {
			if cfg.Nexus.Instances[i].Name != args[0] {
				continue
			}
			if baseURL != "" {
				cfg.Nexus.Instances[i].BaseURL = baseURL
			}
			if restPath != "" {
				cfg.Nexus.Instances[i].RESTPath = restPath
			}
			if err := saveCfg(o, cfg); err != nil {
				return print(cmd, o, saveFailure(err))
			}
			return print(cmd, o, output.Success(args[0], map[string]any{"updated": true}))
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "Run "+product+" instance list --json to see configured names.", 404))
	}}
	update.Flags().String("base-url", "", "New Nexus Repository base URL.")
	update.Flags().String("rest-path", "", "New REST API prefix; empty keeps the current value.")
	c.AddCommand(update)
	c.AddCommand(&cobra.Command{Use: "remove <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the instance removal.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, configMissing(err))
		}
		out := []config.InstanceConfig{}
		found := false
		for _, in := range cfg.Nexus.Instances {
			if in.Name == args[0] {
				found = true
				continue
			}
			out = append(out, in)
		}
		if !found {
			return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
		}
		cfg.Nexus.Instances = out
		if cfg.Nexus.DefaultInstance == args[0] {
			cfg.Nexus.DefaultInstance = ""
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, saveFailure(err))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"removed": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "default [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, configMissing(err))
		}
		if len(args) == 0 {
			return print(cmd, o, output.Success("", map[string]any{"default_instance": cfg.Nexus.DefaultInstance}))
		}
		found := false
		for _, in := range cfg.Nexus.Instances {
			if in.Name == args[0] {
				found = true
			}
		}
		if !found {
			return print(cmd, o, output.Failure("not_found", "instance not found", "Run "+product+" instance list --json to see configured names.", 404))
		}
		cfg.Nexus.DefaultInstance = args[0]
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, saveFailure(err))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"default_instance": args[0]}))
	}})
	return c
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	login := &cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, configMissing(err))
		}
		auth, provided, authErr := authFromFlags(cmd)
		if authErr != nil || !provided {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", authHint, 400))
		}
		idx, err := selectedInstanceIndex(cfg.Nexus, o.Instance)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		cfg.Nexus.Instances[idx].Auth = auth
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, saveFailure(err))
		}
		return print(cmd, o, output.Success(cfg.Nexus.Instances[idx].Name, map[string]any{"logged_in": true, "auth_type": auth.Type}))
	}}
	addAuthFlags(login)
	c.AddCommand(login)
	c.AddCommand(&cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming that the stored credentials should be cleared.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, configMissing(err))
		}
		idx, err := selectedInstanceIndex(cfg.Nexus, o.Instance)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		cfg.Nexus.Instances[idx].Auth = config.AuthConfig{}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, saveFailure(err))
		}
		return print(cmd, o, output.Success(cfg.Nexus.Instances[idx].Name, map[string]any{"logged_out": true, "anonymous": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(cmd, o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		resp, err := cx.Client.Do(httpclient.Request{Method: http.MethodGet, Path: "repositories"})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		v, err := nexus.JSONValue(resp.Body)
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "Check that base_url and rest_path point at the Nexus REST API v1.", 502))
		}
		repos, _ := v.([]any)
		return print(cmd, o, output.Success(cx.Inst.Name, map[string]any{
			"authenticated":    !cx.Anonymous,
			"anonymous":        cx.Anonymous,
			"auth_type":        nexus.AuthLabel(cx.Inst.Auth),
			"repository_count": len(repos),
			"base_url":         cx.Inst.BaseURL,
			"rest_path":        cx.Inst.RESTPath,
		}))
	}})
	return c
}

func selectedInstanceIndex(p config.ProductConfig, explicit string) (int, error) {
	if len(p.Instances) == 0 {
		return -1, fmt.Errorf("no_instance_configured")
	}
	target := explicit
	if target == "" {
		target = p.DefaultInstance
	}
	if target == "" && len(p.Instances) == 1 {
		return 0, nil
	}
	if target == "" {
		return -1, fmt.Errorf("instance_required")
	}
	for i := range p.Instances {
		if p.Instances[i].Name == target {
			return i, nil
		}
	}
	return -1, fmt.Errorf("instance_required")
}

func repoCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "repo"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return getJSONList(o, cmd, "repositories", nil, "repositories")
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "repositories/"+url.PathEscape(args[0]), nil)
	}})
	return c
}

func componentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "component"}
	search := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return runSearch(o, cmd, "search")
	}}
	addSearchFlags(search)
	c.AddCommand(search)
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return runRepositoryList(o, cmd, "components")
	}}
	addRepositoryListFlags(list)
	c.AddCommand(list)
	c.AddCommand(&cobra.Command{Use: "get <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "components/"+url.PathEscape(args[0]), nil)
	}})
	return c
}

func assetCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "asset"}
	search := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return runSearch(o, cmd, "search/assets")
	}}
	addSearchFlags(search)
	c.AddCommand(search)
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return runRepositoryList(o, cmd, "assets")
	}}
	addRepositoryListFlags(list)
	c.AddCommand(list)
	c.AddCommand(&cobra.Command{Use: "get <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "assets/"+url.PathEscape(args[0]), nil)
	}})
	download := &cobra.Command{Use: "download <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return runDownload(o, cmd, args[0])
	}}
	download.Flags().String("output", "", "Local output path; defaults to the asset file name in the current directory.")
	c.AddCommand(download)
	return c
}

func apiCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	get := &cobra.Command{Use: "get <path>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q, err := nexus.ParseKeyValue(mustSA(cmd, "query"))
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Pass --query key=value; repeat the flag for several parameters.", 400))
		}
		return getJSON(o, cmd, args[0], q)
	}}
	get.Flags().StringArray("query", nil, "Raw query parameter in key=value form; repeat for multiple parameters.")
	c.AddCommand(get)
	return c
}

func addSearchFlags(c *cobra.Command) {
	c.Flags().String("repository", "", "Restrict results to one repository name.")
	c.Flags().String("repo-format", "", "Repository format filter such as maven2, npm, docker, raw, pypi, nuget, or helm (the Nexus format query parameter; --format is the output format).")
	c.Flags().String("group", "", "Component group filter, for example a Maven groupId or an npm scope.")
	c.Flags().String("name", "", "Component name filter, for example a Maven artifactId or an npm package name.")
	c.Flags().String("version", "", "Component version filter.")
	c.Flags().String("keyword", "", "Free-text keyword search across component and asset fields (the Nexus q query parameter).")
	c.Flags().String("sort", "", "Sort field: group, name, version, or repository.")
	c.Flags().String("direction", "", "Sort direction: asc or desc.")
	c.Flags().String("maven-group-id", "", "Maven groupId filter (maven.groupId).")
	c.Flags().String("maven-artifact-id", "", "Maven artifactId filter (maven.artifactId).")
	c.Flags().String("maven-base-version", "", "Maven base version filter (maven.baseVersion), for example 1.0-SNAPSHOT.")
	c.Flags().String("maven-extension", "", "Maven extension filter (maven.extension), for example jar or pom.")
	c.Flags().String("maven-classifier", "", "Maven classifier filter (maven.classifier), for example sources.")
	c.Flags().String("docker-image-name", "", "Docker image name filter (docker.imageName).")
	c.Flags().String("docker-image-tag", "", "Docker image tag filter (docker.imageTag).")
	addPagingFlags(c)
}

func addRepositoryListFlags(c *cobra.Command) {
	c.Flags().String("repository", "", "Repository name to list; required.")
	addPagingFlags(c)
}

func addPagingFlags(c *cobra.Command) {
	c.Flags().String("continuation", "", "Continuation token from a previous page (continuation_token in the previous response).")
	c.Flags().Int("limit", nexus.DefaultLimit, fmt.Sprintf("Maximum number of items to return, between 1 and %d; items cut from the last fetched page are reported in dropped and are skipped by continuation_token.", nexus.MaxLimit))
	c.Flags().Bool("all", false, "Follow continuation tokens across pages until --limit or --max-pages is reached.")
	c.Flags().Int("max-pages", nexus.DefaultMaxPages, "Maximum number of pages to fetch when --all is set.")
}

func pagingFromFlags(cmd *cobra.Command) (nexus.PageOptions, error) {
	opts := nexus.PageOptions{Continuation: mustS(cmd, "continuation"), Limit: mustI(cmd, "limit"), All: mustB(cmd, "all"), MaxPages: mustI(cmd, "max-pages")}
	// The cobra defaults are positive, so a zero here is an explicit user value
	// and must not fall back to the package defaults inside Validate.
	if opts.Limit < 1 || opts.Limit > nexus.MaxLimit {
		return opts, fmt.Errorf("--limit must be between 1 and %d", nexus.MaxLimit)
	}
	if opts.MaxPages < 1 {
		return opts, errors.New("--max-pages must be at least 1")
	}
	if err := opts.Validate(); err != nil {
		return opts, err
	}
	return opts, nil
}

func runSearch(o *Opts, cmd *cobra.Command, path string) error {
	opts, err := pagingFromFlags(cmd)
	if err != nil {
		return print(cmd, o, output.Failure("invalid_args", err.Error(), "Use --continuation or --all to read more pages instead of a larger --limit.", 400))
	}
	filters := nexus.SearchFilters{
		Repository:       mustS(cmd, "repository"),
		Format:           mustS(cmd, "repo-format"),
		Group:            mustS(cmd, "group"),
		Name:             mustS(cmd, "name"),
		Version:          mustS(cmd, "version"),
		Keyword:          mustS(cmd, "keyword"),
		Sort:             mustS(cmd, "sort"),
		Direction:        mustS(cmd, "direction"),
		MavenGroupID:     mustS(cmd, "maven-group-id"),
		MavenArtifactID:  mustS(cmd, "maven-artifact-id"),
		MavenBaseVersion: mustS(cmd, "maven-base-version"),
		MavenExtension:   mustS(cmd, "maven-extension"),
		MavenClassifier:  mustS(cmd, "maven-classifier"),
		DockerImageName:  mustS(cmd, "docker-image-name"),
		DockerImageTag:   mustS(cmd, "docker-image-tag"),
	}
	return runPaged(o, cmd, path, filters.Query(), opts)
}

func runRepositoryList(o *Opts, cmd *cobra.Command, path string) error {
	repository := strings.TrimSpace(mustS(cmd, "repository"))
	if repository == "" {
		return print(cmd, o, output.Failure("invalid_args", "--repository is required", "Run "+product+" repo list --json to discover repository names.", 400))
	}
	opts, err := pagingFromFlags(cmd)
	if err != nil {
		return print(cmd, o, output.Failure("invalid_args", err.Error(), "Use --continuation or --all to read more pages instead of a larger --limit.", 400))
	}
	return runPaged(o, cmd, path, map[string]string{"repository": repository}, opts)
}

func runPaged(o *Opts, cmd *cobra.Command, path string, query map[string]string, opts nexus.PageOptions) error {
	cx, err := loadCtx(cmd, o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	result, err := nexus.FetchPages(cx.Client, path, query, opts)
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	return print(cmd, o, output.Success(cx.Inst.Name, result))
}

func runDownload(o *Opts, cmd *cobra.Command, id string) error {
	cx, err := loadCtx(cmd, o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	metaPath := "assets/" + url.PathEscape(id)
	target := mustS(cmd, "output")
	if o.DryRun {
		return print(cmd, o, output.Success(cx.Inst.Name, map[string]any{"dry_run": true, "method": http.MethodGet, "path": metaPath, "output": target, "note": "the asset downloadUrl is read from the asset metadata and must belong to the instance base_url"}))
	}
	resp, err := cx.Client.Do(httpclient.Request{Method: http.MethodGet, Path: metaPath})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	asset, err := nexus.JSONMap(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return print(cmd, o, output.Failure("server_error", err.Error(), "Check that base_url and rest_path point at the Nexus REST API v1.", 502))
	}
	downloadURL := strings.TrimSpace(nexus.StringField(asset, "downloadUrl"))
	if downloadURL == "" {
		return print(cmd, o, output.Failure("not_found", "asset metadata has no downloadUrl", "Run "+product+" asset get with the same id to inspect the asset.", 404))
	}
	if !nexus.URLBelongsToBase(downloadURL, cx.Inst.BaseURL) {
		return print(cmd, o, output.Failure("instance_url_mismatch", "asset downloadUrl is outside the selected instance base_url", "Credentials are only sent to the configured instance; check base_url or select the instance that owns this asset with --instance.", 400))
	}
	target = nexus.DownloadTarget(target, nexus.StringField(asset, "path"))
	body, err := cx.Client.WithTimeout(nexus.DownloadTimeout).Do(httpclient.Request{Method: http.MethodGet, Path: downloadURL})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer body.Body.Close()
	n, sha, err := nexus.SaveBody(body.Body, target)
	if err != nil {
		return print(cmd, o, output.Failure("server_error", httpclient.SanitizeErrorText(err.Error()), "Check that the --output directory is writable.", 500))
	}
	data := map[string]any{
		"asset_id":     id,
		"repository":   nexus.StringField(asset, "repository"),
		"path":         target,
		"bytes":        n,
		"sha1":         sha,
		"content_type": body.Header.Get("Content-Type"),
		"name":         nexus.ResponseName(body, nexus.DownloadTarget("", nexus.StringField(asset, "path"))),
		"download_url": downloadURL,
	}
	if expected := nexus.Checksum(asset, "sha1"); expected != "" {
		data["sha1_verified"] = strings.EqualFold(expected, sha)
	}
	return print(cmd, o, output.Success(cx.Inst.Name, data))
}

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra(product, args[0], cmd.Root())
		if !ok {
			return print(cmd, o, output.Failure("not_found", "command not found", "Run "+product+" commands --json to list command names.", 404))
		}
		return print(cmd, o, output.Success("", schema))
	}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"For agents, --json is the default way to use every " + product + " command and subcommand.",
			"Run " + product + " commands --json, then " + product + " schema <command> --json before constructing a complex call.",
			"Start with " + product + " repo list --json to discover repository names and formats, then search inside them.",
			"Find a component with " + product + " component search --repository <repo> --name <artifact> --version <ver> --json; use --maven-group-id/--maven-artifact-id for Maven coordinates, --group for npm scopes, and --keyword for free text.",
			"Docker images are searched with --docker-image-name and --docker-image-tag, for example " + product + " asset search --repository docker-hosted --docker-image-name payments/api --docker-image-tag 2.3.0 --json.",
			"--repo-format filters by repository format (maven2, npm, docker, raw, pypi, nuget, helm); --format selects the CLI output format.",
			"Results are paged: when truncated is true, pass --continuation <continuation_token> for the next page, or use --all --max-pages <n> to follow pages automatically; --limit caps the items returned and dropped reports items cut from the last fetched page (they are skipped by the continuation token, so keep the default --limit for gap-free walks).",
			"Use " + product + " component get <id> to list a component's assets, then " + product + " asset download <asset-id> --output <file> --json; the envelope carries metadata only (path, bytes, sha1, content_type), never file bytes.",
			"Use --instance when several instances are configured; without it the default instance is used. An instance without an auth block is queried anonymously.",
			product + " is read-only against the repository manager: it never uploads, deletes, or administers anything; only instance and auth commands write, and they change the local EFP config.",
			"Inspect error.code and error.hint before retrying.",
		}
		return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}

func getJSON(o *Opts, cmd *cobra.Command, path string, q map[string]string) error {
	cx, err := loadCtx(cmd, o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	resp, err := cx.Client.Do(httpclient.Request{Method: http.MethodGet, Path: path, Query: q})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	data, err := responseData(resp)
	if err != nil {
		return print(cmd, o, output.Failure("server_error", err.Error(), "", 502))
	}
	return print(cmd, o, output.Success(cx.Inst.Name, data))
}

// getJSONList wraps a JSON array response under listKey with its count so the
// envelope data stays an object.
func getJSONList(o *Opts, cmd *cobra.Command, path string, q map[string]string, listKey string) error {
	cx, err := loadCtx(cmd, o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	resp, err := cx.Client.Do(httpclient.Request{Method: http.MethodGet, Path: path, Query: q})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	v, err := nexus.JSONValue(resp.Body)
	if err != nil {
		return print(cmd, o, output.Failure("server_error", err.Error(), "Check that base_url and rest_path point at the Nexus REST API v1.", 502))
	}
	items, ok := v.([]any)
	if !ok {
		items = []any{}
		if v != nil {
			items = append(items, v)
		}
	}
	return print(cmd, o, output.Success(cx.Inst.Name, map[string]any{listKey: items, "count": len(items)}))
}

// responseData turns a raw response into envelope data: decoded JSON when the
// body is JSON, an empty-body status marker, or the text with its content type.
func responseData(resp *http.Response) (any, error) {
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return map[string]any{"status": resp.StatusCode, "content_type": contentType}, nil
	}
	if strings.Contains(contentType, "json") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		v, err := nexus.JSONValue(strings.NewReader(trimmed))
		if err == nil {
			return v, nil
		}
		if strings.Contains(contentType, "json") {
			return nil, err
		}
	}
	return map[string]any{"text": string(b), "content_type": contentType, "status": resp.StatusCode}, nil
}

func mustS(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func mustB(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func mustI(cmd *cobra.Command, name string) int {
	v, _ := cmd.Flags().GetInt(name)
	return v
}

func mustSA(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringArray(name)
	return v
}
