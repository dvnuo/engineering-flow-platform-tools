package mobile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Observation struct {
	ID              string      `json:"id"`
	RunID           string      `json:"run_id"`
	SessionID       string      `json:"session_id"`
	Context         string      `json:"context,omitempty"`
	SourceHash      string      `json:"source_hash"`
	ScreenshotHash  string      `json:"screenshot_hash,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	SourcePath      string      `json:"source_path,omitempty"`
	ScreenshotPath  string      `json:"screenshot_path,omitempty"`
	CandidatesPath  string      `json:"candidates_path,omitempty"`
	Candidates      []Candidate `json:"candidates"`
	TotalCandidates int         `json:"total_candidates"`
	Source          string      `json:"-"`
	Screenshot      []byte      `json:"-"`
}

type Candidate struct {
	Ref             string        `json:"ref"`
	CandidateID     string        `json:"candidate_id"`
	Role            string        `json:"role,omitempty"`
	Class           string        `json:"class,omitempty"`
	Type            string        `json:"type,omitempty"`
	Name            string        `json:"name,omitempty"`
	Text            string        `json:"text,omitempty"`
	Value           string        `json:"value,omitempty"`
	ResourceID      string        `json:"resource_id,omitempty"`
	AccessibilityID string        `json:"accessibility_id,omitempty"`
	Clickable       bool          `json:"clickable,omitempty"`
	Focusable       bool          `json:"focusable,omitempty"`
	Enabled         bool          `json:"enabled"`
	Visible         bool          `json:"visible"`
	Selected        bool          `json:"selected,omitempty"`
	Checked         bool          `json:"checked,omitempty"`
	Scrollable      bool          `json:"scrollable,omitempty"`
	Password        bool          `json:"password,omitempty"`
	Bounds          Bounds        `json:"bounds,omitempty"`
	ParentHint      string        `json:"parent_hint,omitempty"`
	NearbyText      []string      `json:"nearby_text,omitempty"`
	LocatorHints    []LocatorHint `json:"locator_hints,omitempty"`
}

type Bounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type LocatorHint struct {
	Using      string `json:"using"`
	Value      string `json:"value"`
	Confidence int    `json:"confidence"`
	Reason     string `json:"reason"`
}

type LocateQuery struct {
	Name            string
	Text            string
	Role            string
	ResourceID      string
	AccessibilityID string
	ParentText      string
	NearbyText      string
	Visible         *bool
	Enabled         *bool
	Actionable      bool
	Limit           int
}

type LocateResult struct {
	Matches        []CandidateScore `json:"matches"`
	RecommendedRef string           `json:"recommended_ref,omitempty"`
	Ambiguous      bool             `json:"ambiguous"`
}

type CandidateScore struct {
	Candidate Candidate `json:"candidate"`
	Score     int       `json:"score"`
	Reasons   []string  `json:"reasons"`
}

var boundsPattern = regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)

func BuildObservation(runID, sessionID, obsID, source string, screenshot []byte, limit int) Observation {
	if limit <= 0 {
		limit = 100
	}
	sourceHash := sha256.Sum256([]byte(source))
	screenHash := sha256.Sum256(screenshot)
	candidates := ExtractCandidates(source, obsID)
	total := len(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return Observation{
		ID:              obsID,
		RunID:           runID,
		SessionID:       sessionID,
		SourceHash:      hex.EncodeToString(sourceHash[:]),
		ScreenshotHash:  hex.EncodeToString(screenHash[:]),
		CreatedAt:       time.Now().UTC(),
		Candidates:      candidates,
		TotalCandidates: total,
		Source:          source,
		Screenshot:      screenshot,
	}
}

func ExtractCandidates(source, obsID string) []Candidate {
	dec := xml.NewDecoder(bytes.NewReader([]byte(source)))
	var out []Candidate
	var textContext []string
	seq := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		attrs := attrsMap(start.Attr)
		c := candidateFromAttrs(attrs, start.Name.Local)
		if c.Text != "" || c.Name != "" {
			textContext = append(textContext, firstNonEmpty(c.Text, c.Name))
			if len(textContext) > 8 {
				textContext = textContext[len(textContext)-8:]
			}
		}
		if !informative(c) {
			continue
		}
		seq++
		c.CandidateID = "e" + strconv.Itoa(seq)
		c.Ref = obsID + ":" + c.CandidateID
		c.NearbyText = append([]string{}, textContext...)
		c.ParentHint = parentHint(textContext)
		c.LocatorHints = LocatorHints(c)
		out = append(out, c)
	}
	return out
}

func attrsMap(attrs []xml.Attr) map[string]string {
	out := map[string]string{}
	for _, attr := range attrs {
		out[strings.ToLower(attr.Name.Local)] = strings.TrimSpace(attr.Value)
	}
	return out
}

func candidateFromAttrs(attrs map[string]string, element string) Candidate {
	class := firstNonEmpty(attrs["class"], attrs["type"], element)
	text := firstNonEmpty(attrs["text"], attrs["label"])
	name := firstNonEmpty(attrs["name"], attrs["content-desc"], attrs["label"], attrs["value"], text)
	password := boolAttr(attrs, "password") || strings.Contains(strings.ToLower(class), "secure")
	value := firstNonEmpty(attrs["value"], attrs["text"])
	if password {
		value = ""
		text = ""
	}
	visible := true
	if v, ok := attrs["visible"]; ok {
		visible = parseBool(v)
	}
	if v, ok := attrs["displayed"]; ok {
		visible = parseBool(v)
	}
	enabled := true
	if v, ok := attrs["enabled"]; ok {
		enabled = parseBool(v)
	}
	return Candidate{
		Role:            inferRole(class, attrs),
		Class:           class,
		Type:            attrs["type"],
		Name:            redactCandidate(name, password),
		Text:            redactCandidate(text, password),
		Value:           redactCandidate(value, password),
		ResourceID:      firstNonEmpty(attrs["resource-id"], attrs["resourceid"], attrs["id"]),
		AccessibilityID: firstNonEmpty(attrs["content-desc"], attrs["label"], attrs["name"]),
		Clickable:       boolAttr(attrs, "clickable") || boolAttr(attrs, "hittable"),
		Focusable:       boolAttr(attrs, "focusable") || boolAttr(attrs, "focused"),
		Enabled:         enabled,
		Visible:         visible,
		Selected:        boolAttr(attrs, "selected"),
		Checked:         boolAttr(attrs, "checked"),
		Scrollable:      boolAttr(attrs, "scrollable"),
		Password:        password,
		Bounds:          parseBounds(attrs),
	}
}

func informative(c Candidate) bool {
	if c.Role == "container" && c.Name == "" && c.Text == "" && c.ResourceID == "" && c.AccessibilityID == "" && !c.Clickable && !c.Focusable && !c.Scrollable {
		return false
	}
	return c.Name != "" || c.Text != "" || c.ResourceID != "" || c.AccessibilityID != "" || c.Clickable || c.Focusable || c.Scrollable || c.Role != "container"
}

func inferRole(class string, attrs map[string]string) string {
	lower := strings.ToLower(class)
	switch {
	case strings.Contains(lower, "button"):
		return "button"
	case strings.Contains(lower, "edittext") || strings.Contains(lower, "textfield") || strings.Contains(lower, "textinput"):
		return "textbox"
	case strings.Contains(lower, "checkbox"):
		return "checkbox"
	case strings.Contains(lower, "switch"):
		return "switch"
	case strings.Contains(lower, "radio"):
		return "radio"
	case strings.Contains(lower, "image"):
		return "image"
	case strings.Contains(lower, "textview") || strings.Contains(lower, "statictext"):
		return "text"
	case boolAttr(attrs, "scrollable") || strings.Contains(lower, "scroll"):
		return "scroll"
	case boolAttr(attrs, "clickable") || boolAttr(attrs, "hittable"):
		return "button"
	default:
		return "container"
	}
}

func parseBounds(attrs map[string]string) Bounds {
	if raw := attrs["bounds"]; raw != "" {
		m := boundsPattern.FindStringSubmatch(raw)
		if len(m) == 5 {
			x1, _ := strconv.Atoi(m[1])
			y1, _ := strconv.Atoi(m[2])
			x2, _ := strconv.Atoi(m[3])
			y2, _ := strconv.Atoi(m[4])
			return Bounds{X: x1, Y: y1, Width: x2 - x1, Height: y2 - y1}
		}
	}
	x, _ := strconv.Atoi(firstNonEmpty(attrs["x"], attrs["left"]))
	y, _ := strconv.Atoi(firstNonEmpty(attrs["y"], attrs["top"]))
	w, _ := strconv.Atoi(firstNonEmpty(attrs["width"], attrs["w"]))
	h, _ := strconv.Atoi(firstNonEmpty(attrs["height"], attrs["h"]))
	return Bounds{X: x, Y: y, Width: w, Height: h}
}

func boolAttr(attrs map[string]string, key string) bool {
	return parseBool(attrs[key])
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

func parentHint(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func redactCandidate(value string, password bool) string {
	if password {
		return ""
	}
	lower := strings.ToLower(value)
	for _, needle := range []string{"password=", "token=", "api_key=", "access_key="} {
		if strings.Contains(lower, needle) {
			return "***REDACTED***"
		}
	}
	return value
}

func LocatorHints(c Candidate) []LocatorHint {
	var hints []LocatorHint
	if c.AccessibilityID != "" {
		hints = append(hints, LocatorHint{Using: "accessibility id", Value: c.AccessibilityID, Confidence: 100, Reason: "stable accessibility id/name"})
	}
	if c.ResourceID != "" {
		hints = append(hints, LocatorHint{Using: "id", Value: c.ResourceID, Confidence: 95, Reason: "stable resource id"})
	}
	if c.Text != "" {
		escaped := strings.ReplaceAll(c.Text, `"`, `\"`)
		hints = append(hints, LocatorHint{Using: "-android uiautomator", Value: `new UiSelector().text("` + escaped + `")`, Confidence: 70, Reason: "visible Android text"})
		hints = append(hints, LocatorHint{Using: "-ios predicate string", Value: `name == "` + escaped + `" OR label == "` + escaped + `"`, Confidence: 70, Reason: "visible iOS name/label"})
	}
	if c.Class != "" && (c.Text != "" || c.Name != "") {
		name := firstNonEmpty(c.Text, c.Name)
		hints = append(hints, LocatorHint{Using: "xpath", Value: `//*[@text="` + name + `" or @name="` + name + `" or @label="` + name + `"]`, Confidence: 30, Reason: "last-resort source-tree fallback"})
	}
	return hints
}

func Locate(obs Observation, q LocateQuery) LocateResult {
	var scores []CandidateScore
	for _, c := range obs.Candidates {
		score, reasons := scoreCandidate(c, q)
		if score <= 0 {
			continue
		}
		scores = append(scores, CandidateScore{Candidate: c, Score: score, Reasons: reasons})
	}
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].Score != scores[j].Score {
			return scores[i].Score > scores[j].Score
		}
		return scores[i].Candidate.Ref < scores[j].Candidate.Ref
	})
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(scores) > limit {
		scores = scores[:limit]
	}
	res := LocateResult{Matches: scores}
	if len(scores) == 1 && scores[0].Score >= 80 {
		res.RecommendedRef = scores[0].Candidate.Ref
		return res
	}
	if len(scores) > 1 {
		res.Ambiguous = scores[0].Score-scores[1].Score < 20
		if !res.Ambiguous && scores[0].Score >= 90 {
			res.RecommendedRef = scores[0].Candidate.Ref
		}
	}
	return res
}

func scoreCandidate(c Candidate, q LocateQuery) (int, []string) {
	score := 0
	var reasons []string
	if q.AccessibilityID != "" {
		if equalFold(c.AccessibilityID, q.AccessibilityID) {
			score += 100
			reasons = append(reasons, "exact accessibility id")
		} else {
			return 0, nil
		}
	}
	if q.ResourceID != "" {
		if equalFold(c.ResourceID, q.ResourceID) {
			score += 95
			reasons = append(reasons, "exact resource id")
		} else {
			return 0, nil
		}
	}
	if q.Role != "" {
		if equalFold(c.Role, q.Role) {
			score += 35
			reasons = append(reasons, "role match")
		} else {
			return 0, nil
		}
	}
	if q.Name != "" {
		if equalFold(c.Name, q.Name) || equalFold(c.AccessibilityID, q.Name) {
			score += 55
			reasons = append(reasons, "exact name match")
		} else if containsFold(c.Name, q.Name) || containsFold(c.Text, q.Name) {
			score += 20
			reasons = append(reasons, "fuzzy name match")
		} else {
			return 0, nil
		}
	}
	if q.Text != "" {
		if equalFold(c.Text, q.Text) {
			score += 45
			reasons = append(reasons, "exact text match")
		} else if containsFold(c.Text, q.Text) || containsFold(c.Name, q.Text) {
			score += 18
			reasons = append(reasons, "fuzzy text match")
		} else {
			return 0, nil
		}
	}
	if q.ParentText != "" {
		if containsFold(c.ParentHint, q.ParentText) {
			score += 15
			reasons = append(reasons, "parent context")
		} else {
			return 0, nil
		}
	}
	if q.NearbyText != "" {
		if nearbyContains(c.NearbyText, q.NearbyText) {
			score += 15
			reasons = append(reasons, "nearby text")
		} else {
			return 0, nil
		}
	}
	if q.Visible != nil {
		if c.Visible == *q.Visible {
			score += 10
			reasons = append(reasons, "visible state")
		} else {
			return 0, nil
		}
	}
	if q.Enabled != nil {
		if c.Enabled == *q.Enabled {
			score += 10
			reasons = append(reasons, "enabled state")
		} else {
			return 0, nil
		}
	}
	if q.Actionable {
		if c.Clickable || c.Focusable || c.Role == "button" || c.Role == "textbox" {
			score += 10
			reasons = append(reasons, "actionable")
		} else {
			return 0, nil
		}
	}
	if score == 0 && (q.Name != "" || q.Text != "" || q.Role != "" || q.ResourceID != "" || q.AccessibilityID != "") {
		return 0, nil
	}
	return score, reasons
}

func CandidateByRef(obs Observation, ref string) (Candidate, bool) {
	for _, c := range obs.Candidates {
		if c.Ref == ref {
			return c, true
		}
	}
	return Candidate{}, false
}

func RefObservationID(ref string) string {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

func equalFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func containsFold(a, b string) bool {
	return strings.Contains(strings.ToLower(a), strings.ToLower(strings.TrimSpace(b)))
}

func nearbyContains(values []string, needle string) bool {
	for _, value := range values {
		if containsFold(value, needle) {
			return true
		}
	}
	return false
}

func JSONHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
