package daemon

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/decionis/docker/internal/api"
)

const enrollmentSentinel = "dcn_enroll_SENTINEL-NEVER-LOGGED-0123456789"
const mintedKeySentinel = "dcn_live_MINTED-SENTINEL-NEVER-LOGGED"

func withFakeExchange(t *testing.T, exchange *api.EnrollmentExchange, exchangeErr error) {
	t.Helper()
	original := exchangeEnrollment
	exchangeEnrollment = func(_ context.Context, _ string, token string) (*api.EnrollmentExchange, error) {
		if token != enrollmentSentinel {
			return nil, api.ErrEnrollmentInvalid
		}
		if exchangeErr != nil {
			return nil, exchangeErr
		}
		return exchange, nil
	}
	t.Cleanup(func() { exchangeEnrollment = original })
}

func enrollBody() string {
	return `{"enrollment_token":"` + enrollmentSentinel + `"}`
}

func TestConnectViaEnrollmentStoresMintedCredentials(t *testing.T) {
	fake := &fakeHosted{listReports: emptyReports}
	rig := newRig(t, fake)
	withFakeExchange(t, &api.EnrollmentExchange{
		OrgID:         "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e",
		RawKey:        mintedKeySentinel,
		ConnectorSlug: "docker-desktop",
		Environment:   "production",
	}, nil)

	response, body := rig.do(t, http.MethodPut, "/api/connection", enrollBody())
	if response.StatusCode != http.StatusOK {
		t.Fatalf("enrollment connect failed: %d (%v)", response.StatusCode, body)
	}
	if body["connected"] != true {
		t.Fatalf("expected connected status, got %v", body)
	}
	connection, apiKey, ok, err := rig.store.Load()
	if err != nil || !ok {
		t.Fatalf("minted credentials must be stored: ok=%v err=%v", ok, err)
	}
	if connection.OrgID != "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e" || apiKey != mintedKeySentinel {
		t.Fatalf("stored credentials mismatch: %+v", connection)
	}
	if strings.Contains(rig.logs.String(), enrollmentSentinel) || strings.Contains(rig.logs.String(), mintedKeySentinel) {
		t.Fatal("enrollment token or minted key leaked into logs")
	}
}

func TestConnectViaEnrollmentInvalidTokenStoresNothing(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	withFakeExchange(t, nil, api.ErrEnrollmentInvalid)

	response, body := rig.do(t, http.MethodPut, "/api/connection", enrollBody())
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%v)", response.StatusCode, body)
	}
	if _, _, ok, _ := rig.store.Load(); ok {
		t.Fatal("nothing must be stored on a rejected exchange")
	}
}

func TestConnectViaEnrollmentUsedToken(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	withFakeExchange(t, nil, api.ErrEnrollmentUsed)

	response, _ := rig.do(t, http.MethodPut, "/api/connection", enrollBody())
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for a consumed token, got %d", response.StatusCode)
	}
}

func TestConnectViaEnrollmentKeepsCredentialsWhenProbeFails(t *testing.T) {
	// The exchange consumed the single-use token and minted real credentials;
	// a transient probe failure must not discard them.
	rig := newRig(t, &fakeHosted{
		listReports: func(context.Context, string, int) (*api.ReportsResponse, error) {
			return nil, errors.New("transient upstream blip")
		},
	})
	withFakeExchange(t, &api.EnrollmentExchange{
		OrgID:  "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e",
		RawKey: mintedKeySentinel,
	}, nil)

	response, body := rig.do(t, http.MethodPut, "/api/connection", enrollBody())
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with stored creds despite probe failure, got %d", response.StatusCode)
	}
	if body["connected"] != true || body["last_error"] == nil {
		t.Fatalf("expected connected with last_error set, got %v", body)
	}
	if _, _, ok, _ := rig.store.Load(); !ok {
		t.Fatal("minted credentials must survive a failed probe")
	}
}

func TestConnectRejectsMixedCredentialModes(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	withFakeExchange(t, nil, nil)

	response, _ := rig.do(t, http.MethodPut, "/api/connection",
		`{"enrollment_token":"`+enrollmentSentinel+`","org_id":"0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e","api_key":"k"}`)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("mixing enrollment_token with org_id/api_key must be a 400, got %d", response.StatusCode)
	}
}
