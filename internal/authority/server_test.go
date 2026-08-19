package authority

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/decionis/docker/internal/api"
)

func testServer(evaluator Evaluator) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(NewGate(evaluator, time.Second), logger, "test").Handler()
}

func postGate(handler http.Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/gate", strings.NewReader(body)))
	return rec
}

func decodeDecision(t *testing.T, rec *httptest.ResponseRecorder) Decision {
	t.Helper()
	var decision Decision
	if err := json.Unmarshal(rec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("response was not a decision: %s", rec.Body.String())
	}
	return decision
}

func TestAuthorizedActionAnswers200(t *testing.T) {
	handler := testServer(&stubEvaluator{verdict: &api.Verdict{Outcome: "APPROVE", Reason: "ok"}})
	rec := postGate(handler, `{"decision_type":"refund","amount":10}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !decodeDecision(t, rec).Allowed {
		t.Fatal("body must agree with the status")
	}
}

func TestEveryDenialAnswers403SoStatusAloneFailsClosed(t *testing.T) {
	// A caller that reads only the HTTP status must still deny. Every one of
	// these paths has to be 403, not 500 or 200-with-a-flag.
	cases := []struct {
		name      string
		evaluator Evaluator
		body      string
	}{
		{"rejected", &stubEvaluator{verdict: &api.Verdict{Outcome: "REJECT"}}, `{"decision_type":"refund"}`},
		{"review", &stubEvaluator{verdict: &api.Verdict{Outcome: "REVIEW"}}, `{"decision_type":"refund"}`},
		{"refused", &stubEvaluator{err: api.ErrEvaluationRefused}, `{"decision_type":"refund"}`},
		{"malformed body", &stubEvaluator{}, `not json`},
		{"unknown field", &stubEvaluator{}, `{"decision_type":"refund","sneaky":true}`},
		{"missing decision_type", &stubEvaluator{}, `{"amount":10}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rec := postGate(testServer(testCase.evaluator), testCase.body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("got %d, want 403: %s", rec.Code, rec.Body.String())
			}
			if decodeDecision(t, rec).Allowed {
				t.Fatal("a denial must never report allowed")
			}
		})
	}
}

func TestUnknownRouteDenies(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(&stubEvaluator{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/anything", nil))
	if rec.Code != http.StatusNotFound || decodeDecision(t, rec).Allowed {
		t.Fatalf("unknown route must deny: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOversizedDescriptorDenies(t *testing.T) {
	huge := `{"decision_type":"refund","context":{"blob":"` + strings.Repeat("x", 200<<10) + `"}}`
	rec := postGate(testServer(&stubEvaluator{}), huge)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("oversized body must deny, got %d", rec.Code)
	}
}

func TestHealthDoesNotAuthorizeAnything(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer(&stubEvaluator{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health should answer 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "allowed") {
		t.Fatal("health must not look like a decision")
	}
}

func TestCallerContextIsNotLogged(t *testing.T) {
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := NewServer(
		NewGate(&stubEvaluator{verdict: &api.Verdict{Outcome: "APPROVE"}}, time.Second),
		logger, "test",
	).Handler()

	postGate(handler, `{"decision_type":"refund","context":{"customer_email":"secret@example.com"}}`)
	if strings.Contains(logs.String(), "secret@example.com") {
		t.Fatalf("the caller's data must not reach logs: %s", logs.String())
	}
}

var _ = context.Background
