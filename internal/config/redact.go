package config

func RedactRoot(c RootConfig) RootConfig {
	for i := range c.Jira.Instances {
		c.Jira.Instances[i].Auth = RedactAuth(c.Jira.Instances[i].Auth)
	}
	for i := range c.Confluence.Instances {
		c.Confluence.Instances[i].Auth = RedactAuth(c.Confluence.Instances[i].Auth)
	}
	for i := range c.Jenkins.Instances {
		c.Jenkins.Instances[i].Auth = RedactAuth(c.Jenkins.Instances[i].Auth)
	}
	c.AWS = RedactAWS(c.AWS)
	return c
}

func RedactAWS(a AWSConfig) AWSConfig {
	a.Password = redact(a.Password)
	return a
}

func RedactAuth(a AuthConfig) AuthConfig {
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
