package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/store"
)

func signupDaemon(t *testing.T) *Daemon {
	t.Helper()
	return New("test", store.New(t.TempDir()), slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(_ api.Config) (HostedAPI, error) {
			return &fakeHosted{listReports: emptyReports}, nil
		})
}

func postJSON(d *Daemon, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	d.Handler().ServeHTTP(rec, req)
	return rec
}

func putJSON(d *Daemon, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/connection", strings.NewReader(body))
	d.Handler().ServeHTTP(rec, req)
	return rec
}

func stubProvision(t *testing.T, workspace *api.Workspace, err error) *string {
	t.Helper()
	original := provisionWorkspace
	seenUsername := ""
	provisionWorkspace = func(_ context.Context, _ string, username string) (*api.Workspace, error) {
		seenUsername = username
		return workspace, err
	}
	t.Cleanup(func() { provisionWorkspace = original })
	return &seenUsername
}

func stubAccountConnect(t *testing.T, workspace *api.Workspace, err error) *[2]string {
	t.Helper()
	original := connectWithAccount
	var seen [2]string
	connectWithAccount = func(_ context.Context, _ string, email, password string) (*api.Workspace, error) {
		seen[0], seen[1] = email, password
		return workspace, err
	}
	t.Cleanup(func() { connectWithAccount = original })
	return &seen
}

func TestAutoConnectMintsAWorkspaceWithNoInput(t *testing.T) {
	d := signupDaemon(t)
	seenUsername := stubProvision(t, &api.Workspace{
		OrgID: "11111111-1111-4111-8111-111111111111", RawKey: "dcn_live_x",
		OrgName: "My Docker Workspace", Provisional: true,
	}, nil)

	rec := postJSON(d, "/api/connect/auto", "{}")
	if rec.Code != http.StatusOK {
		t.Fatalf("auto connect: got %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"connected":true`) {
		t.Fatalf("status after auto connect: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "dcn_live_x") {
		t.Fatal("status leaked the minted key")
	}
	if *seenUsername != "" {
		t.Fatalf("no username should be sent by default, got %q", *seenUsername)
	}
}

func TestAutoConnectRefusesWhenAlreadyConnected(t *testing.T) {
	d := signupDaemon(t)
	stubProvision(t, &api.Workspace{OrgID: "11111111-1111-4111-8111-111111111111", RawKey: "k"}, nil)
	if rec := postJSON(d, "/api/connect/auto", "{}"); rec.Code != http.StatusOK {
		t.Fatalf("first connect: %d", rec.Code)
	}
	// A second call must not replace a working connection with a new empty one.
	rec := postJSON(d, "/api/connect/auto", "{}")
	if rec.Code != http.StatusConflict {
		t.Fatalf("second auto connect: got %d, want 409", rec.Code)
	}
}

func TestAutoConnectStaysDisconnectedWhenSignupFails(t *testing.T) {
	d := signupDaemon(t)
	stubProvision(t, nil, api.ErrSignupUnavailable)

	rec := postJSON(d, "/api/connect/auto", "{}")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("failed signup: got %d, want 502", rec.Code)
	}
	statusRec := httptest.NewRecorder()
	d.Handler().ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if strings.Contains(statusRec.Body.String(), `"connected":true`) {
		t.Fatal("daemon must stay disconnected after a failed signup")
	}
}

func TestConnectWithEmailAndPassword(t *testing.T) {
	d := signupDaemon(t)
	seen := stubAccountConnect(t, &api.Workspace{
		OrgID: "11111111-1111-4111-8111-111111111111", RawKey: "dcn_live_y", OrgName: "Acme",
	}, nil)

	rec := putJSON(d, `{"email":"owner@acme.test","password":"correct horse"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("credentials connect: got %d body %s", rec.Code, rec.Body.String())
	}
	if seen[0] != "owner@acme.test" || seen[1] != "correct horse" {
		t.Fatalf("credentials not forwarded verbatim: %+v", *seen)
	}
	body := rec.Body.String()
	if strings.Contains(body, "correct horse") || strings.Contains(body, "dcn_live_y") {
		t.Fatalf("response leaked credential material: %s", body)
	}

	// The password must never reach the store — only the minted key does.
	connection, apiKey, ok, err := d.store.Load()
	if err != nil || !ok {
		t.Fatalf("store load: ok=%v err=%v", ok, err)
	}
	if apiKey == "correct horse" || strings.Contains(connection.OrgID, "correct") {
		t.Fatal("password reached the store")
	}
	if apiKey != "dcn_live_y" {
		t.Fatalf("stored key = %q, want the minted key", apiKey)
	}
}

func TestConnectCredentialErrorsMapToStableCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
		code string
	}{
		{"invalid", api.ErrCredentialsInvalid, http.StatusUnauthorized, "invalid_credentials"},
		{"locked", api.ErrCredentialsLocked, http.StatusTooManyRequests, "account_locked"},
		{"no workspace", api.ErrNoWorkspace, http.StatusForbidden, "no_workspace"},
		{"unreachable", api.ErrSignupUnavailable, http.StatusBadGateway, "upstream_unreachable"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			d := signupDaemon(t)
			stubAccountConnect(t, nil, testCase.err)
			rec := putJSON(d, `{"email":"owner@acme.test","password":"nope"}`)
			if rec.Code != testCase.want {
				t.Fatalf("got %d, want %d (%s)", rec.Code, testCase.want, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), testCase.code) {
				t.Fatalf("body %s missing code %s", rec.Body.String(), testCase.code)
			}
		})
	}
}

func TestConnectRejectsMixedCredentialShapes(t *testing.T) {
	d := signupDaemon(t)
	stubAccountConnect(t, &api.Workspace{OrgID: "o", RawKey: "k"}, nil)
	rec := putJSON(d, `{"email":"owner@acme.test","password":"x","enrollment_token":"dcn_enroll_abcdefghijklmnop"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mixed shapes: got %d, want 400", rec.Code)
	}
}

func TestConnectRequiresBothEmailAndPassword(t *testing.T) {
	d := signupDaemon(t)
	rec := putJSON(d, `{"email":"owner@acme.test"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("password missing: got %d, want 400", rec.Code)
	}
}
