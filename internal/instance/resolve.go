package instance

import (
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/config"
)

func Resolve(product config.ProductConfig, explicit, rawURL, productName string) (Result, error) {
	if len(product.Instances) == 0 {
		return Result{}, errors.New("no_instance_configured")
	}
	if explicit != "" {
		for _, in := range product.Instances {
			if in.Name == explicit {
				return Result{Instance: in, Entity: parseEntity(rawURL, productName)}, nil
			}
		}
		return Result{}, errors.New("instance_required")
	}
	if rawURL != "" && strings.HasPrefix(rawURL, "http") {
		matches := matchByURL(product.Instances, rawURL)
		if len(matches) > 1 {
			cand := make([]string, 0, len(matches))
			for _, m := range matches {
				cand = append(cand, m.Name)
			}
			sort.Strings(cand)
			return Result{Candidates: cand}, errors.New("ambiguous_instance")
		}
		if len(matches) == 1 {
			return Result{Instance: matches[0], Entity: parseEntity(rawURL, productName)}, nil
		}
	}
	if product.DefaultInstance != "" {
		for _, in := range product.Instances {
			if in.Name == product.DefaultInstance {
				return Result{Instance: in, Entity: parseEntity(rawURL, productName)}, nil
			}
		}
	}
	if len(product.Instances) > 1 {
		return Result{}, errors.New("instance_required")
	}
	return Result{Instance: product.Instances[0], Entity: parseEntity(rawURL, productName)}, nil
}

func matchByURL(instances []config.InstanceConfig, raw string) []config.InstanceConfig {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	n := normalize(u.Scheme + "://" + u.Host + u.Path)
	long := 0
	var out []config.InstanceConfig
	for _, in := range instances {
		b := normalize(in.BaseURL)
		if strings.HasPrefix(n, b) {
			if len(b) > long {
				long = len(b)
				out = []config.InstanceConfig{in}
			} else if len(b) == long {
				out = append(out, in)
			}
		}
	}
	return out
}

func normalize(s string) string { return strings.TrimRight(strings.ToLower(s), "/") }

func parseEntity(raw, product string) ResolvedEntity {
	e := ResolvedEntity{Type: "unknown", Attrs: map[string]string{}}
	u, err := url.Parse(raw)
	if err != nil {
		return e
	}
	p := u.Path
	if product == "jira" {
		re1 := regexp.MustCompile(`/browse/([A-Z][A-Z0-9]+-\d+)`)
		re2 := regexp.MustCompile(`/rest/api/\d+/issue/([A-Z][A-Z0-9]+-\d+)`)
		if m := re1.FindStringSubmatch(p); len(m) == 2 {
			e.Type = "issue"
			e.Attrs["key"] = m[1]
		}
		if m := re2.FindStringSubmatch(p); len(m) == 2 {
			e.Type = "issue"
			e.Attrs["key"] = m[1]
		}
	}
	if product == "confluence" {
		if id := u.Query().Get("pageId"); id != "" && strings.Contains(p, "/pages/viewpage.action") {
			e.Type = "page"
			e.Attrs["id"] = id
		}
		re2 := regexp.MustCompile(`/spaces/([^/]+)/pages/(\d+)/`)
		if m := re2.FindStringSubmatch(p); len(m) == 3 {
			e.Type = "page"
			e.Attrs["space"] = m[1]
			e.Attrs["id"] = m[2]
		}
		re3 := regexp.MustCompile(`/display/([^/]+)/(.+)`)
		if m := re3.FindStringSubmatch(p); len(m) == 3 {
			e.Type = "page"
			e.Attrs["space"] = m[1]
			e.Attrs["title"] = m[2]
		}
	}
	return e
}
