package appd

import (
	"strings"
	"testing"
)

func TestParseTimestamp(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
	}{
		{"1700000000000", 1700000000000},
		{" 1700000000000 ", 1700000000000},
		{"1700000000", 1700000000000},
		{"2023-11-14T22:13:20Z", 1700000000000},
		{"2023-11-14T23:13:20+01:00", 1700000000000},
		{"2023-11-14T22:13:20.500Z", 1700000000500},
	} {
		got, err := ParseTimestamp(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("ParseTimestamp(%q)=%d err=%v want %d", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "yesterday", "2023-11-14", "-5", "1700000000000ms"} {
		if _, err := ParseTimestamp(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestResolveTimeRange(t *testing.T) {
	cases := []struct {
		name  string
		in    TimeRangeInput
		want  TimeRange
		query map[string]string
	}{
		{"before now", TimeRangeInput{DurationMins: 60}, TimeRange{Type: RangeBeforeNow, DurationInMins: 60}, map[string]string{"time-range-type": "BEFORE_NOW", "duration-in-mins": "60"}},
		{"between", TimeRangeInput{DurationMins: 60, StartTime: "1700000000000", EndTime: "1700003600000"}, TimeRange{Type: RangeBetweenTimes, StartTime: 1700000000000, EndTime: 1700003600000}, map[string]string{"time-range-type": "BETWEEN_TIMES", "start-time": "1700000000000", "end-time": "1700003600000"}},
		{"between rfc3339", TimeRangeInput{StartTime: "2023-11-14T22:13:20Z", EndTime: "2023-11-14T23:13:20Z"}, TimeRange{Type: RangeBetweenTimes, StartTime: 1700000000000, EndTime: 1700003600000}, map[string]string{"time-range-type": "BETWEEN_TIMES", "start-time": "1700000000000", "end-time": "1700003600000"}},
		{"before time", TimeRangeInput{DurationMins: 30, BeforeTime: "1700003600000"}, TimeRange{Type: RangeBeforeTime, DurationInMins: 30, EndTime: 1700003600000}, map[string]string{"time-range-type": "BEFORE_TIME", "duration-in-mins": "30", "end-time": "1700003600000"}},
		{"after time", TimeRangeInput{DurationMins: 30, AfterTime: "1700000000000"}, TimeRange{Type: RangeAfterTime, DurationInMins: 30, StartTime: 1700000000000}, map[string]string{"time-range-type": "AFTER_TIME", "duration-in-mins": "30", "start-time": "1700000000000"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTimeRange(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("got %+v err=%v want %+v", got, err, tc.want)
			}
			q := got.Query()
			if len(q) != len(tc.query) {
				t.Fatalf("query %v want %v", q, tc.query)
			}
			for k, v := range tc.query {
				if q[k] != v {
					t.Fatalf("query %s=%q want %q", k, q[k], v)
				}
			}
		})
	}
	for _, tc := range []struct {
		name string
		in   TimeRangeInput
		msg  string
	}{
		{"start only", TimeRangeInput{DurationMins: 60, StartTime: "1700000000000"}, "together"},
		{"end only", TimeRangeInput{DurationMins: 60, EndTime: "1700000000000"}, "together"},
		{"both anchors", TimeRangeInput{DurationMins: 60, BeforeTime: "1", AfterTime: "2"}, "only one"},
		{"between plus before", TimeRangeInput{StartTime: "1", EndTime: "2", BeforeTime: "3"}, "cannot be combined"},
		{"end before start", TimeRangeInput{StartTime: "1700003600000", EndTime: "1700000000000"}, "later than"},
		{"zero duration", TimeRangeInput{DurationMins: 0}, "positive"},
		{"negative duration before", TimeRangeInput{DurationMins: -1, BeforeTime: "1700000000000"}, "positive"},
		{"bad start", TimeRangeInput{StartTime: "soon", EndTime: "1700000000000"}, "--start-time"},
		{"bad before", TimeRangeInput{DurationMins: 5, BeforeTime: "soon"}, "--before-time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveTimeRange(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.msg) {
				t.Fatalf("err=%v want substring %q", err, tc.msg)
			}
		})
	}
}
