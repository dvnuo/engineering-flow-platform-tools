package vision

import "testing"

func TestParseOutputTextJSON(t *testing.T) {
	p, err := ParseResponse(map[string]any{"output_text": `{"answer":"ok"}`})
	if err != nil {
		t.Fatal(err)
	}
	if p.Result.(map[string]any)["answer"] != "ok" {
		t.Fatalf("bad parse: %#v", p)
	}
}

func TestParseInvalidJSONKeepsRawText(t *testing.T) {
	p, err := ParseResponse(map[string]any{"output_text": "plain answer"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) != 1 || p.Result.(map[string]any)["raw_text"] != "plain answer" {
		t.Fatalf("bad fallback: %#v", p)
	}
}
