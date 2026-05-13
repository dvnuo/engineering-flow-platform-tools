package config

func RedactAuth(a Auth) Auth {
	a.Password = redact(a.Password)
	a.APIKey = redact(a.APIKey)
	a.Token = redact(a.Token)
	return a
}

func redact(v string) string {
	if v == "" {
		return ""
	}
	return "***REDACTED***"
}
