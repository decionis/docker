package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientEnforcesHTTPS(t *testing.T) {
	_, err := NewClient(Config{BaseURL: "http://api.decionis.com", OrgID: "o", APIKey: "k"})
	if err == nil {
		t.Fatal("non-localhost http must be rejected (rules/security.rules.md Rule 2.6)")
	}
	if _, err := NewClient(Config{BaseURL: "http://127.0.0.1:9", OrgID: "o", APIKey: "k"}); err != nil {
		t.Fatalf("localhost http must be allowed for development: %v", err)
	}
	if _, err := NewClient(Config{BaseURL: "https://user:pass@api.decionis.com", OrgID: "o", APIKey: "k"}); err == nil {
		t.Fatal("credential-bearing URLs must be rejected")
	}
}

func TestListReportsSendsBearerAndQuery(t *testing.T) {
	var gotAuth, gotQuery, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(ReportsResponse{Service: "decionis-protocol", Mode: "ENFORCEMENT"})
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, OrgID: "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e", APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	response, err := client.ListReports(context.Background(), "ENFORCEMENT", 25)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if response.Service != "decionis-protocol" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("expected bearer auth per the published contract, got %q", gotAuth)
	}
	if gotPath != "/v1/protocol/shadow/evaluate-decision/reports" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotQuery != "limit=25&mode=ENFORCEMENT&org_id=0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e" {
		t.Fatalf("unexpected query %q", gotQuery)
	}
}

func TestStatusErrorsAreSanitized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"secret":"body content that must never surface"}`))
	}))
	defer server.Close()

	client, _ := NewClient(Config{BaseURL: server.URL, OrgID: "o", APIKey: "sk-test"})
	_, err := client.ListReports(context.Background(), "ENFORCEMENT", 1)
	if err == nil {
		t.Fatal("401 must be an error")
	}
	if !IsAuthError(err) {
		t.Fatalf("401 must classify as auth error: %v", err)
	}
	if got := err.Error(); got != "decionis api: list reports returned HTTP 401" {
		t.Fatalf("error must carry op+status only, got %q", got)
	}
}

func TestFetchJwksRequiresHTTPS(t *testing.T) {
	client, _ := NewClient(Config{BaseURL: "https://api.decionis.com", OrgID: "o", APIKey: "k"})
	if _, err := client.FetchJwks(context.Background(), "http://evil.example/jwks.json"); err == nil {
		t.Fatal("plain-http JWKS URLs must be rejected")
	}
}
