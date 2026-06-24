package commands

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browserstack"
	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func appCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "app"}
	c.AddCommand(
		appUploadCmd(o), appListCmd(o), appGetCmd(o), appResolveCmd(o), appDeleteCmd(o),
		appLaunchCmd(o), appCloseCmd(o), appResetCmd(o), appActivateCmd(o), appTerminateCmd(o), appDeepLinkCmd(o),
	)
	return c
}

func appUploadCmd(o *Opts) *cobra.Command {
	var file, remoteURL, customID string
	var iosKeychain, dryRun bool
	c := &cobra.Command{Use: "upload", RunE: func(cmd *cobra.Command, args []string) error {
		if (strings.TrimSpace(file) == "") == (strings.TrimSpace(remoteURL) == "") {
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --file or --url is required", "Pass --file app.apk or --url https://...", 400))
		}
		var sha string
		if file != "" {
			if !browserstack.ValidAppExtension(file) {
				return print(cmd, o, output.Failure("invalid_args", "unsupported app extension", "BrowserStack App Automate supports .apk, .aab, .xapk, and .ipa app uploads.", 400))
			}
			var err error
			sha, err = browserstack.SHA256File(file)
			if err != nil {
				return renderErr(cmd, o, err)
			}
		}
		if dryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "file": file, "url": remoteURL, "custom_id": customID, "sha256": sha}))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		app, err := svc.Control.UploadApp(ctx, browserstack.UploadAppRequest{FilePath: file, URL: remoteURL, CustomID: customID, IOSKeychainSupport: iosKeychain, SHA256: sha})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		_ = svc.Store.SaveAppCache(appRefFromUploaded(app, customID, sha, filepath.Base(file)))
		return print(cmd, o, output.Success("", app))
	}}
	c.Flags().StringVar(&file, "file", "", "")
	c.Flags().StringVar(&remoteURL, "url", "", "")
	c.Flags().StringVar(&customID, "custom-id", "", "")
	c.Flags().BoolVar(&iosKeychain, "ios-keychain-support", false, "")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "")
	return c
}

func appListCmd(o *Opts) *cobra.Command {
	var limit, offset int
	var customID string
	var group bool
	c := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		apps, err := svc.Control.ListApps(cmd.Context(), browserstack.ListAppsRequest{Limit: limit, Offset: offset, CustomID: customID, Group: group})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"apps": apps, "count": len(apps)}))
	}}
	c.Flags().IntVar(&limit, "limit", 20, "")
	c.Flags().IntVar(&offset, "offset", 0, "")
	c.Flags().StringVar(&customID, "custom-id", "", "")
	c.Flags().BoolVar(&group, "group", false, "")
	return c
}

func appGetCmd(o *Opts) *cobra.Command {
	var appURL, appID, customID string
	c := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		if appURL == "" && appID == "" && customID == "" {
			return print(cmd, o, output.Failure("invalid_args", "one of --app-url, --app-id, or --custom-id is required", "Run mobile app list --json to discover app references.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		apps, err := svc.Control.ListApps(cmd.Context(), browserstack.ListAppsRequest{Limit: 100, CustomID: customID})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		for _, app := range apps {
			if (appURL != "" && app.AppURL == appURL) || (appID != "" && (app.AppID == appID || browserstack.AppIDFromURL(app.AppURL) == appID)) || (customID != "" && app.CustomID == customID) {
				return print(cmd, o, output.Success("", app))
			}
		}
		return print(cmd, o, output.Failure("app_not_found", "matching app was not found in recent BrowserStack apps", "Upload or resolve the app again.", 404))
	}}
	c.Flags().StringVar(&appURL, "app-url", "", "")
	c.Flags().StringVar(&appID, "app-id", "", "")
	c.Flags().StringVar(&customID, "custom-id", "", "")
	return c
}

func appResolveCmd(o *Opts) *cobra.Command {
	var appURL, file, remoteURL, customID string
	var dryRun bool
	c := &cobra.Command{Use: "resolve", RunE: func(cmd *cobra.Command, args []string) error {
		appURL = strings.TrimSpace(appURL)
		if appURL != "" && !strings.HasPrefix(appURL, "bs://") {
			return print(cmd, o, output.Failure("invalid_args", "--app-url must start with bs://", "Pass a BrowserStack app URL such as bs://<app-id>.", 400))
		}
		if appURL != "" {
			return print(cmd, o, output.Success("", mobile.AppRef{AppURL: appURL, CustomID: customID}))
		}
		if file == "" && remoteURL == "" && customID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--app-url, --file, --url, or --custom-id is required", "Pass an existing bs:// app or an app source to upload.", 400))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		var sha string
		if file != "" {
			if !browserstack.ValidAppExtension(file) {
				return print(cmd, o, output.Failure("invalid_args", "unsupported app extension", "BrowserStack App Automate supports .apk, .aab, .xapk, and .ipa.", 400))
			}
			sha, err = browserstack.SHA256File(file)
			if err != nil {
				return renderErr(cmd, o, err)
			}
			var cachedURL string
			if cached, err := svc.Store.LoadAppCache(sha); err == nil && cached.AppURL != "" {
				cachedURL = cached.AppURL
				if mobile.AppCacheReusable(cached, time.Now().UTC()) {
					return print(cmd, o, output.Success("", map[string]any{"app": cached, "reused": true, "source": "local_cache"}))
				}
			}
			if ref, ok := findRecentAppRef(cmd.Context(), svc, customID, sha, cachedURL); ok {
				_ = svc.Store.SaveAppCache(ref)
				return print(cmd, o, output.Success("", map[string]any{"app": ref, "reused": true, "source": "browserstack_recent_apps"}))
			}
		}
		if customID != "" {
			if ref, ok := findRecentAppRef(cmd.Context(), svc, customID, sha, ""); ok {
				_ = svc.Store.SaveAppCache(ref)
				return print(cmd, o, output.Success("", map[string]any{"app": ref, "reused": true, "source": "browserstack_recent_apps"}))
			}
		}
		if dryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "would_upload": true, "file": file, "url": remoteURL, "custom_id": customID, "sha256": sha}))
		}
		app, err := svc.Control.UploadApp(cmd.Context(), browserstack.UploadAppRequest{FilePath: file, URL: remoteURL, CustomID: customID, SHA256: sha})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		ref := appRefFromUploaded(app, customID, sha, app.AppName)
		_ = svc.Store.SaveAppCache(ref)
		return print(cmd, o, output.Success("", map[string]any{"app": ref, "reused": false}))
	}}
	c.Flags().StringVar(&appURL, "app-url", "", "")
	c.Flags().StringVar(&file, "file", "", "")
	c.Flags().StringVar(&remoteURL, "url", "", "")
	c.Flags().StringVar(&customID, "custom-id", "", "")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "")
	return c
}

func appRefFromUploaded(app browserstack.UploadedApp, customID, sha, fallbackName string) mobile.AppRef {
	uploadedAt := parseBrowserStackTime(app.UploadedAt)
	name := firstNonEmpty(app.AppName, fallbackName)
	ref := mobile.AppRef{
		AppURL:     app.AppURL,
		CustomID:   firstNonEmpty(customID, app.CustomID),
		SHA256:     firstNonEmpty(sha, app.SHA256),
		Name:       name,
		UploadedAt: uploadedAt,
	}
	return mobile.NormalizeAppCacheRef(ref, time.Now().UTC())
}

func findRecentAppRef(ctx context.Context, svc *services, customID, sha, appURL string) (mobile.AppRef, bool) {
	apps, err := svc.Control.ListApps(ctx, browserstack.ListAppsRequest{Limit: 100, CustomID: customID})
	if err != nil {
		return mobile.AppRef{}, false
	}
	for _, app := range apps {
		if strings.TrimSpace(app.AppURL) == "" {
			continue
		}
		if sha != "" && app.SHA256 != "" && !strings.EqualFold(app.SHA256, sha) {
			continue
		}
		matches := false
		switch {
		case sha != "" && app.SHA256 != "":
			matches = strings.EqualFold(app.SHA256, sha)
		case appURL != "":
			matches = app.AppURL == appURL
		case customID != "":
			matches = app.CustomID == "" || app.CustomID == customID
		}
		if matches {
			return appRefFromUploaded(app, customID, firstNonEmpty(sha, app.SHA256), app.AppName), true
		}
	}
	return mobile.AppRef{}, false
}

func parseBrowserStackTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func appDeleteCmd(o *Opts) *cobra.Command {
	var appID, appURL string
	var yes, dryRun bool
	c := &cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if appID == "" && appURL == "" {
			return print(cmd, o, output.Failure("invalid_args", "--app-id or --app-url is required", "Pass the BrowserStack app id or bs:// app URL.", 400))
		}
		if !yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes is required for app delete", "Re-run with --yes after confirming deletion.", 400))
		}
		id := appID
		if id == "" {
			id = browserstack.AppIDFromURL(appURL)
		}
		if dryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "app_id": id}))
		}
		svc, err := newServices(o, true)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		out, err := svc.Control.DeleteApp(cmd.Context(), id)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", out))
	}}
	c.Flags().StringVar(&appID, "app-id", "", "")
	c.Flags().StringVar(&appURL, "app-url", "", "")
	c.Flags().BoolVar(&yes, "yes", false, "")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "")
	return c
}

func appLaunchCmd(o *Opts) *cobra.Command {
	var runID string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "launch", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, runID, "app_launch", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return nil, svc.Appium.LaunchApp(ctx, st.SessionID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}

func appCloseCmd(o *Opts) *cobra.Command {
	var runID string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "close", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, runID, "app_close", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return nil, svc.Appium.CloseApp(ctx, st.SessionID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}

func appResetCmd(o *Opts) *cobra.Command {
	var runID string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "reset", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the active run id.", 400))
		}
		return runGesture(cmd, o, runID, "app_reset", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return nil, svc.Appium.ResetApp(ctx, st.SessionID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}

func appActivateCmd(o *Opts) *cobra.Command {
	var runID, appID string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "activate", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" || appID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id and --app-id are required", "Pass the package name or bundle id to activate.", 400))
		}
		return runGesture(cmd, o, runID, "app_activate", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return map[string]any{"app_id": appID}, svc.Appium.ActivateApp(ctx, st.SessionID, appID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&appID, "app-id", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}

func appTerminateCmd(o *Opts) *cobra.Command {
	var runID, appID string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "terminate", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" || appID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id and --app-id are required", "Pass the package name or bundle id to terminate.", 400))
		}
		return runGesture(cmd, o, runID, "app_terminate", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return map[string]any{"app_id": appID}, svc.Appium.TerminateApp(ctx, st.SessionID, appID)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&appID, "app-id", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}

func appDeepLinkCmd(o *Opts) *cobra.Command {
	var runID, link, packageName string
	actionOpts := defaultActionOptions()
	c := &cobra.Command{Use: "deep-link", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" || link == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id and --url are required", "Pass the deep link URL and, on Android, --package.", 400))
		}
		return runGesture(cmd, o, runID, "app_deep_link", actionOpts, func(ctx context.Context, svc *services, st *mobile.RunState) (map[string]any, error) {
			return map[string]any{"url": link, "package": packageName}, svc.Appium.DeepLink(ctx, st.SessionID, link, packageName)
		})
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&link, "url", "", "")
	c.Flags().StringVar(&packageName, "package", "", "")
	bindActionOptions(c, &actionOpts, false)
	return c
}
