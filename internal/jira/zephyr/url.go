package zephyr

import (
	"net/url"
	"strings"
)

type ResolvedURL struct {
	Product    string `json:"product"`
	Plugin     string `json:"plugin"`
	ProjectKey string `json:"project_key,omitempty"`
	PluginKey  string `json:"plugin_key"`
	Page       string `json:"page"`
	Tab        string `json:"tab,omitempty"`
}

func ResolveURL(raw string) (ResolvedURL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ResolvedURL{}, NewError("zephyr_url_unrecognized", "Jira Zephyr URL could not be parsed", "Use a Jira project URL with selectedItem=com.thed.zephyr.je:...", 400)
	}
	selected := u.Query().Get("selectedItem")
	parts := strings.SplitN(selected, ":", 2)
	if len(parts) != 2 || parts[0] != PluginKey || strings.TrimSpace(parts[1]) == "" {
		return ResolvedURL{}, NewError("zephyr_url_unrecognized", "URL is not a recognized Zephyr project page", "Look for selectedItem=com.thed.zephyr.je:<page> in the Jira URL.", 400)
	}
	return ResolvedURL{
		Product:    "jira",
		Plugin:     "zephyr",
		ProjectKey: projectKeyFromPath(u.Path),
		PluginKey:  parts[0],
		Page:       parts[1],
		Tab:        strings.TrimPrefix(u.Fragment, "#"),
	}, nil
}

func projectKeyFromPath(p string) string {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "projects" && parts[i+1] != "" {
			return parts[i+1]
		}
	}
	return ""
}
