package appd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Time range types of the Controller REST API.
const (
	RangeBeforeNow    = "BEFORE_NOW"
	RangeBeforeTime   = "BEFORE_TIME"
	RangeAfterTime    = "AFTER_TIME"
	RangeBetweenTimes = "BETWEEN_TIMES"
)

// TimeRangeInput carries the raw time flags shared by every time-ranged
// command. Timestamps are epoch milliseconds or RFC3339 strings.
type TimeRangeInput struct {
	DurationMins int
	StartTime    string
	EndTime      string
	BeforeTime   string
	AfterTime    string
}

// TimeRange is the resolved, explicit window sent to the Controller and
// echoed back in command output so agents always see what was queried.
type TimeRange struct {
	Type           string `json:"type"`
	DurationInMins int    `json:"duration_in_mins,omitempty"`
	StartTime      int64  `json:"start_time,omitempty"`
	EndTime        int64  `json:"end_time,omitempty"`
}

// Query renders the range as Controller query parameters.
func (t TimeRange) Query() map[string]string {
	q := map[string]string{"time-range-type": t.Type}
	if t.Type != RangeBetweenTimes && t.DurationInMins > 0 {
		q["duration-in-mins"] = strconv.Itoa(t.DurationInMins)
	}
	if t.StartTime > 0 {
		q["start-time"] = strconv.FormatInt(t.StartTime, 10)
	}
	if t.EndTime > 0 {
		q["end-time"] = strconv.FormatInt(t.EndTime, 10)
	}
	return q
}

// ResolveTimeRange maps the shared time flags onto exactly one Controller
// time-range-type:
//   - --start-time plus --end-time            -> BETWEEN_TIMES
//   - --before-time plus --duration-mins      -> BEFORE_TIME
//   - --after-time plus --duration-mins       -> AFTER_TIME
//   - --duration-mins alone                   -> BEFORE_NOW
func ResolveTimeRange(in TimeRangeInput) (TimeRange, error) {
	hasStart := strings.TrimSpace(in.StartTime) != ""
	hasEnd := strings.TrimSpace(in.EndTime) != ""
	hasBefore := strings.TrimSpace(in.BeforeTime) != ""
	hasAfter := strings.TrimSpace(in.AfterTime) != ""
	if (hasStart || hasEnd) && (hasBefore || hasAfter) {
		return TimeRange{}, errors.New("--start-time/--end-time cannot be combined with --before-time or --after-time")
	}
	if hasStart != hasEnd {
		return TimeRange{}, errors.New("--start-time and --end-time must be given together")
	}
	if hasBefore && hasAfter {
		return TimeRange{}, errors.New("use only one of --before-time and --after-time")
	}
	if hasStart {
		start, err := ParseTimestamp(in.StartTime)
		if err != nil {
			return TimeRange{}, fmt.Errorf("--start-time: %w", err)
		}
		end, err := ParseTimestamp(in.EndTime)
		if err != nil {
			return TimeRange{}, fmt.Errorf("--end-time: %w", err)
		}
		if end <= start {
			return TimeRange{}, errors.New("--end-time must be later than --start-time")
		}
		return TimeRange{Type: RangeBetweenTimes, StartTime: start, EndTime: end}, nil
	}
	if in.DurationMins <= 0 {
		return TimeRange{}, errors.New("--duration-mins must be a positive number of minutes")
	}
	if hasBefore {
		end, err := ParseTimestamp(in.BeforeTime)
		if err != nil {
			return TimeRange{}, fmt.Errorf("--before-time: %w", err)
		}
		return TimeRange{Type: RangeBeforeTime, DurationInMins: in.DurationMins, EndTime: end}, nil
	}
	if hasAfter {
		start, err := ParseTimestamp(in.AfterTime)
		if err != nil {
			return TimeRange{}, fmt.Errorf("--after-time: %w", err)
		}
		return TimeRange{Type: RangeAfterTime, DurationInMins: in.DurationMins, StartTime: start}, nil
	}
	return TimeRange{Type: RangeBeforeNow, DurationInMins: in.DurationMins}, nil
}

// ParseTimestamp accepts epoch milliseconds (an integer; ten digits or fewer
// are treated as epoch seconds and scaled) or an RFC3339 / RFC3339Nano
// string, and returns epoch milliseconds.
func ParseTimestamp(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, errors.New("timestamp is empty")
	}
	if isDigits(s) {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp %q: %v", raw, err)
		}
		if len(s) <= 10 {
			n *= 1000
		}
		return n, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UnixMilli(), nil
		}
	}
	return 0, fmt.Errorf("invalid timestamp %q: use epoch milliseconds or RFC3339 such as 2026-09-19T08:00:00Z", raw)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
