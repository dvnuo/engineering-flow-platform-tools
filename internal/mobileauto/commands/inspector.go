package commands

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/mobileauto"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type inspectorConfigOptions struct {
	RunID           string
	App             string
	Platform        string
	Device          string
	OSVersion       string
	Network         string
	LocalIdentifier string
	Project         string
	Build           string
	Name            string
	ShowSecret      bool
	SecretMode      string
	Out             string
}

func inspectorCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "inspector"}
	c.AddCommand(inspectorConfigCmd(o), inspectorAttachCmd(o), inspectorExportCmd(o), inspectorLocatorCmd(o))
	return c
}

func inspectorConfigCmd(o *Opts) *cobra.Command {
	opts := inspectorConfigOptions{}
	c := &cobra.Command{Use: "config", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var st *mobileauto.RunState
		if opts.RunID != "" {
			loaded, err := svc.Store.LoadRun(opts.RunID)
			if err != nil {
				return renderErr(cmd, o, err)
			}
			st = &loaded
		}
		mode, err := normalizeInspectorSecretMode(opts.SecretMode)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Use --secret-mode redacted or --secret-mode env.", 400))
		}
		opts.SecretMode = mode
		data := inspectorConfigData(svc, opts, st)
		if opts.Out != "" {
			if err := writeJSONFile(opts.Out, data); err != nil {
				return renderErr(cmd, o, err)
			}
			data["out"] = opts.Out
		}
		return print(cmd, o, output.Success("", data))
	}}
	bindInspectorConfigFlags(c, &opts)
	return c
}

func inspectorAttachCmd(o *Opts) *cobra.Command {
	var runID string
	var showSecret bool
	var secretMode string
	c := &cobra.Command{Use: "attach", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the run id to inspect.", 400))
		}
		mode, err := normalizeInspectorSecretMode(secretMode)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Use --secret-mode redacted or --secret-mode env.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err := svc.Store.LoadRun(runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		opts := inspectorConfigOptions{RunID: runID, ShowSecret: showSecret, SecretMode: mode}
		data := inspectorConfigData(svc, opts, &st)
		contexts := []string{}
		currentContext := ""
		warnings := []string{}
		if st.SessionID != "" {
			if got, err := svc.Appium.Contexts(cmd.Context(), st.SessionID); err == nil {
				contexts = got
			} else {
				warnings = append(warnings, "contexts unavailable: "+output.RedactString(err.Error()))
				markRunLostIfSessionGone(svc, &st, err)
			}
			if got, err := svc.Appium.CurrentContext(cmd.Context(), st.SessionID); err == nil {
				currentContext = got
			} else {
				warnings = append(warnings, "current context unavailable: "+output.RedactString(err.Error()))
				markRunLostIfSessionGone(svc, &st, err)
			}
		}
		if len(warnings) > 0 {
			data["warnings"] = appendStringValues(data["warnings"], warnings...)
		}
		data["attach"] = map[string]any{
			"run_id":          runID,
			"run_status":      st.Status,
			"session_id":      st.SessionID,
			"dashboard_url":   st.DashboardURL,
			"browser_url":     st.BrowserURL,
			"public_url":      st.PublicURL,
			"current_context": currentContext,
			"contexts":        classifyContexts(contexts),
			"warnings":        warnings,
			"handoff_hint":    "Use mobile-auto run handoff --run-id " + runID + " --mode inspector --json before manual Inspector work, then mobile-auto run resume.",
		}
		return print(cmd, o, output.Success("", data))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().BoolVar(&showSecret, "show-secret", false, "")
	c.Flags().StringVar(&secretMode, "secret-mode", "redacted", "")
	return c
}

func inspectorExportCmd(o *Opts) *cobra.Command {
	var runID, outDir string
	c := &cobra.Command{Use: "export", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" || outDir == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id and --out are required", "Pass a run id and output directory.", 400))
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
			return print(cmd, o, output.Failure("stale_observation", "no current observation is available", "Run mobile-auto observe first, then export.", 409))
		}
		obs, err := svc.Store.LoadObservation(runID, st.LatestObservationID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			return renderErr(cmd, o, err)
		}
		copied := map[string]string{}
		for name, path := range map[string]string{
			"source_xml":     obs.SourcePath,
			"screenshot_png": obs.ScreenshotPath,
			"candidates":     obs.CandidatesPath,
		} {
			if path == "" {
				continue
			}
			dst := filepath.Join(outDir, filepath.Base(path))
			if err := copyFile(path, dst); err == nil {
				copied[name] = dst
			}
		}
		manifest := map[string]any{"run": st, "observation": obs, "files": copied}
		manifestPath := filepath.Join(outDir, "inspector-manifest.json")
		if err := writeJSONFile(manifestPath, manifest); err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"run_id": runID, "out": outDir, "manifest": manifestPath, "files": copied}))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&outDir, "out", "", "")
	return c
}

func inspectorLocatorCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "locator"}
	var file string
	imp := &cobra.Command{Use: "import", RunE: func(cmd *cobra.Command, args []string) error {
		if file == "" {
			return print(cmd, o, output.Failure("invalid_args", "--file is required", "Pass an Appium Inspector locator JSON/YAML file.", 400))
		}
		locator, err := readInspectorLocator(file)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		data := inspectorLocatorData(locator)
		return print(cmd, o, output.Success("", data))
	}}
	imp.Flags().StringVar(&file, "file", "", "")
	c.AddCommand(imp)
	return c
}

func bindInspectorConfigFlags(c *cobra.Command, opts *inspectorConfigOptions) {
	c.Flags().StringVar(&opts.RunID, "run-id", "", "")
	c.Flags().StringVar(&opts.App, "app", "", "")
	c.Flags().StringVar(&opts.Platform, "platform", "", "")
	c.Flags().StringVar(&opts.Device, "device", "", "")
	c.Flags().StringVar(&opts.OSVersion, "os-version", "", "")
	c.Flags().StringVar(&opts.Network, "network", "", "")
	c.Flags().StringVar(&opts.LocalIdentifier, "local-identifier", "", "")
	c.Flags().StringVar(&opts.Project, "project", "", "")
	c.Flags().StringVar(&opts.Build, "build", "", "")
	c.Flags().StringVar(&opts.Name, "name", "", "")
	c.Flags().BoolVar(&opts.ShowSecret, "show-secret", false, "")
	c.Flags().StringVar(&opts.SecretMode, "secret-mode", "redacted", "")
	c.Flags().StringVar(&opts.Out, "out", "", "")
}

func inspectorConfigData(svc *services, opts inspectorConfigOptions, st *mobileauto.RunState) map[string]any {
	appiumURL := svc.Runtime.Mobile.BrowserStack.AppiumBaseURL
	parts := appiumURLParts(appiumURL)
	platform := firstNonEmpty(opts.Platform, valueFromRun(st, "platform"), svc.Runtime.Mobile.Defaults.Platform)
	device := firstNonEmpty(opts.Device, runDeviceName(st))
	app := firstNonEmpty(opts.App, runAppURL(st))
	build := firstNonEmpty(opts.Build, runBuildName(st))
	name := firstNonEmpty(opts.Name, runSessionName(st))
	network := firstNonEmpty(opts.Network, runNetworkMode(st), svc.Runtime.Mobile.Defaults.NetworkMode)
	localID := firstNonEmpty(opts.LocalIdentifier, runLocalIdentifier(st))
	caps := map[string]any{}
	if platform != "" {
		caps["platformName"] = inspectorCanonicalPlatform(platform)
		if strings.EqualFold(platform, "ios") {
			caps["appium:automationName"] = "XCUITest"
		} else {
			caps["appium:automationName"] = "UiAutomator2"
		}
	}
	if app != "" {
		caps["appium:app"] = app
	}
	if device != "" {
		caps["appium:deviceName"] = device
	}
	if osVersion := firstNonEmpty(opts.OSVersion, runOSVersion(st)); osVersion != "" {
		caps["appium:platformVersion"] = osVersion
	}
	bstack := map[string]any{
		"userName":  inspectorUsername(svc, opts.SecretMode),
		"accessKey": inspectorAccessKey(svc, opts.SecretMode),
	}
	if opts.Project != "" || runProjectName(st) != "" {
		bstack["projectName"] = firstNonEmpty(opts.Project, runProjectName(st))
	}
	if build != "" {
		bstack["buildName"] = build
	}
	if name != "" {
		bstack["sessionName"] = name
	}
	if network != "public" && localID != "" {
		bstack["local"] = true
		bstack["localIdentifier"] = localID
	}
	caps["bstack:options"] = bstack
	data := map[string]any{
		"remote_url":        appiumURL,
		"server":            parts,
		"username":          inspectorUsername(svc, opts.SecretMode),
		"access_key":        inspectorAccessKey(svc, opts.SecretMode),
		"auth":              inspectorAuthSummary(opts),
		"capabilities":      caps,
		"network_mode":      network,
		"local_identifier":  localID,
		"run_id":            opts.RunID,
		"session_id":        runSessionID(st),
		"dashboard_url":     runDashboardURL(st),
		"copy_to_inspector": map[string]any{"remote_server_url": appiumURL, "capabilities": caps},
	}
	if warnings := inspectorConfigWarnings(opts); len(warnings) > 0 {
		data["warnings"] = warnings
	}
	return data
}

func inspectorCanonicalPlatform(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ios":
		return "iOS"
	case "android":
		return "Android"
	default:
		return v
	}
}

func appiumURLParts(raw string) map[string]string {
	u, err := url.Parse(raw)
	if err != nil {
		return map[string]string{"url": raw}
	}
	return map[string]string{"url": raw, "scheme": u.Scheme, "host": u.Host, "path": u.Path}
}

func normalizeInspectorSecretMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "redacted", nil
	}
	switch mode {
	case "redacted", "env":
		return mode, nil
	default:
		return "", mobileError("invalid_args", "unsupported inspector secret mode: "+mode, "Use redacted or env.", 400)
	}
}

func inspectorUsername(svc *services, mode string) string {
	if mode == "env" {
		return "${BROWSERSTACK_USERNAME}"
	}
	return svc.Runtime.Credentials.Username
}

func inspectorAccessKey(svc *services, mode string) string {
	if svc.Runtime.Credentials.AccessKey == "" {
		return ""
	}
	if mode == "env" {
		return "${BROWSERSTACK_ACCESS_KEY}"
	}
	return output.Redacted
}

func inspectorAuthSummary(opts inspectorConfigOptions) map[string]any {
	mode := firstNonEmpty(opts.SecretMode, "redacted")
	out := map[string]any{"mode": mode}
	if mode == "env" {
		out["env_vars"] = map[string]string{"username": "BROWSERSTACK_USERNAME", "key": "BROWSERSTACK_ACCESS_KEY"}
		out["note"] = "capabilities keep sensitive accessKey fields redacted in CLI JSON; use these environment variables when pasting into Inspector."
	}
	return out
}

func inspectorConfigWarnings(opts inspectorConfigOptions) []string {
	if !opts.ShowSecret {
		return nil
	}
	return []string{"--show-secret is ignored for JSON safety; use --secret-mode env to emit environment variable placeholders instead."}
}

func appendStringValues(existing any, values ...string) []string {
	out := []string{}
	if list, ok := existing.([]string); ok {
		out = append(out, list...)
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func valueFromRun(st *mobileauto.RunState, key string) string {
	if st == nil {
		return ""
	}
	switch key {
	case "platform":
		return st.Platform
	default:
		return ""
	}
}

func runDeviceName(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.Device.Name
}

func runOSVersion(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.Device.OSVersion
}

func runAppURL(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.App.AppURL
}

func runBuildName(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.BuildName
}

func runSessionName(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.SessionName
}

func runProjectName(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.ProjectName
}

func runNetworkMode(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.Network.Mode
}

func runLocalIdentifier(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.Network.LocalIdentifier
}

func runSessionID(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return st.SessionID
}

func runDashboardURL(st *mobileauto.RunState) string {
	if st == nil {
		return ""
	}
	return firstNonEmpty(st.DashboardURL, st.BrowserURL, st.PublicURL)
}

func readInspectorLocator(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		if yamlErr := yaml.Unmarshal(b, &raw); yamlErr != nil {
			return nil, err
		}
	}
	return raw, nil
}

func inspectorLocatorData(raw map[string]any) map[string]any {
	using := stringMapAny(raw, "using", "strategy", "locatorStrategy")
	value := stringMapAny(raw, "value", "selector", "locator", "target")
	q := map[string]string{}
	switch strings.ToLower(strings.TrimSpace(using)) {
	case "accessibility id", "accessibilityid", "accessibility_id":
		q["accessibility_id"] = value
	case "id", "resource id", "resource-id", "resource_id":
		q["resource_id"] = value
	case "name":
		q["name"] = value
	case "text":
		q["text"] = value
	default:
		if v := stringMapAny(raw, "accessibility_id", "accessibilityId", "content-desc"); v != "" {
			q["accessibility_id"] = v
		}
		if v := stringMapAny(raw, "resource_id", "resourceId", "id"); v != "" {
			q["resource_id"] = v
		}
		if v := stringMapAny(raw, "name", "label"); v != "" {
			q["name"] = v
		}
		if v := stringMapAny(raw, "text"); v != "" {
			q["text"] = v
		}
	}
	args := []string{"locate"}
	for key, value := range q {
		args = append(args, "--"+strings.ReplaceAll(key, "_", "-"), value)
	}
	return map[string]any{
		"input":          raw,
		"using":          using,
		"value":          value,
		"locate_query":   q,
		"cli_args":       args,
		"workflow_step":  map[string]any{"action": "locate", "name": q["name"], "text": q["text"], "resource_id": q["resource_id"]},
		"fallback_note":  "XPath/class chain selectors are returned as raw locator hints; prefer accessibility id or resource id for durable CLI workflows.",
		"raw_locator":    map[string]string{"using": using, "value": value},
		"requires_refit": len(q) == 0,
	}
}

func stringMapAny(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}

func writeJSONFile(path string, data any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	b, err := json.MarshalIndent(output.RedactValue(data), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
