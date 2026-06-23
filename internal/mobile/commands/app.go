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
	c.AddCommand(appUploadCmd(o), appListCmd(o), appGetCmd(o), appResolveCmd(o), appDeleteCmd(o))
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
		_ = svc.Store.SaveAppCache(mobile.AppRef{AppURL: app.AppURL, CustomID: customID, SHA256: sha, Name: filepath.Base(file)})
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
		if strings.HasPrefix(appURL, "bs://") {
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
			if cached, err := svc.Store.LoadAppCache(sha); err == nil && cached.AppURL != "" {
				return print(cmd, o, output.Success("", map[string]any{"app": cached, "reused": true, "source": "local_cache"}))
			}
		}
		if customID != "" {
			apps, err := svc.Control.ListApps(cmd.Context(), browserstack.ListAppsRequest{Limit: 20, CustomID: customID})
			if err == nil {
				for _, app := range apps {
					if app.AppURL != "" {
						ref := mobile.AppRef{AppURL: app.AppURL, CustomID: customID, SHA256: sha, Name: app.AppName}
						_ = svc.Store.SaveAppCache(ref)
						return print(cmd, o, output.Success("", map[string]any{"app": ref, "reused": true, "source": "browserstack_recent_apps"}))
					}
				}
			}
		}
		if dryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "would_upload": true, "file": file, "url": remoteURL, "custom_id": customID, "sha256": sha}))
		}
		app, err := svc.Control.UploadApp(cmd.Context(), browserstack.UploadAppRequest{FilePath: file, URL: remoteURL, CustomID: customID, SHA256: sha})
		if err != nil {
			return renderErr(cmd, o, err)
		}
		ref := mobile.AppRef{AppURL: app.AppURL, CustomID: customID, SHA256: sha, Name: app.AppName}
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
