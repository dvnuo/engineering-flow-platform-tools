package automation

import (
	"context"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
)

type TableOptions struct {
	PageOptions
	Selector    string
	LimitRows   int
	LimitCells  int
	IncludeHTML bool
}

type TableCell struct {
	Index   int    `json:"index"`
	Text    string `json:"text,omitempty"`
	Header  bool   `json:"header,omitempty"`
	ColSpan int    `json:"col_span,omitempty"`
	RowSpan int    `json:"row_span,omitempty"`
}

type TableRow struct {
	Index int         `json:"index"`
	Cells []TableCell `json:"cells"`
}

type PageTable struct {
	Index       int        `json:"index"`
	Caption     string     `json:"caption,omitempty"`
	Headers     []string   `json:"headers,omitempty"`
	Rows        []TableRow `json:"rows"`
	RowCount    int        `json:"row_count"`
	ColumnCount int        `json:"column_count"`
	Selector    string     `json:"selector_hint,omitempty"`
	HTMLPreview string     `json:"html_preview,omitempty"`
	HTMLLength  int        `json:"html_length,omitempty"`
}

type TableResult struct {
	Session     string      `json:"session"`
	TargetID    string      `json:"target_id"`
	URL         string      `json:"url"`
	Title       string      `json:"title"`
	Selector    string      `json:"selector,omitempty"`
	LimitRows   int         `json:"limit_rows"`
	LimitCells  int         `json:"limit_cells"`
	IncludeHTML bool        `json:"include_html,omitempty"`
	Count       int         `json:"count"`
	Tables      []PageTable `json:"tables"`
}

type PageListOptions struct {
	PageOptions
	Selector   string
	LimitItems int
}

type PageListItem struct {
	Index int    `json:"index"`
	Text  string `json:"text,omitempty"`
	Href  string `json:"href,omitempty"`
	Level int    `json:"level,omitempty"`
}

type PageList struct {
	Index     int            `json:"index"`
	Kind      string         `json:"kind,omitempty"`
	Ordered   bool           `json:"ordered,omitempty"`
	Selector  string         `json:"selector_hint,omitempty"`
	ItemCount int            `json:"item_count"`
	Items     []PageListItem `json:"items"`
}

type PageListResult struct {
	Session    string     `json:"session"`
	TargetID   string     `json:"target_id"`
	URL        string     `json:"url"`
	Title      string     `json:"title"`
	Selector   string     `json:"selector,omitempty"`
	LimitItems int        `json:"limit_items"`
	Count      int        `json:"count"`
	Lists      []PageList `json:"lists"`
}

func (m *Manager) Table(ctx context.Context, opts TableOptions) (TableResult, error) {
	opts = normalizeTableOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return TableResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw struct {
		Count  int         `json:"count"`
		Tables []PageTable `json:"tables"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(tableExtractionExpression(opts), &raw, chromedp.EvalAsValue),
	); err != nil {
		return TableResult{}, mapPageError(err, "automation_failed")
	}
	return TableResult{
		Session:     session.Name,
		TargetID:    target.ID,
		URL:         RedactURL(finalURL),
		Title:       RedactString(title),
		Selector:    RedactString(opts.Selector),
		LimitRows:   opts.LimitRows,
		LimitCells:  opts.LimitCells,
		IncludeHTML: opts.IncludeHTML,
		Count:       raw.Count,
		Tables:      sanitizePageTables(raw.Tables, opts.IncludeHTML),
	}, nil
}

func (m *Manager) PageList(ctx context.Context, opts PageListOptions) (PageListResult, error) {
	opts = normalizePageListOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return PageListResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw struct {
		Count int        `json:"count"`
		Lists []PageList `json:"lists"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(listExtractionExpression(opts), &raw, chromedp.EvalAsValue),
	); err != nil {
		return PageListResult{}, mapPageError(err, "automation_failed")
	}
	return PageListResult{
		Session:    session.Name,
		TargetID:   target.ID,
		URL:        RedactURL(finalURL),
		Title:      RedactString(title),
		Selector:   RedactString(opts.Selector),
		LimitItems: opts.LimitItems,
		Count:      raw.Count,
		Lists:      sanitizePageLists(raw.Lists),
	}, nil
}

func normalizeTableOptions(opts TableOptions) TableOptions {
	if opts.LimitRows <= 0 {
		opts.LimitRows = 50
	}
	if opts.LimitCells <= 0 {
		opts.LimitCells = 20
	}
	return opts
}

func normalizePageListOptions(opts PageListOptions) PageListOptions {
	if opts.LimitItems <= 0 {
		opts.LimitItems = 100
	}
	return opts
}

func sanitizePageTables(raw []PageTable, includeHTML bool) []PageTable {
	out := make([]PageTable, len(raw))
	for i, table := range raw {
		table.Caption = TruncateBytes(RedactString(table.Caption), 1000)
		for j, header := range table.Headers {
			table.Headers[j] = TruncateBytes(RedactString(header), 1000)
		}
		for r := range table.Rows {
			for c := range table.Rows[r].Cells {
				table.Rows[r].Cells[c].Text = TruncateBytes(RedactString(table.Rows[r].Cells[c].Text), 1000)
			}
		}
		table.Selector = normalizeSelectorHint(table.Selector)
		if includeHTML && table.HTMLPreview != "" {
			table.HTMLPreview = TruncateBytes(RedactString(table.HTMLPreview), 20000)
		} else {
			table.HTMLPreview = ""
			table.HTMLLength = 0
		}
		out[i] = table
	}
	return out
}

func sanitizePageLists(raw []PageList) []PageList {
	out := make([]PageList, len(raw))
	for i, list := range raw {
		list.Kind = strings.ToLower(RedactString(list.Kind))
		list.Selector = normalizeSelectorHint(list.Selector)
		for j := range list.Items {
			list.Items[j].Text = TruncateBytes(RedactString(list.Items[j].Text), 1000)
			list.Items[j].Href = RedactURL(list.Items[j].Href)
		}
		out[i] = list
	}
	return out
}

func tableExtractionExpression(opts TableOptions) string {
	return `(function () {
  const selector = ` + strconv.Quote(strings.TrimSpace(opts.Selector)) + `;
  const limitRows = ` + strconv.Itoa(opts.LimitRows) + `;
  const limitCells = ` + strconv.Itoa(opts.LimitCells) + `;
  const includeHTML = ` + strconv.FormatBool(opts.IncludeHTML) + `;
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const textOf = (el, max) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim().slice(0, max);
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
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
  const selected = selector ? Array.from(document.querySelectorAll(selector)) : Array.from(document.querySelectorAll("table"));
  const tables = [];
  for (const node of selected) {
    if (String(node.tagName || "").toLowerCase() === "table") tables.push(node);
    for (const table of Array.from(node.querySelectorAll ? node.querySelectorAll("table") : [])) tables.push(table);
  }
  const unique = Array.from(new Set(tables));
  const output = [];
  for (let tableIndex = 0; tableIndex < unique.length; tableIndex++) {
    const table = unique[tableIndex];
    const rows = Array.from(table.querySelectorAll("tr"));
    const columnCount = Math.max(0, ...rows.map(row => Array.from(row.children).reduce((sum, cell) => sum + Math.max(1, Number(cell.colSpan || 1)), 0)));
    const headerCells = Array.from(table.querySelectorAll("thead th"));
    const fallbackHeaders = headerCells.length ? [] : Array.from((rows[0] && rows[0].querySelectorAll("th")) || []);
    const headers = (headerCells.length ? headerCells : fallbackHeaders).slice(0, limitCells).map(cell => textOf(cell, 1000));
    const bodyRows = Array.from(table.tBodies || []).flatMap(body => Array.from(body.rows || []));
    const dataRows = bodyRows.length ? bodyRows : rows;
    const extractedRows = dataRows.slice(0, limitRows).map((row, rowIndex) => ({
      index: rowIndex,
      cells: Array.from(row.children).slice(0, limitCells).map((cell, cellIndex) => ({
        index: cellIndex,
        text: textOf(cell, 1000),
        header: String(cell.tagName || "").toLowerCase() === "th",
        col_span: Math.max(1, Number(cell.colSpan || 1)),
        row_span: Math.max(1, Number(cell.rowSpan || 1))
      }))
    }));
    const html = includeHTML ? String(table.outerHTML || "") : "";
    output.push({
      index: tableIndex,
      caption: textOf(table.querySelector("caption"), 1000),
      headers,
      rows: extractedRows,
      row_count: rows.length,
      column_count: columnCount,
      selector_hint: selectorFor(table),
      html_preview: includeHTML ? html.slice(0, 20000) : "",
      html_length: includeHTML ? html.length : 0
    });
  }
  return {count: unique.length, tables: output};
})()`
}

func listExtractionExpression(opts PageListOptions) string {
	return `(function () {
  const selector = ` + strconv.Quote(strings.TrimSpace(opts.Selector)) + `;
  const limitItems = ` + strconv.Itoa(opts.LimitItems) + `;
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const textOf = (el, max) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim().slice(0, max);
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
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
  const listSelector = "ul,ol,[role='list']";
  const selected = selector ? Array.from(document.querySelectorAll(selector)) : Array.from(document.querySelectorAll(listSelector));
  const lists = [];
  for (const node of selected) {
    const tag = String(node.tagName || "").toLowerCase();
    const role = attr(node, "role").toLowerCase();
    if (tag === "ul" || tag === "ol" || role === "list") lists.push(node);
    for (const list of Array.from(node.querySelectorAll ? node.querySelectorAll(listSelector) : [])) lists.push(list);
  }
  const unique = Array.from(new Set(lists));
  const output = unique.map((list, listIndex) => {
    const tag = String(list.tagName || "").toLowerCase();
    const role = attr(list, "role").toLowerCase();
    const rawItems = Array.from(list.querySelectorAll("li,[role='listitem']"));
    const items = rawItems.slice(0, limitItems).map((item, itemIndex) => {
      let level = 1;
      let node = item.parentElement;
      while (node && node !== list) {
        const nodeTag = String(node.tagName || "").toLowerCase();
        const nodeRole = attr(node, "role").toLowerCase();
        if (nodeTag === "ul" || nodeTag === "ol" || nodeRole === "list") level++;
        node = node.parentElement;
      }
      const link = item.querySelector("a[href]");
      return {
        index: itemIndex,
        text: textOf(item, 1000),
        href: link ? String(link.href || attr(link, "href")) : "",
        level
      };
    });
    return {
      index: listIndex,
      kind: tag === "ol" ? "ordered" : "unordered",
      ordered: tag === "ol",
      selector_hint: selectorFor(list),
      item_count: rawItems.length,
      items
    };
  });
  return {count: unique.length, lists: output};
})()`
}
