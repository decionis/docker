// Package updatecheck discovers whether a newer extension image exists by
// reading Docker Hub's public tags listing — an anonymous, credential-free
// GET against a public endpoint (documented in SECURITY.md's outbound
// network posture). It never phones home anywhere else and carries no
// identifiers beyond the request itself.
//
// The contract is fail-open but honest: when the listing is unreachable the
// result says Checked=false and claims nothing — an update is only ever
// announced when both versions parse and the latest is strictly newer
// (rules/discovery.rules.md Rule 1.2: no claims beyond verified state).
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

// Result is what the daemon serves to the UI.
type Result struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	// Checked is false when the tags listing could not be read; the other
	// fields then claim nothing.
	Checked bool `json:"checked"`
}

// hubBaseURL is swapped in tests.
var hubBaseURL = "https://hub.docker.com"

type semver [3]int

func parseSemver(value string) (semver, bool) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var parsed semver
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return semver{}, false
		}
		parsed[i] = number
	}
	return parsed, true
}

func newer(candidate, current semver) bool {
	for i := range candidate {
		if candidate[i] != current[i] {
			return candidate[i] > current[i]
		}
	}
	return false
}

// latestTag reads the repository's public tags listing and returns the
// highest semver tag (non-semver tags like "latest" are ignored).
func latestTag(ctx context.Context, repository string) (string, error) {
	requestURL := hubBaseURL + "/v2/repositories/" + url.PathEscape(repository) + "/tags?page_size=100"
	// url.PathEscape escapes the namespace separator too; restore it.
	requestURL = strings.ReplaceAll(requestURL, "%2F", "/")

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("updatecheck: build request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("updatecheck: tags listing unreachable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("updatecheck: tags listing returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("updatecheck: tags listing read failed")
	}
	var listing struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return "", fmt.Errorf("updatecheck: malformed tags listing")
	}

	best := ""
	var bestParsed semver
	for _, tag := range listing.Results {
		parsed, ok := parseSemver(tag.Name)
		if !ok {
			continue
		}
		if best == "" || newer(parsed, bestParsed) {
			best = tag.Name
			bestParsed = parsed
		}
	}
	if best == "" {
		return "", fmt.Errorf("updatecheck: no semver tags in listing")
	}
	return best, nil
}

// Check reports whether repository has a release newer than currentVersion.
// It never returns an error: unreachable or unparsable state yields
// Checked=false (or UpdateAvailable=false for non-semver dev builds).
func Check(ctx context.Context, repository, currentVersion string) Result {
	result := Result{CurrentVersion: currentVersion}

	latest, err := latestTag(ctx, repository)
	if err != nil {
		return result // Checked stays false; nothing is claimed.
	}
	result.Checked = true
	result.LatestVersion = latest

	current, currentOK := parseSemver(currentVersion)
	latestParsed, latestOK := parseSemver(latest)
	if currentOK && latestOK && newer(latestParsed, current) {
		result.UpdateAvailable = true
	}
	return result
}
