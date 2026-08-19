package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeEnrollmentSuccess(t *testing.T) {
	var gotPath, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body struct {
			EnrollmentToken string `json:"enrollment_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotToken = body.EnrollmentToken
		_ = json.NewEncoder(w).Encode(map[string]any{
			"org_id":         "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e",
			"raw_key":        "dcn_live_minted_key",
			"connector_slug": "docker-desktop",
			"environment":    "production",
		})
	}))
	defer server.Close()

	exchange, err := mustPublicClient(t, server.URL).ExchangeEnrollment(context.Background(), "dcn_enroll_0123456789abcdef")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if gotPath != "/v1/connectors/enrollments/exchange" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotToken != "dcn_enroll_0123456789abcdef" {
		t.Fatalf("token not forwarded verbatim")
	}
	if exchange.OrgID != "0b6f8a3e-6f2a-4c3e-9b1d-2f4a5b6c7d8e" || exchange.RawKey != "dcn_live_minted_key" {
		t.Fatalf("unexpected exchange: %+v", exchange)
	}
}

func TestExchangeEnrollmentErrorMapping(t *testing.T) {
	status := http.StatusUnauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":"ENROLLMENT_INVALID","message":"server detail that must not surface"}`))
	}))
	defer server.Close()

	_, err := mustPublicClient(t, server.URL).ExchangeEnrollment(context.Background(), "dcn_enroll_0123456789abcdef")
	if !errors.Is(err, ErrEnrollmentInvalid) {
		t.Fatalf("401 must map to ErrEnrollmentInvalid, got %v", err)
	}

	status = http.StatusConflict
	_, err = mustPublicClient(t, server.URL).ExchangeEnrollment(context.Background(), "dcn_enroll_0123456789abcdef")
	if !errors.Is(err, ErrEnrollmentUsed) {
		t.Fatalf("409 must map to ErrEnrollmentUsed, got %v", err)
	}
}

func TestExchangeEnrollmentGuards(t *testing.T) {
	// A non-loopback http base URL must be refused when the client is built,
	// before any request can be shaped.
	if _, err := NewPublicClient("http://api.decionis.com"); err == nil {
		t.Fatal("non-localhost http must be rejected")
	}
	if _, err := mustPublicClient(t, "https://api.decionis.com").ExchangeEnrollment(context.Background(), "short"); !errors.Is(err, ErrEnrollmentInvalid) {
		t.Fatal("obviously invalid tokens must be rejected locally without a network call")
	}
}

// mustPublicClient builds a pre-auth client for a test server, failing the
// test if the base URL does not pass the transport gate.
func mustPublicClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := NewPublicClient(baseURL)
	if err != nil {
		t.Fatalf("NewPublicClient(%q): %v", baseURL, err)
	}
	return client
}
