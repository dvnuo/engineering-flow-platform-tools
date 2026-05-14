package confluence

import "net/url"

func IsAbsolute(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.IsAbs()
}

