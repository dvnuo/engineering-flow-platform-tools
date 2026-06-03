package logtool

import (
	"strings"
	"testing"
)

func TestParseStreamPlainAndStacktrace(t *testing.T) {
	input := strings.Join([]string{
		"2026-06-03T10:00:00Z INFO service started",
		"2026-06-03T10:01:00Z ERROR database password=secret timeout after 3000ms",
		"java.lang.RuntimeException: boom",
		"    at example.Main.main(Main.java:10)",
	}, "\n") + "\n"
	var events []ParsedEvent
	result, err := ParseStream(strings.NewReader(input), "app.log", ParseOptions{FormatHint: "auto", MaxLineBytes: 65536}, func(ev ParsedEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Lines != 4 || len(events) != 2 {
		t.Fatalf("lines=%d events=%d %#v", result.Lines, len(events), events)
	}
	if events[1].Level != "ERROR" || events[1].LineStart != 2 || events[1].LineEnd != 4 {
		t.Fatalf("bad error event: %#v", events[1])
	}
	if strings.Contains(events[1].Message, "secret") {
		t.Fatalf("secret leaked in parsed message: %s", events[1].Message)
	}
}

func TestParseStreamJSONL(t *testing.T) {
	input := `{"timestamp":"2026-06-03T10:00:00Z","level":"warning","service":{"name":"api"},"message":"slow request took 123ms"}` + "\n"
	var events []ParsedEvent
	_, err := ParseStream(strings.NewReader(input), "app.jsonl", ParseOptions{FormatHint: "json", MaxLineBytes: 65536}, func(ev ParsedEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events=%d", len(events))
	}
	if events[0].Timestamp != "2026-06-03T10:00:00Z" || events[0].Level != "WARN" || events[0].Service != "api" || events[0].Message != "slow request took 123ms" {
		t.Fatalf("bad json event: %#v", events[0])
	}
}

func TestParseStreamBoundsVeryLongSingleLine(t *testing.T) {
	line := strings.Repeat("A", 2*1024*1024)
	var events []ParsedEvent
	result, err := ParseStream(strings.NewReader(line), "huge.log", ParseOptions{FormatHint: "plain", MaxLineBytes: 1024}, func(ev ParsedEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events=%d", len(events))
	}
	if result.Bytes != int64(len(line)) || events[0].ByteEnd != int64(len(line)) {
		t.Fatalf("bad byte accounting result=%#v event=%#v want=%d", result, events[0], len(line))
	}
	if !strings.Contains(events[0].Raw, lineTruncatedMarker) || !strings.Contains(events[0].Message, lineTruncatedMarker) {
		t.Fatalf("missing line truncation marker: %#v", events[0])
	}
	if len(events[0].Message) >= len(line)/10 {
		t.Fatalf("message preview was not bounded: got=%d source=%d", len(events[0].Message), len(line))
	}
}

func TestParseStreamCapsHugeMultilineEventPreview(t *testing.T) {
	var b strings.Builder
	b.WriteString("2026-06-03T10:00:00Z ERROR first line\n")
	for i := 0; i < int(defaultMaxEventLines)+50; i++ {
		b.WriteString("continuation line\n")
	}
	input := b.String()
	var events []ParsedEvent
	result, err := ParseStream(strings.NewReader(input), "huge-event.log", ParseOptions{FormatHint: "plain", MaxLineBytes: 65536}, func(ev ParsedEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events=%d", len(events))
	}
	wantLines := defaultMaxEventLines + 51
	if result.Lines != wantLines || events[0].LineEnd != wantLines {
		t.Fatalf("bad line accounting result=%#v event=%#v want=%d", result, events[0], wantLines)
	}
	if events[0].ByteEnd != int64(len(input)) {
		t.Fatalf("bad byte end=%d want=%d", events[0].ByteEnd, len(input))
	}
	if !strings.Contains(events[0].Message, eventTruncatedMarker) {
		t.Fatalf("missing event truncation marker: %s", events[0].Message)
	}
}
