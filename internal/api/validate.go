package api

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateBaseURL is the single gate every user-supplied control-plane base
// URL passes before it can appear in an outbound request (daemon connect,
// enrollment exchange, client construction). It enforces the transport
// policy of rules/security.rules.md Rule 2.6 — https everywhere, plain http
// only for loopback development planes — and returns a normalized origin
// (plus optional path prefix) rebuilt from the parsed components: scheme,
// validated host, and path only. Credentials, query, and fragment can never
// survive into the value requests are built from.
func ValidateBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		trimmed = DefaultBaseURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errors.New("base URL is not a valid URL")
	}
	host := parsed.Hostname()
	if host == "" {
		return "", errors.New("base URL must include a host")
	}
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	switch parsed.Scheme {
	case "https":
		// Always allowed.
	case "http":
		if !loopback {
			return "", errors.New("base URL must use https")
		}
	default:
		return "", errors.New("base URL must use https")
	}
	if parsed.User != nil {
		return "", errors.New("base URL must be a plain origin without credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL must not carry a query or fragment")
	}

	rebuilt := url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: strings.TrimRight(parsed.Path, "/")}
	return rebuilt.String(), nil
}
