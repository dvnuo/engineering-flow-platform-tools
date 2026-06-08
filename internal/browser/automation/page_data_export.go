package automation

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type DataExportOptions struct {
	PageOptions
	Selector   string
	OutPath    string
	Format     string
	LimitRows  int
	LimitCells int
	LimitItems int
}

type DataExportResult struct {
	Session   string    `json:"session"`
	TargetID  string    `json:"target_id"`
	URL       string    `json:"url,omitempty"`
	Title     string    `json:"title,omitempty"`
	Path      string    `json:"path"`
	Format    string    `json:"format"`
	Kind      string    `json:"kind"`
	Count     int       `json:"count"`
	Bytes     int64     `json:"bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScrollCollectOptions struct {
	PageOptions
	Selector             string
	ItemSelector         string
	OutPath              string
	Format               string
	Limit                int
	MaxScrolls           int
	ScrollStep           int
	IntervalMilliseconds int
}

type ScrollCollectItem struct {
	Index    int    `json:"index"`
	Text     string `json:"text,omitempty"`
	Href     string `json:"href,omitempty"`
	Selector string `json:"selector_hint,omitempty"`
}

type ScrollCollectResult struct {
	Session    string              `json:"session"`
	TargetID   string              `json:"target_id"`
	URL        string              `json:"url"`
	Title      string              `json:"title"`
	Path       string              `json:"path,omitempty"`
	Format     string              `json:"format,omitempty"`
	Count      int                 `json:"count"`
	Bytes      int64               `json:"bytes,omitempty"`
	Limit      int                 `json:"limit"`
	MaxScrolls int                 `json:"max_scrolls"`
	Items      []ScrollCollectItem `json:"items,omitempty"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

func (m *Manager) TableExport(ctx context.Context, opts DataExportOptions) (DataExportResult, error) {
	opts, err := normalizeDataExportOptions(opts, "table")
	if err != nil {
		return DataExportResult{}, err
	}
	table, err := m.Table(ctx, TableOptions{PageOptions: opts.PageOptions, Selector: opts.Selector, LimitRows: opts.LimitRows, LimitCells: opts.LimitCells})
	if err != nil {
		return DataExportResult{}, err
	}
	var b []byte
	if opts.Format == "json" {
		b, err = json.MarshalIndent(table, "", "  ")
	} else {
		b, err = tableCSV(table)
	}
	if err != nil {
		return DataExportResult{}, NewError("automation_failed", err.Error(), "Table export could not be encoded.", 500)
	}
	path, size, err := writeExportArtifact(opts.OutPath, b)
	if err != nil {
		return DataExportResult{}, err
	}
	return DataExportResult{Session: table.Session, TargetID: table.TargetID, URL: table.URL, Title: table.Title, Path: path, Format: opts.Format, Kind: "table", Count: table.Count, Bytes: size, UpdatedAt: m.now()}, nil
}

func (m *Manager) ListExport(ctx context.Context, opts DataExportOptions) (DataExportResult, error) {
	opts, err := normalizeDataExportOptions(opts, "list")
	if err != nil {
		return DataExportResult{}, err
	}
	list, err := m.PageList(ctx, PageListOptions{PageOptions: opts.PageOptions, Selector: opts.Selector, LimitItems: opts.LimitItems})
	if err != nil {
		return DataExportResult{}, err
	}
	var b []byte
	if opts.Format == "json" {
		b, err = json.MarshalIndent(list, "", "  ")
	} else {
		b, err = listCSV(list)
	}
	if err != nil {
		return DataExportResult{}, NewError("automation_failed", err.Error(), "List export could not be encoded.", 500)
	}
	path, size, err := writeExportArtifact(opts.OutPath, b)
	if err != nil {
		return DataExportResult{}, err
	}
	return DataExportResult{Session: list.Session, TargetID: list.TargetID, URL: list.URL, Title: list.Title, Path: path, Format: opts.Format, Kind: "list", Count: list.Count, Bytes: size, UpdatedAt: m.now()}, nil
}

func (m *Manager) ScrollCollect(ctx context.Context, opts ScrollCollectOptions) (ScrollCollectResult, error) {
	opts, err := normalizeScrollCollectOptions(opts)
	if err != nil {
		return ScrollCollectResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return ScrollCollectResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw struct {
		Items []ScrollCollectItem `json:"items"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(scrollCollectExpression(opts), &raw, chromedp.EvalAsValue, evalAwaitPromise),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return ScrollCollectResult{}, mapPageError(err, "automation_failed")
	}
	items := sanitizeScrollCollectItems(raw.Items)
	result := ScrollCollectResult{
		Session:    session.Name,
		TargetID:   target.ID,
		URL:        RedactURL(finalURL),
		Title:      RedactString(title),
		Format:     opts.Format,
		Count:      len(items),
		Limit:      opts.Limit,
		MaxScrolls: opts.MaxScrolls,
		Items:      items,
		UpdatedAt:  m.now(),
	}
	if strings.TrimSpace(opts.OutPath) == "" {
		return result, nil
	}
	var b []byte
	if opts.Format == "json" {
		b, err = json.MarshalIndent(result, "", "  ")
	} else {
		b, err = scrollItemsCSV(items)
	}
	if err != nil {
		return ScrollCollectResult{}, NewError("automation_failed", err.Error(), "Scroll collection export could not be encoded.", 500)
	}
	path, size, err := writeExportArtifact(opts.OutPath, b)
	if err != nil {
		return ScrollCollectResult{}, err
	}
	result.Path = path
	result.Bytes = size
	return result, nil
}

func normalizeDataExportOptions(opts DataExportOptions, kind string) (DataExportOptions, error) {
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if opts.Format == "" {
		opts.Format = "json"
	}
	if opts.Format != "json" && opts.Format != "csv" {
		return opts, invalidArgs("--format must be json or csv", "Pass --format json or --format csv.")
	}
	if strings.TrimSpace(opts.OutPath) == "" {
		return opts, invalidArgs("--out is required", "Pass an output file path for the "+kind+" export.")
	}
	if opts.LimitRows <= 0 {
		opts.LimitRows = 500
	}
	if opts.LimitCells <= 0 {
		opts.LimitCells = 50
	}
	if opts.LimitItems <= 0 {
		opts.LimitItems = 1000
	}
	return opts, nil
}

func normalizeScrollCollectOptions(opts ScrollCollectOptions) (ScrollCollectOptions, error) {
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if opts.Format == "" {
		opts.Format = "json"
	}
	if opts.Format != "json" && opts.Format != "csv" {
		return opts, invalidArgs("--format must be json or csv", "Pass --format json or --format csv.")
	}
	if opts.Limit <= 0 {
		opts.Limit = 500
	}
	if opts.Limit > 10000 {
		opts.Limit = 10000
	}
	if opts.MaxScrolls <= 0 {
		opts.MaxScrolls = 20
	}
	if opts.MaxScrolls > 200 {
		opts.MaxScrolls = 200
	}
	if opts.ScrollStep <= 0 {
		opts.ScrollStep = 900
	}
	if opts.IntervalMilliseconds <= 0 {
		opts.IntervalMilliseconds = 250
	}
	return opts, nil
}

func writeExportArtifact(path string, b []byte) (string, int64, error) {
	path = filepath.Clean(expandHome(strings.TrimSpace(path)))
	if path == "" || path == "." {
		return "", 0, invalidArgs("--out must point at a file", "Pass a writable output path.")
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", 0, NewError("artifact_write_failed", err.Error(), "Check output directory permissions.", 500)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", 0, NewError("artifact_write_failed", err.Error(), "Export artifact could not be written.", 500)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return "", 0, NewError("artifact_write_failed", err.Error(), "Export artifact was written but metadata could not be read.", 500)
	}
	return path, stat.Size(), nil
}

func tableCSV(result TableResult) ([]byte, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write([]string{"table_index", "row_index", "cell_index", "header", "text"}); err != nil {
		return nil, err
	}
	for _, table := range result.Tables {
		for _, row := range table.Rows {
			for _, cell := range row.Cells {
				if err := writer.Write([]string{strconv.Itoa(table.Index), strconv.Itoa(row.Index), strconv.Itoa(cell.Index), strconv.FormatBool(cell.Header), cell.Text}); err != nil {
					return nil, err
				}
			}
		}
	}
	writer.Flush()
	return []byte(builder.String()), writer.Error()
}

func listCSV(result PageListResult) ([]byte, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write([]string{"list_index", "item_index", "level", "text", "href"}); err != nil {
		return nil, err
	}
	for _, list := range result.Lists {
		for _, item := range list.Items {
			if err := writer.Write([]string{strconv.Itoa(list.Index), strconv.Itoa(item.Index), strconv.Itoa(item.Level), item.Text, item.Href}); err != nil {
				return nil, err
			}
		}
	}
	writer.Flush()
	return []byte(builder.String()), writer.Error()
}

func scrollItemsCSV(items []ScrollCollectItem) ([]byte, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write([]string{"index", "text", "href", "selector"}); err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := writer.Write([]string{strconv.Itoa(item.Index), item.Text, item.Href, item.Selector}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return []byte(builder.String()), writer.Error()
}

func sanitizeScrollCollectItems(raw []ScrollCollectItem) []ScrollCollectItem {
	out := make([]ScrollCollectItem, 0, len(raw))
	for i, item := range raw {
		item.Index = i
		item.Text = TruncateBytes(RedactString(item.Text), 2000)
		item.Href = RedactURL(item.Href)
		item.Selector = normalizeSelectorHint(item.Selector)
		out = append(out, item)
	}
	return out
}

func scrollCollectExpression(opts ScrollCollectOptions) string {
	return `(async function () {
  const selector = ` + strconv.Quote(strings.TrimSpace(opts.Selector)) + `;
  const itemSelector = ` + strconv.Quote(strings.TrimSpace(opts.ItemSelector)) + `;
  const limit = ` + strconv.Itoa(opts.Limit) + `;
  const maxScrolls = ` + strconv.Itoa(opts.MaxScrolls) + `;
  const scrollStep = ` + strconv.Itoa(opts.ScrollStep) + `;
  const interval = ` + strconv.Itoa(opts.IntervalMilliseconds) + `;
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
  const textOf = (el, max) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim().slice(0, max);
  const selectorFor = (el) => {
    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && node !== document.documentElement && parts.length < 6) {
      const tag = String(node.tagName || "").toLowerCase();
      const id = attr(node, "id");
      if (id) {
        parts.unshift(tag + "#" + cssEscape(id));
        break;
      }
      let nth = 1;
      let prev = node.previousElementSibling;
      while (prev) {
        if (String(prev.tagName || "").toLowerCase() === tag) nth++;
        prev = prev.previousElementSibling;
      }
      parts.unshift(tag + ":nth-of-type(" + nth + ")");
      node = node.parentElement;
    }
    return parts.join(" > ");
  };
  const root = selector ? document.querySelector(selector) : document.scrollingElement || document.documentElement;
  const readItems = () => {
    const scope = root || document;
    let nodes = [];
    if (itemSelector) {
      nodes = Array.from(scope.querySelectorAll ? scope.querySelectorAll(itemSelector) : []);
    } else {
      nodes = Array.from(scope.querySelectorAll ? scope.querySelectorAll("tr,li,[role='row'],[role='listitem'],article,.item,.row") : []);
      if (!nodes.length) nodes = Array.from(scope.children || []);
    }
    return nodes.map(node => {
      const link = node.querySelector && node.querySelector("a[href]");
      return {text: textOf(node, 2000), href: link ? String(link.href || attr(link, "href")) : "", selector_hint: selectorFor(node)};
    }).filter(item => item.text || item.href);
  };
  const seen = new Set();
  const items = [];
  const addItems = () => {
    for (const item of readItems()) {
      const key = item.text + "\n" + item.href;
      if (seen.has(key)) continue;
      seen.add(key);
      item.index = items.length;
      items.push(item);
      if (items.length >= limit) break;
    }
  };
  const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));
  addItems();
  for (let i = 0; i < maxScrolls && items.length < limit; i++) {
    if (root && root !== document.scrollingElement && root.scrollBy) root.scrollBy(0, scrollStep);
    else window.scrollBy(0, scrollStep);
    await sleep(interval);
    const before = items.length;
    addItems();
    const scroller = root || document.scrollingElement || document.documentElement;
    const atEnd = scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 2;
    if (items.length === before && atEnd) break;
  }
  return {items};
})()`
}
