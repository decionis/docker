package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSemverAndNewer(t *testing.T) {
	cases := []struct {
		current, candidate string
		isNewer            bool
	}{
		{"0.1.2", "0.1.3", true},
		{"0.1.2", "0.2.0", true},
		{"0.1.2", "1.0.0", true},
		{"0.1.2", "0.1.2", false},
		{"0.1.3", "0.1.2", false},
		{"1.0.0", "0.9.9", false},
	}
	for _, c := range cases {
		current, ok1 := parseSemver(c.current)
		candidate, ok2 := parseSemver(c.candidate)
		if !ok1 || !ok2 {
			t.Fatalf("parse failed for %q/%q", c.current, c.candidate)
		}
		if got := newer(candidate, current); got != c.isNewer {
			t.Errorf("newer(%s over %s) = %v, want %v", c.candidate, c.current, got, c.isNewer)
		}
	}
	for _, invalid := range []string{"", "latest", "0.1", "0.1.2.3", "0.1.x", "0.0.0-dev"} {
		if _, ok := parseSemver(invalid); ok {
			t.Errorf("parseSemver(%q) must fail", invalid)
		}
	}
}

func withFakeHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	original := hubBaseURL
	hubBaseURL = server.URL
	t.Cleanup(func() {
		hubBaseURL = original
		server.Close()
	})
}

func tagsResponse(names ...string) string {
	body := `{"results":[`
	for i, name := range names {
		if i > 0 {
			body += ","
		}
		body += `{"name":"` + name + `"}`
	}
	return body + `]}`
}

func TestCheckAnnouncesNewerRelease(t *testing.T) {
	withFakeHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/repositories/decionis/desktop-extension/tags" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(tagsResponse("latest", "0.1.0", "0.1.3", "0.1.2")))
	})

	result := Check(context.Background(), "decionis/desktop-extension", "0.1.2")
	if !result.Checked || !result.UpdateAvailable || result.LatestVersion != "0.1.3" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckIsQuietWhenCurrent(t *testing.T) {
	withFakeHub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(tagsResponse("latest", "0.1.3")))
	})

	result := Check(context.Background(), "decionis/desktop-extension", "0.1.3")
	if !result.Checked || result.UpdateAvailable {
		t.Fatalf("no update must be announced at the latest version: %+v", result)
	}
}

func TestCheckNeverAnnouncesForDevBuilds(t *testing.T) {
	withFakeHub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(tagsResponse("0.1.3")))
	})

	result := Check(context.Background(), "decionis/desktop-extension", "0.0.0-dev")
	if !result.Checked || result.UpdateAvailable {
		t.Fatalf("dev builds must not claim an update: %+v", result)
	}
	if result.LatestVersion != "0.1.3" {
		t.Fatalf("latest version should still be reported: %+v", result)
	}
}

func TestCheckFailsOpenButHonest(t *testing.T) {
	withFakeHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})

	result := Check(context.Background(), "decionis/desktop-extension", "0.1.2")
	if result.Checked || result.UpdateAvailable || result.LatestVersion != "" {
		t.Fatalf("an unreachable listing must claim nothing: %+v", result)
	}
}
