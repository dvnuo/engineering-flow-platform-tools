package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/appium"
	"engineering-flow-platform-tools/internal/browserstack"
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

type Opts struct {
	ConfigPath string
	Format     string
	JSON       bool
	Verbose    bool
}

type services struct {
	Runtime mobile.RuntimeConfig
	Store   *mobile.StateStore
	Control *browserstack.Client
	Appium  *appium.Client
	Tunnel  *mobile.TunnelManager
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{Use: "mobile", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "")
	c.PersistentFlags().StringVar(&o.ConfigPath, "config", "", "")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "")
	c.AddCommand(
		doctorCmd(o), authCmd(o), appCmd(o), deviceCmd(o), capacityCmd(o), tunnelCmd(o),
		projectCmd(o), buildCmd(o), sessionCmd(o), runCmd(o),
		observeCmd(o), locateCmd(o), tapCmd(o), tapPointCmd(o), typeCmd(o), clearCmd(o),
		scrollCmd(o), scrollToCmd(o), swipeCmd(o), longPressCmd(o), doubleTapCmd(o), dragCmd(o), backCmd(o), keyboardCmd(o), contextCmd(o),
		permissionsCmd(o), assertCmd(o), waitCmd(o), inspectorCmd(o), workflowCmd(o), testCmd(o), artifactCmd(o), keepaliveCmd(o),
		commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o),
	)
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "mobile",
		Binary:  "mobile",
		Short:   "Operate BrowserStack App Automate real devices through a governed Appium CLI",
		Long: strings.TrimSpace(`mobile is a terminal-invoked CLI for Engineering Flow Platform agents that need to upload apps, resolve BrowserStack real devices, manage capacity and Local tunnels, run Appium sessions, observe mobile UI state, and perform bounded ref-based actions.

It is not an MCP server, not BrowserStack AI, and not an arbitrary Appium pass-through. For agents, use --json on every command and inspect command schemas before complex calls.`),
		Examples: []string{
			`mobile run start --file ./app.apk --platform android --network public --json`,
			`mobile observe --run-id run-... --json`,
			`mobile locate --run-id run-... --role button --name Login --json`,
			`mobile tap --run-id run-... --ref obs-...:e1 --json`,
			`mobile run handoff --run-id run-... --hold-for 10m --json`,
			`mobile run finish --run-id run-... --status passed --collect-artifacts --json`,
			`mobile commands --json`,
			`mobile schema run.start --json`,
		},
		Instructions: "copy cmd/mobile/mobile-cli.instructions.md to ~/.copilot/instructions/mobile-cli.instructions.md.",
		Groups: map[string]string{
			"app":       "Manage BrowserStack App Automate app uploads and app references.",
			"device":    "List and deterministically resolve BrowserStack real devices.",
			"capacity":  "Inspect and wait for BrowserStack parallel and queue capacity.",
			"tunnel":    "Manage BrowserStack Local tunnels for private network sessions.",
			"run":       "Orchestrate full mobile runs, human handoff, resume, and cleanup.",
			"session":   "Start, inspect, mark, and stop remote Appium/BrowserStack sessions.",
			"inspector": "Generate Appium Inspector config, attach hints, and observation exports.",
			"test":      "Run structured mobile automation suites with reports and evidence.",
			"artifact":  "Collect local and BrowserStack artifacts without printing large content.",
		},
	})
	return c
}

func newServices(o *Opts, requireAuth bool) (*services, error) {
	rt, err := mobile.LoadRuntimeConfig(o.ConfigPath)
	if err != nil {
		return nil, err
	}
	if requireAuth {
		if err := mobile.RequireCredentials(rt); err != nil {
			return nil, err
		}
	}
	store := mobile.NewStateStore(rt.Mobile.StateDir, rt.Mobile.ArtifactsDir)
	if err := store.Ensure(); err != nil {
		return nil, mobile.NewError("config_error", "state/artifact directories are not writable", "Check mobile.state_dir and mobile.artifacts_dir permissions.", 400)
	}
	verify := true
	if rt.Mobile.BrowserStack.VerifySSL != nil {
		verify = *rt.Mobile.BrowserStack.VerifySSL
	}
	bsCreds := browserstack.Credentials{Username: rt.Credentials.Username, AccessKey: rt.Credentials.AccessKey}
	control, err := browserstack.New(rt.Mobile.BrowserStack.APIBaseURL, bsCreds, verify, rt.Mobile.BrowserStack.CACert, rt.HTTPProxy)
	if err != nil {
		return nil, err
	}
	appiumClient, err := appium.New(rt.Mobile.BrowserStack.AppiumBaseURL, bsCreds, verify, rt.Mobile.BrowserStack.CACert, rt.HTTPProxy)
	if err != nil {
		return nil, err
	}
	tunnel := &mobile.TunnelManager{Store: store, Config: rt.Mobile.BrowserStack.Local, Credentials: rt.Credentials}
	return &services{Runtime: rt, Store: store, Control: control, Appium: appiumClient, Tunnel: tunnel}, nil
}

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.CommandsFromCobra("mobile", cmd.Root())}))
	}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra("mobile", args[0], cmd.Root())
		if !ok {
			return print(cmd, o, output.Failure("not_found", "command not found", "Run mobile commands --json to list command names.", 404))
		}
		return print(cmd, o, output.Success("", schema))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := mobileLLMTips()
		if fmtOut(o) == "json" {
			return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.CommandsFromCobra("mobile", cmd.Root())}))
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "# mobile CLI usage for agents\n\n- "+strings.Join(tips, "\n- "))
		return err
	}}
}

func mobileLLMTips() []string {
	return []string{
		"mobile is a terminal CLI, not MCP and not a BrowserStack AI integration.",
		"Always use --json for agent workflows.",
		"Run mobile commands --json and mobile schema <command> --json before complex calls.",
		"Recommended flow: run start, observe, locate, action, observe, assert, run finish.",
		"Never invent refs, selectors, XPath, resource IDs, or coordinates.",
		"Use only refs from the latest observation; re-observe after every mutating action.",
		"Never act on ambiguous locate results.",
		"Use --text-env or --text-stdin for secrets; typed secret values are not returned.",
		"Public sessions do not start BrowserStack Local; private sessions require managed or external tunnel identifiers.",
		"handoff transfers control to the human and mutating actions stay locked until resume.",
		"Always finish runs and collect artifacts on failure when useful.",
	}
}

func doctorCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "doctor", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		local := svc.Runtime.Mobile.BrowserStack.Local
		bin := firstNonEmpty(os.Getenv(local.BinaryEnv), local.Binary)
		_, binErr := exec.LookPath(bin)
		apiProxy := svc.Control.ProxyDiagnostic()
		appiumProxy := svc.Appium.ProxyDiagnostic()
		data := map[string]any{
			"config_path":                   svc.Runtime.Path,
			"warnings":                      svc.Runtime.Warnings,
			"credentials_present":           map[string]bool{"username": svc.Runtime.Username, "access_key": svc.Runtime.AccessKey},
			"api_base_url":                  svc.Runtime.Mobile.BrowserStack.APIBaseURL,
			"appium_base_url":               svc.Runtime.Mobile.BrowserStack.AppiumBaseURL,
			"state_dir":                     svc.Runtime.Mobile.StateDir,
			"artifacts_dir":                 svc.Runtime.Mobile.ArtifactsDir,
			"state_dir_writable":            true,
			"artifacts_dir_writable":        true,
			"local_binary":                  bin,
			"local_binary_found":            binErr == nil,
			"verify_ssl":                    svc.Runtime.Mobile.BrowserStack.VerifySSL == nil || *svc.Runtime.Mobile.BrowserStack.VerifySSL,
			"browserstack_api_proxy":        apiProxy,
			"appium_proxy":                  appiumProxy,
			"browserstack_api_proxy_source": apiProxy.Source,
			"browserstack_api_proxy_host":   apiProxy.Host,
			"browserstack_api_proxy_port":   apiProxy.Port,
			"appium_proxy_source":           appiumProxy.Source,
			"appium_proxy_host":             appiumProxy.Host,
			"appium_proxy_port":             appiumProxy.Port,
			"http2_enabled":                 false,
			"transport_mode":                apiProxy.TransportMode,
		}
		return print(cmd, o, output.Success("", data))
	}}
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	var username, accessKey string
	var accessKeyStdin bool
	login := &cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		username = strings.TrimSpace(username)
		accessKey = strings.TrimSpace(accessKey)
		if accessKeyStdin {
			b, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return print(cmd, o, output.Failure("invalid_args", "could not read access key from stdin", "Pipe the BrowserStack access key to --access-key-stdin.", 400))
			}
			accessKey = strings.TrimRight(string(b), "\r\n")
		}
		if username == "" || accessKey == "" {
			return print(cmd, o, output.Failure("invalid_args", "--username and an access key are required", "Use --access-key-stdin to avoid shell history, or set BROWSERSTACK_USERNAME/BROWSERSTACK_ACCESS_KEY.", 400))
		}
		if accessKeyStdin && cmd.Flags().Changed("access-key") {
			return print(cmd, o, output.Failure("invalid_args", "use exactly one access key source", "Choose --access-key or --access-key-stdin.", 400))
		}
		path, cfg, err := loadMobileRootConfig(o.ConfigPath)
		if err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "Check --config or EFP_CONFIG.", 400))
		}
		cfg.Mobile.Normalize()
		cfg.Mobile.BrowserStack.Username = username
		cfg.Mobile.BrowserStack.AccessKey = accessKey
		if err := config.Save(path, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "Check config file permissions.", 500))
		}
		return print(cmd, o, output.Success("", map[string]any{
			"logged_in":   true,
			"config_path": path,
			"username":    username,
			"hint":        "Environment variables BROWSERSTACK_USERNAME/BROWSERSTACK_ACCESS_KEY still take precedence when set.",
		}))
	}}
	login.Flags().StringVar(&username, "username", "", "")
	login.Flags().StringVar(&accessKey, "access-key", "", "")
	login.Flags().BoolVar(&accessKeyStdin, "access-key-stdin", false, "")
	c.AddCommand(login)
	var yes bool
	logout := &cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes is required for auth logout", "Re-run with --yes after confirming removal from config.", 400))
		}
		path, cfg, err := loadMobileRootConfig(o.ConfigPath)
		if err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "Check --config or EFP_CONFIG.", 400))
		}
		cfg.Mobile.BrowserStack.Username = ""
		cfg.Mobile.BrowserStack.AccessKey = ""
		if err := config.Save(path, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "Check config file permissions.", 500))
		}
		return print(cmd, o, output.Success("", map[string]any{"logged_out": true, "config_path": path}))
	}}
	logout.Flags().BoolVar(&yes, "yes", false, "")
	c.AddCommand(logout)
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		if err := svc.Control.AuthTest(ctx); err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"authenticated": true, "provider": "browserstack"}))
	}})
	return c
}

func loadMobileRootConfig(flagPath string) (string, config.RootConfig, error) {
	path, err := config.ResolvePath(flagPath)
	if err != nil {
		return "", config.RootConfig{}, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", config.RootConfig{}, err
		}
		cfg = config.RootConfig{}
		cfg.Normalize()
	}
	return path, cfg, nil
}

func print(cmd *cobra.Command, o *Opts, env output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), env)
}

func renderErr(cmd *cobra.Command, o *Opts, err error) error {
	var me *mobile.Error
	if errors.As(err, &me) {
		env := output.Failure(me.Code, me.Message, me.Hint, me.Status)
		env.Error.Retryable = me.Retryable
		env.Error.RecommendedAction = me.RecommendedAction
		return print(cmd, o, env)
	}
	return print(cmd, o, output.Failure("server_error", output.RedactString(err.Error()), "", 500))
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
