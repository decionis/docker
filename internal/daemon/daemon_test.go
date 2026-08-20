package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/canonicaljson"
	"github.com/decionis/docker/internal/dossier"
	"github.com/decionis/docker/internal/store"
)

const keySentinel = "sk-live-SENTINEL-NEVER-LOGGED"

type fakeHosted struct {
	listReports      func(ctx context.Context, mode string, limit int) (*api.ReportsResponse, error)
	getDossier       func(ctx context.Context, id string) (map[string]any, string, error)
	fetchJwks        func(ctx context.Context, url string) (*dossier.Jwks, error)
	evaluateDecision func(ctx context.Context, action api.ActionDescriptor) (*api.Verdict, error)
	listCalls        int
}

func (f *fakeHosted) ListReports(ctx context.Context, mode string, limit int) (*api.ReportsResponse, error) {
	f.listCalls++
	return f.listReports(ctx, mode, limit)
}

func (f *fakeHosted) GetDossier(ctx context.Context, id string) (map[string]any, string, error) {
	return f.getDossier(ctx, id)
}

func (f *fakeHosted) FetchJwks(ctx context.Context, url string) (*dossier.Jwks, error) {
	return f.fetchJwks(ctx, url)
}

func (f *fakeHosted) EvaluateDecision(ctx context.Context, action api.ActionDescriptor) (*api.Verdict, error) {
	return f.evaluateDecision(ctx, action)
}

func emptyReports(_ context.Context, mode string, _ int) (*api.ReportsResponse, error) {
	return &api.ReportsResponse{Service: "decionis-protocol", Mode: mode, Reports: []api.Report{}}, nil
}

type testRig struct {
	daemon *Daemon
	server *httptest.Server
	logs   *bytes.Buffer
	store  *store.Store
	fake   *fakeHosted
}

func newRig(t *testing.T, fake *fakeHosted) *testRig {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	st := store.New(filepath.Join(t.TempDir(), "data"))
	d := New("test", st, logger, func(config api.Config) (HostedAPI, error) {
		if _, err := api.NewClient(config); err != nil {
			return nil, err
		}
		return fake, nil
	})
	server := httptest.NewServer(d.Handler())
	t.Cleanup(server.Close)
	return &testRig{daemon: d, server: server, logs: logs, store: st, fake: fake}
}

func (rig *testRig) do(t *testing.T, method, path string, body string) (*http.Response, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, rig.server.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}
	t.Cleanup(func() { response.Body.Close() })
	var decoded map[string]any
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response, decoded
}

func connectBody() string {
	payload := map[string]string{
		"base_url": "https://api.decionis.com",
		"org_id":   "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e",
		"api_key":  keySentinel,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func TestConnectionNotStoredWhenUpstreamRejectsKey(t *testing.T) {
	rig := newRig(t, &fakeHosted{
		listReports: func(context.Context, string, int) (*api.ReportsResponse, error) {
			return nil, &api.StatusError{Op: "list reports", StatusCode: http.StatusUnauthorized}
		},
	})
	response, body := rig.do(t, http.MethodPut, "/api/connection", connectBody())
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%v)", response.StatusCode, body)
	}
	if _, _, ok, _ := rig.store.Load(); ok {
		t.Fatal("rejected credentials must not be stored (fail closed)")
	}
}

func TestConnectionNotStoredWhenUpstreamUnreachable(t *testing.T) {
	rig := newRig(t, &fakeHosted{
		listReports: func(context.Context, string, int) (*api.ReportsResponse, error) {
			return nil, context.DeadlineExceeded
		},
	})
	response, _ := rig.do(t, http.MethodPut, "/api/connection", connectBody())
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", response.StatusCode)
	}
	if _, _, ok, _ := rig.store.Load(); ok {
		t.Fatal("unverified credentials must not be stored")
	}
}

func TestConnectionStoredAndRestored(t *testing.T) {
	fake := &fakeHosted{listReports: emptyReports}
	rig := newRig(t, fake)
	response, _ := rig.do(t, http.MethodPut, "/api/connection", connectBody())
	if response.StatusCode != http.StatusOK {
		t.Fatalf("connect failed: %d", response.StatusCode)
	}
	connection, apiKey, ok, err := rig.store.Load()
	if err != nil || !ok {
		t.Fatalf("store must hold the connection: ok=%v err=%v", ok, err)
	}
	if connection.OrgID != "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e" || apiKey != keySentinel {
		t.Fatalf("stored connection mismatch: %+v", connection)
	}

	restored := New("test", rig.store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		func(api.Config) (HostedAPI, error) { return fake, nil })
	restored.LoadStoredConnection()
	server := httptest.NewServer(restored.Handler())
	defer server.Close()
	statusResponse, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	defer statusResponse.Body.Close()
	var status struct {
		Connected bool `json:"connected"`
	}
	_ = json.NewDecoder(statusResponse.Body).Decode(&status)
	if !status.Connected {
		t.Fatal("restored daemon must report connected")
	}
}

func TestDecisionsRequireConnection(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	response, body := rig.do(t, http.MethodGet, "/api/decisions", "")
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 when disconnected, got %d (%v)", response.StatusCode, body)
	}
}

func TestDecisionsFailClosedWhenUpstreamDies(t *testing.T) {
	healthy := true
	rig := newRig(t, &fakeHosted{
		listReports: func(ctx context.Context, mode string, limit int) (*api.ReportsResponse, error) {
			if healthy {
				return emptyReports(ctx, mode, limit)
			}
			return nil, &api.StatusError{Op: "list reports", StatusCode: http.StatusBadGateway}
		},
	})
	rig.do(t, http.MethodPut, "/api/connection", connectBody())

	healthy = false
	response, body := rig.do(t, http.MethodGet, "/api/decisions?mode=SHADOW", "")
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("upstream failure must surface as 502, got %d (%v)", response.StatusCode, body)
	}
}

func TestDecisionsCacheAvoidsRefetch(t *testing.T) {
	fake := &fakeHosted{listReports: emptyReports}
	rig := newRig(t, fake)
	rig.do(t, http.MethodPut, "/api/connection", connectBody())
	callsAfterConnect := fake.listCalls

	rig.do(t, http.MethodGet, "/api/decisions", "")
	rig.do(t, http.MethodGet, "/api/decisions", "")
	if fake.listCalls != callsAfterConnect+1 {
		t.Fatalf("expected 1 upstream fetch for two cached reads, got %d", fake.listCalls-callsAfterConnect)
	}
}

func TestOversizedConnectionBodyRejected(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	huge := `{"org_id":"0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e","api_key":"` + strings.Repeat("x", maxRequestBody+1) + `"}`
	response, _ := rig.do(t, http.MethodPut, "/api/connection", huge)
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", response.StatusCode)
	}
}

func TestUnknownFieldsRejected(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	response, _ := rig.do(t, http.MethodPut, "/api/connection",
		`{"org_id":"0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e","api_key":"k","unexpected":true}`)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown fields, got %d", response.StatusCode)
	}
}

func loadDossierFixture(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../dossier/testdata/dossier.json")
	if err != nil {
		t.Fatalf("read dossier fixture: %v", err)
	}
	decoded, err := canonicaljson.Decode(raw)
	if err != nil {
		t.Fatalf("decode dossier fixture: %v", err)
	}
	return decoded.(map[string]any)
}

func loadJwksFixture(t *testing.T) *dossier.Jwks {
	t.Helper()
	raw, err := os.ReadFile("../dossier/testdata/jwks.json")
	if err != nil {
		t.Fatalf("read jwks fixture: %v", err)
	}
	jwks := &dossier.Jwks{}
	if err := json.Unmarshal(raw, jwks); err != nil {
		t.Fatalf("decode jwks fixture: %v", err)
	}
	return jwks
}

func TestDossierVerificationEndToEnd(t *testing.T) {
	payload := loadDossierFixture(t)
	jwks := loadJwksFixture(t)
	rig := newRig(t, &fakeHosted{
		listReports: emptyReports,
		getDossier: func(context.Context, string) (map[string]any, string, error) {
			return payload, "https://api.decionis.com/v1/protocol/dossiers/x", nil
		},
		fetchJwks: func(context.Context, string) (*dossier.Jwks, error) { return jwks, nil },
	})
	rig.do(t, http.MethodPut, "/api/connection", connectBody())

	response, body := rig.do(t, http.MethodGet, "/api/dossiers/7c1d2e3f-4a5b-6c7d-8e9f-0a1b2c3d4e5f", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("dossier fetch failed: %d (%v)", response.StatusCode, body)
	}
	verification := body["verification"].(map[string]any)
	if verification["verified"] != true {
		t.Fatalf("fixture dossier must verify through the daemon: %v", verification)
	}
	reproducibility := body["reproducibility"].(map[string]any)
	if reproducibility["posture"] != "reproduction_ready" {
		t.Fatalf("unexpected reproducibility: %v", reproducibility)
	}
}

func TestDossierJwksFailureIsUnverified(t *testing.T) {
	payload := loadDossierFixture(t)
	rig := newRig(t, &fakeHosted{
		listReports: emptyReports,
		getDossier: func(context.Context, string) (map[string]any, string, error) {
			return payload, "https://api.decionis.com/v1/protocol/dossiers/x", nil
		},
		fetchJwks: func(context.Context, string) (*dossier.Jwks, error) {
			return nil, &api.StatusError{Op: "fetch jwks", StatusCode: http.StatusBadGateway}
		},
	})
	rig.do(t, http.MethodPut, "/api/connection", connectBody())

	response, body := rig.do(t, http.MethodGet, "/api/dossiers/abc", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with unverified result, got %d", response.StatusCode)
	}
	verification := body["verification"].(map[string]any)
	if verification["verified"] != false {
		t.Fatal("a missing JWKS must yield an unverified dossier (fail closed)")
	}
}

// TestApiKeyNeverLogged exercises connect (success + failure) and decision
// flows, then asserts the API key appears nowhere in the daemon's log output
// (rules/security.rules.md Rules 2.4, 7.2).
func TestApiKeyNeverLogged(t *testing.T) {
	healthy := false
	rig := newRig(t, &fakeHosted{
		listReports: func(ctx context.Context, mode string, limit int) (*api.ReportsResponse, error) {
			if healthy {
				return emptyReports(ctx, mode, limit)
			}
			return nil, &api.StatusError{Op: "list reports", StatusCode: http.StatusUnauthorized}
		},
	})

	rig.do(t, http.MethodPut, "/api/connection", connectBody()) // rejected
	healthy = true
	rig.do(t, http.MethodPut, "/api/connection", connectBody()) // accepted
	rig.do(t, http.MethodGet, "/api/decisions", "")
	rig.do(t, http.MethodDelete, "/api/connection", "")

	if strings.Contains(rig.logs.String(), keySentinel) {
		t.Fatal("api key leaked into logs")
	}
}
