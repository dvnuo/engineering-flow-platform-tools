package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type ChromeDPRunner struct{}

func NewChromeDPRunner() *ChromeDPRunner {
	return &ChromeDPRunner{}
}

func (r *ChromeDPRunner) Probe(ctx context.Context, opts ProbeOptions) (ProbeResult, error) {
	if err := validateProbeOptions(opts); err != nil {
		return ProbeResult{}, err
	}
	if opts.OutDir == "" {
		opts.OutDir = "result"
	}
	if opts.ProfileDir == "" {
		opts.ProfileDir = DefaultProfileDir()
	}
	if UnsafeProfileDir(opts.ProfileDir) {
		return ProbeResult{}, &ProbeError{Code: "invalid_args", Message: "--profile must point at a dedicated directory, not a filesystem root", Hint: "Use a dedicated probe profile directory.", Status: 400}
	}
	if LooksLikeDefaultBrowserProfile(opts.ProfileDir) {
		return ProbeResult{}, &ProbeError{Code: "invalid_args", Message: "--profile must not point at a default Edge/Chrome/Chromium profile", Hint: "Use a dedicated probe profile directory.", Status: 400}
	}
	if opts.MaxNetworkEvents <= 0 {
		opts.MaxNetworkEvents = 1000
	}

	if err := os.MkdirAll(opts.OutDir, 0o700); err != nil {
		return ProbeResult{}, artifactWriteError(err)
	}
	if opts.CleanProfile {
		if err := os.RemoveAll(opts.ProfileDir); err != nil {
			return ProbeResult{}, artifactWriteError(err)
		}
	}
	if err := os.MkdirAll(opts.ProfileDir, 0o700); err != nil {
		return ProbeResult{}, artifactWriteError(err)
	}

	browserPath, err := FindBrowser(opts.Browser, opts.BrowserExe)
	if err != nil {
		return ProbeResult{}, err
	}

	var eventsMu sync.Mutex
	events := make([]NetworkEvent, 0)
	totalEvents := 0
	requestMethods := map[string]string{}

	allocOpts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(browserPath),
		chromedp.UserDataDir(opts.ProfileDir),
		chromedp.Flag("headless", opts.Headless),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("window-size", "1366,900"),
	}
	if opts.IgnoreCertErrors {
		allocOpts = append(allocOpts, chromedp.Flag("ignore-certificate-errors", true))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	chromedp.ListenTarget(browserCtx, func(ev any) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			totalEvents++
			reqURL := ""
			method := ""
			if e.Request != nil {
				reqURL = e.Request.URL
				if e.Request.URLFragment != "" {
					reqURL += e.Request.URLFragment
				}
				method = e.Request.Method
			}
			requestMethods[e.RequestID.String()] = method
			if e.RedirectResponse != nil {
				totalEvents++
				appendNetworkEvent(&events, opts.MaxNetworkEvents, NetworkEvent{
					Kind:         "response",
					Time:         time.Now().UTC().Format(time.RFC3339Nano),
					RequestID:    e.RequestID.String(),
					Method:       method,
					URL:          RedactURL(e.RedirectResponse.URL),
					ResourceType: e.Type.String(),
					Status:       int(e.RedirectResponse.Status),
					MimeType:     e.RedirectResponse.MimeType,
				})
			}
			appendNetworkEvent(&events, opts.MaxNetworkEvents, NetworkEvent{
				Kind:         "request",
				Time:         time.Now().UTC().Format(time.RFC3339Nano),
				RequestID:    e.RequestID.String(),
				Method:       method,
				URL:          RedactURL(reqURL),
				ResourceType: e.Type.String(),
			})
		case *network.EventResponseReceived:
			totalEvents++
			respURL := ""
			status := 0
			mimeType := ""
			if e.Response != nil {
				respURL = e.Response.URL
				status = int(e.Response.Status)
				mimeType = e.Response.MimeType
			}
			appendNetworkEvent(&events, opts.MaxNetworkEvents, NetworkEvent{
				Kind:         "response",
				Time:         time.Now().UTC().Format(time.RFC3339Nano),
				RequestID:    e.RequestID.String(),
				Method:       requestMethods[e.RequestID.String()],
				URL:          RedactURL(respURL),
				ResourceType: e.Type.String(),
				Status:       status,
				MimeType:     mimeType,
			})
		}
	})

	if err := chromedp.Run(browserCtx); err != nil {
		return ProbeResult{}, mapProbeRunError(err, "browser_launch_failed")
	}
	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		return ProbeResult{}, mapProbeRunError(err, "network_capture_failed")
	}
	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(opts.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(time.Duration(opts.WaitSeconds)*time.Second),
	); err != nil {
		return ProbeResult{}, mapProbeRunError(err, "navigation_failed")
	}

	selectorFound := false
	if opts.Selector != "" {
		selectorCtx, cancel := context.WithTimeout(browserCtx, 12*time.Second)
		selectorErr := chromedp.Run(selectorCtx, chromedp.WaitVisible(opts.Selector, chromedp.ByQuery))
		cancel()
		selectorFound = selectorErr == nil
	}

	var fetchResult map[string]any
	var fetchErr error
	if opts.FetchAPI != "" {
		fetchResult, fetchErr = runFetchAPI(browserCtx, opts.FetchAPI)
	}

	var finalURL, title, html, bodyText string
	var screenshot []byte
	actions := []chromedp.Action{
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
		chromedp.Text("body", &bodyText, chromedp.ByQuery),
	}
	if opts.SaveScreenshot {
		actions = append(actions, chromedp.FullScreenshot(&screenshot, 90))
	}
	if err := chromedp.Run(browserCtx, actions...); err != nil {
		return ProbeResult{}, mapProbeRunError(err, "navigation_failed")
	}

	eventsMu.Lock()
	capturedEvents := append([]NetworkEvent{}, events...)
	networkCount := totalEvents
	eventsMu.Unlock()

	bodyPreview := truncate(RedactText(bodyText), 3000)
	files := ArtifactPaths(opts.OutDir, opts.SaveHTML, opts.SaveScreenshot, opts.FetchAPI != "")
	result := ProbeResult{
		InputURL:       RedactURL(opts.URL),
		FinalURL:       RedactURL(finalURL),
		Title:          title,
		Selector:       opts.Selector,
		SelectorFound:  selectorFound,
		ProfileDir:     opts.ProfileDir,
		BrowserPath:    browserPath,
		OutDir:         opts.OutDir,
		BodyPreview:    bodyPreview,
		APIEvents:      FilterAPIEvents(capturedEvents, opts.NetworkFilter, opts.MaxNetworkEvents),
		NetworkCount:   networkCount,
		Files:          files,
		FetchAPIResult: fetchResult,
	}
	result.AuthIndicators = ClassifyAuthIndicators(opts.URL, finalURL, title, bodyPreview, selectorFound, capturedEvents)

	if err := writeArtifacts(files, opts, result, capturedEvents, html, screenshot, fetchResult); err != nil {
		return result, artifactWriteError(err)
	}
	if fetchErr != nil {
		return result, &ProbeError{Code: "fetch_api_failed", Message: fetchErr.Error(), Hint: "Inspect fetch_api_result.json and network.json.", Status: 502}
	}
	if opts.RequireSelector && opts.Selector != "" && !selectorFound {
		return result, &ProbeError{
			Code:    "selector_not_found",
			Message: "Selector was not found.",
			Hint:    "Selector was not found. Inspect the generated summary.json, screenshot.png, and page.html.",
			Status:  404,
		}
	}
	return result, nil
}

func validateProbeOptions(opts ProbeOptions) error {
	u, err := url.Parse(opts.URL)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return &ProbeError{Code: "invalid_args", Message: "--url must be an http or https URL", Hint: "Run browser schema probe --json.", Status: 400}
	}
	if opts.WaitSeconds < 0 {
		return &ProbeError{Code: "invalid_args", Message: "--wait must be zero or greater", Hint: "Run browser schema probe --json.", Status: 400}
	}
	if opts.TimeoutSeconds < 0 {
		return &ProbeError{Code: "invalid_args", Message: "--timeout must be zero or greater", Hint: "Run browser schema probe --json.", Status: 400}
	}
	return nil
}

func appendNetworkEvent(events *[]NetworkEvent, max int, event NetworkEvent) {
	if max <= 0 {
		max = 1000
	}
	if len(*events) >= max {
		return
	}
	*events = append(*events, event)
}

func runFetchAPI(ctx context.Context, target string) (map[string]any, error) {
	var result map[string]any
	expr := fmt.Sprintf(`(async () => {
  const target = %s;
  try {
    const res = await fetch(target, { credentials: "include" });
    const body = await res.text();
    return {
      ok: res.ok,
      status: res.status,
      url: res.url,
      contentType: res.headers.get("content-type") || "",
      bodyPreview: body.slice(0, 20000)
    };
  } catch (err) {
    return {
      ok: false,
      status: 0,
      url: target,
      contentType: "",
      bodyPreview: "",
      error: String(err)
    };
  }
})()`, strconv.Quote(target))
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &result, chromedp.EvalAsValue)); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("fetch returned no result")
	}
	if rawURL, _ := result["url"].(string); rawURL != "" {
		result["url"] = RedactURL(rawURL)
	}
	if preview, _ := result["bodyPreview"].(string); preview != "" {
		result["bodyPreview"] = truncate(RedactText(preview), 20000)
	}
	if status, _ := result["status"].(float64); status == 0 {
		if msg, _ := result["error"].(string); msg != "" {
			return result, errors.New(msg)
		}
	}
	return result, nil
}

func writeArtifacts(files ProbeFiles, opts ProbeOptions, result ProbeResult, networkEvents []NetworkEvent, html string, screenshot []byte, fetchResult map[string]any) error {
	if files.Screenshot != "" && opts.SaveScreenshot {
		if err := os.WriteFile(files.Screenshot, screenshot, 0o600); err != nil {
			return err
		}
	}
	if files.HTML != "" && opts.SaveHTML {
		if err := os.WriteFile(files.HTML, []byte(html), 0o600); err != nil {
			return err
		}
	}
	if err := writeJSON(files.Network, networkEvents); err != nil {
		return err
	}
	if files.FetchAPI != "" && fetchResult != nil {
		if err := writeJSON(files.FetchAPI, fetchResult); err != nil {
			return err
		}
	}
	return writeJSON(files.Summary, result)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func artifactWriteError(err error) *ProbeError {
	return &ProbeError{Code: "artifact_write_failed", Message: err.Error(), Hint: "Check --out permissions and available disk space.", Status: 500}
}

func mapProbeRunError(err error, fallback string) *ProbeError {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &ProbeError{Code: "timeout", Message: err.Error(), Hint: "Increase --timeout or check browser/network availability.", Status: 408}
	}
	switch fallback {
	case "browser_launch_failed":
		return &ProbeError{Code: "browser_launch_failed", Message: err.Error(), Hint: "Check --browser-exe and whether the browser can be launched.", Status: 500}
	case "network_capture_failed":
		return &ProbeError{Code: "network_capture_failed", Message: err.Error(), Hint: "The browser launched, but DevTools network capture could not be enabled.", Status: 500}
	case "navigation_failed":
		return &ProbeError{Code: "navigation_failed", Message: err.Error(), Hint: "Check the URL, TLS, proxy, and browser policy prompts.", Status: 502}
	default:
		return &ProbeError{Code: "server_error", Message: err.Error(), Status: 500}
	}
}
