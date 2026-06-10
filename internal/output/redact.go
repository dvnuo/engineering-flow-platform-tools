package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

const Redacted = "***REDACTED***"

var (
	embeddedURLPattern       = regexp.MustCompile(`https?://[^\s"'<>\\]+`)
	authHeaderPattern        = regexp.MustCompile(`(?i)\b(authorization|proxy-authorization)\s*[:=]\s*(?:bearer|basic)?\s*[A-Za-z0-9._~+/=-]+`)
	cookieHeaderPattern      = regexp.MustCompile(`(?i)\b(set-cookie|cookie)\s*[:=]\s*[^,\r\n]+`)
	authCredentialPattern    = regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+[A-Za-z0-9._~+/=-]+`)
	githubTokenPattern       = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+)\b`)
	openAIKeyPattern         = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)
	slackTokenPattern        = regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)
	jwtPattern               = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	dataImagePattern         = regexp.MustCompile(`(?i)(data:image/[a-z0-9.+-]+;base64,)[A-Za-z0-9+/=_-]+`)
	tidPattern               = regexp.MustCompile(`(?i)\btid=[^;&,\s"']+`)
	sensitiveFieldPattern    = regexp.MustCompile(`(?i)(["']?[A-Za-z0-9_.~-]*(?:access[_-]?token|id[_-]?token|refresh[_-]?token|token|api[_-]?key|apikey|password|passwd|pwd|authorization|cookie|secret|credential|session)[A-Za-z0-9_.~-]*["']?\s*[:=]\s*)(["'][^"']*["']|[^,}\]\s;&]+)`)
	sensitiveAssignPattern   = regexp.MustCompile(`(?i)([A-Za-z0-9_.~-]*(?:access[_-]?token|id[_-]?token|refresh[_-]?token|token|api[_-]?key|apikey|password|passwd|pwd|jwt|saml|secret|credential|cookie|authorization|session|sig|signature)[A-Za-z0-9_.~-]*\s*=\s*)[^&#\s,;"'}]+`)
	labeledSecretWordPattern = regexp.MustCompile(`(?i)\b(?:secret|token|password|api[-_]?key)[-_][A-Za-z0-9._~+/\-=]{8,}\b`)
)

func RedactEnvelope(env Envelope) Envelope {
	out := Envelope{
		OK:       env.OK,
		Instance: RedactString(env.Instance),
		Data:     RedactValue(env.Data),
	}
	if env.Error != nil {
		err := *env.Error
		err.Code = RedactString(err.Code)
		err.Message = RedactString(err.Message)
		err.Hint = RedactString(err.Hint)
		err.TemplateID = RedactString(err.TemplateID)
		err.File = RedactString(err.File)
		err.MissingFiles = redactStringSlice(err.MissingFiles)
		err.OrphanTemplateDirs = redactStringSlice(err.OrphanTemplateDirs)
		out.Error = &err
	}
	return out
}

func RedactValue(value any) any {
	if value == nil {
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return RedactString(fmt.Sprint(value))
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return RedactString(fmt.Sprint(value))
	}
	return redactDecoded(decoded)
}

func RedactString(s string) string {
	if s == "" {
		return ""
	}
	out := embeddedURLPattern.ReplaceAllStringFunc(s, redactURL)
	out = authHeaderPattern.ReplaceAllString(out, `${1}: `+Redacted)
	out = cookieHeaderPattern.ReplaceAllString(out, `${1}: `+Redacted)
	out = authCredentialPattern.ReplaceAllString(out, Redacted)
	out = githubTokenPattern.ReplaceAllString(out, Redacted)
	out = openAIKeyPattern.ReplaceAllString(out, Redacted)
	out = slackTokenPattern.ReplaceAllString(out, Redacted)
	out = jwtPattern.ReplaceAllString(out, Redacted)
	out = dataImagePattern.ReplaceAllString(out, `${1}`+Redacted)
	out = tidPattern.ReplaceAllString(out, "tid="+Redacted)
	out = replaceSensitiveFields(out)
	out = replaceSensitiveAssignments(out)
	out = labeledSecretWordPattern.ReplaceAllString(out, Redacted)
	return out
}

func replaceSensitiveFields(s string) string {
	return sensitiveFieldPattern.ReplaceAllStringFunc(s, func(match string) string {
		if strings.Contains(match, "REDACTED") {
			return match
		}
		parts := sensitiveFieldPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return Redacted
		}
		return parts[1] + `"` + Redacted + `"`
	})
}

func replaceSensitiveAssignments(s string) string {
	return sensitiveAssignPattern.ReplaceAllStringFunc(s, func(match string) string {
		if strings.Contains(match, "REDACTED") {
			return match
		}
		parts := sensitiveAssignPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return Redacted
		}
		return parts[1] + Redacted
	})
}

func redactDecoded(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			cleanKey := RedactString(key)
			if isSensitiveOutputKey(key) {
				out[cleanKey] = Redacted
				continue
			}
			out[cleanKey] = redactDecoded(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = redactDecoded(item)
		}
		return out
	case string:
		return RedactString(v)
	default:
		return v
	}
}

func redactStringSlice(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = RedactString(value)
	}
	return out
}

func isSensitiveOutputKey(key string) bool {
	norm := normalizeKey(key)
	switch norm {
	case "password", "passwd", "pwd", "apikey", "apitoken", "token", "accesstoken", "refreshtoken", "idtoken",
		"authorization", "proxyauthorization", "cookie", "cookies", "setcookie", "secret", "clientsecret",
		"privatekey", "credential", "credentials", "sessiontoken", "csrftoken", "xcsrftoken", "xsrftoken",
		"awssecretaccesskey":
		return true
	}
	if strings.HasSuffix(norm, "token") ||
		strings.HasSuffix(norm, "password") ||
		strings.HasSuffix(norm, "apikey") ||
		strings.HasSuffix(norm, "secret") ||
		strings.HasSuffix(norm, "privatekey") {
		return true
	}
	return false
}

func isSensitiveURLKey(key string) bool {
	norm := normalizeKey(key)
	switch norm {
	case "code", "sig", "signature", "ticket", "state", "saml", "jwt":
		return true
	}
	return isSensitiveOutputKey(key) ||
		strings.Contains(norm, "token") ||
		strings.Contains(norm, "session") ||
		strings.Contains(norm, "cookie") ||
		strings.Contains(norm, "credential")
}

func normalizeKey(key string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(key)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func redactURL(raw string) string {
	urlPart, suffix := splitURLSuffix(raw)
	u, err := url.Parse(urlPart)
	if err != nil || u == nil {
		return redactStringNoURL(raw)
	}
	if u.User != nil {
		u.User = url.User("REDACTED")
	}
	q := u.Query()
	for key := range q {
		if isSensitiveURLKey(key) {
			q.Set(key, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	if u.Fragment != "" {
		u.Fragment = "REDACTED"
	}
	return u.String() + suffix
}

func redactStringNoURL(s string) string {
	out := sensitiveAssignPattern.ReplaceAllString(s, `${1}`+Redacted)
	out = tidPattern.ReplaceAllString(out, "tid="+Redacted)
	return out
}

func splitURLSuffix(raw string) (string, string) {
	suffix := ""
	for len(raw) > 0 {
		last := raw[len(raw)-1]
		if !strings.ContainsRune(".,;)]}", rune(last)) {
			break
		}
		suffix = string(last) + suffix
		raw = raw[:len(raw)-1]
	}
	return raw, suffix
}
