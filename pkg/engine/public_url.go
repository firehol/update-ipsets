package engine

import (
	"net/url"
	"strings"
)

func (e *Engine) publicSiteBaseURL() string {
	return publicSiteBaseURLForRuntime(e.Runtime())
}

func publicSiteBaseURLForRuntime(rt Runtime) string {
	if base := normalizeAbsolutePublicURL(rt.PublicBaseURL); base != "" {
		return base
	}
	return derivePublicSiteBaseFromWebURL(rt.WebURL)
}

func (e *Engine) publicFeedURLPrefix(siteBase string) string {
	return publicFeedURLPrefixForRuntime(e.Runtime(), siteBase)
}

func publicFeedURLPrefixForRuntime(rt Runtime, siteBase string) string {
	if prefix := normalizeAbsolutePublicURL(rt.WebURL); prefix != "" {
		return prefix
	}
	if siteBase == "" {
		return ""
	}
	return joinPublicURL(siteBase, "ipsets")
}

func normalizeAbsolutePublicURL(raw string) string {
	parsed, ok := parseAbsolutePublicURL(raw)
	if !ok {
		return ""
	}
	return strings.TrimRight(publicURLWithPath(parsed, strings.TrimRight(parsed.Path, "/")), "/")
}

func derivePublicSiteBaseFromWebURL(raw string) string {
	parsed, ok := parseAbsolutePublicURL(raw)
	if !ok {
		return ""
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "" || path == "/":
		path = ""
	case strings.HasSuffix(path, "/ipsets"):
		path = strings.TrimSuffix(path, "/ipsets")
		if path == "/" {
			path = ""
		}
	}
	return strings.TrimRight(publicURLWithPath(parsed, path), "/")
}

func publicURLWithPath(parsed *url.URL, path string) string {
	return (&url.URL{
		Scheme:      parsed.Scheme,
		Opaque:      parsed.Opaque,
		User:        parsed.User,
		Host:        parsed.Host,
		Path:        path,
		Fragment:    parsed.Fragment,
		RawQuery:    parsed.RawQuery,
		RawFragment: parsed.RawFragment,
		ForceQuery:  parsed.ForceQuery,
		OmitHost:    parsed.OmitHost,
	}).String()
}

func parseAbsolutePublicURL(raw string) (*url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, false
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, true
}

func joinPublicURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		if base == "" {
			return "/"
		}
		return base
	}
	if base == "" {
		return "/" + path
	}
	return base + "/" + path
}
