package automation

import (
	"strings"
	"testing"
)

func TestNormalizeStructuredExtractionOptions(t *testing.T) {
	table := normalizeTableOptions(TableOptions{})
	if table.LimitRows != 50 || table.LimitCells != 20 {
		t.Fatalf("table defaults = %#v", table)
	}
	list := normalizePageListOptions(PageListOptions{})
	if list.LimitItems != 100 {
		t.Fatalf("list defaults = %#v", list)
	}
}

func TestSanitizePageTablesRedactsCellsHeadersAndHTML(t *testing.T) {
	raw := []PageTable{{
		Index:    0,
		Caption:  "session=abc",
		Headers:  []string{"token=secret"},
		Selector: "table#access_token=secret",
		Rows: []TableRow{{
			Index: 0,
			Cells: []TableCell{{
				Index: 0,
				Text:  "Authorization: Bearer private",
			}},
		}},
		HTMLPreview: `<table data-code="abc"><td>token=secret</td></table>`,
		HTMLLength:  52,
	}}
	got := sanitizePageTables(raw, true)
	table := got[0]
	joined := table.Caption + strings.Join(table.Headers, " ") + table.Rows[0].Cells[0].Text + table.Selector + table.HTMLPreview
	for _, leaked := range []string{"session=abc", "Bearer private", "data-code=\"abc\""} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("table leaked %q in %#v", leaked, table)
		}
	}
	if table.HTMLPreview == "" || table.HTMLLength == 0 {
		t.Fatalf("expected redacted html preview to be retained: %#v", table)
	}
}

func TestSanitizePageTablesDropsHTMLWhenNotRequested(t *testing.T) {
	got := sanitizePageTables([]PageTable{{HTMLPreview: "<table></table>", HTMLLength: 15}}, false)
	if got[0].HTMLPreview != "" || got[0].HTMLLength != 0 {
		t.Fatalf("html should be omitted: %#v", got[0])
	}
}

func TestSanitizePageListsRedactsItemsAndHrefs(t *testing.T) {
	raw := []PageList{{
		Kind:     "ORDERED",
		Selector: "ol#session=abc",
		Items: []PageListItem{{
			Index: 0,
			Text:  "token=secret",
			Href:  "https://intranet.test/cb?code=abc",
			Level: 2,
		}},
	}}
	got := sanitizePageLists(raw)
	list := got[0]
	joined := list.Kind + list.Selector + list.Items[0].Text + list.Items[0].Href
	for _, leaked := range []string{"session=abc", "secret", "code=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("list leaked %q in %#v", leaked, list)
		}
	}
	if list.Kind != "ordered" || list.Items[0].Level != 2 {
		t.Fatalf("list fields changed unexpectedly: %#v", list)
	}
}
