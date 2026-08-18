package daemon

import (
	"context"
	"net/http"
	"testing"

	"github.com/decionis/docker/internal/updatecheck"
)

func withFakeUpdateCheck(t *testing.T, result updatecheck.Result) *int {
	t.Helper()
	calls := 0
	original := checkUpdate
	checkUpdate = func(_ context.Context, _ string, currentVersion string) updatecheck.Result {
		calls++
		out := result
		out.CurrentVersion = currentVersion
		return out
	}
	t.Cleanup(func() { checkUpdate = original })
	return &calls
}

func TestUpdateEndpointAnnouncesNewerRelease(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	calls := withFakeUpdateCheck(t, updatecheck.Result{
		Checked:         true,
		LatestVersion:   "9.9.9",
		UpdateAvailable: true,
	})

	response, body := rig.do(t, http.MethodGet, "/api/update", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update endpoint failed: %d", response.StatusCode)
	}
	if body["update_available"] != true || body["latest_version"] != "9.9.9" {
		t.Fatalf("unexpected update payload: %v", body)
	}
	if body["current_version"] != "test" {
		t.Fatalf("current version must be the daemon's own: %v", body)
	}

	// Second read is served from cache — no second upstream call.
	rig.do(t, http.MethodGet, "/api/update", "")
	if *calls != 1 {
		t.Fatalf("expected 1 upstream check for two reads, got %d", *calls)
	}
}

func TestUpdateEndpointClaimsNothingWhenUnchecked(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	withFakeUpdateCheck(t, updatecheck.Result{Checked: false})

	response, body := rig.do(t, http.MethodGet, "/api/update", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update endpoint failed: %d", response.StatusCode)
	}
	if body["checked"] != false || body["update_available"] != false {
		t.Fatalf("an unchecked result must claim nothing: %v", body)
	}
}
