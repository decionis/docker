package authority

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/decionis/docker/internal/api"
)

func gatewayServer(evaluator Evaluator, forwardArgs bool) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(NewGate(evaluator, time.Second), logger, "test").
		WithGatewayOptions("", forwardArgs)
}

func postToolCall(server *Server, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/before-tool-call", strings.NewReader(body))
	server.Handler().ServeHTTP(rec, req)
	return rec
}

const toolCallJSON = `{"method":"tools/call","params":{"name":"delete_database","arguments":{"name":"prod","force":true}}}`

func TestApprovalMustSendAnEmptyBody(t *testing.T) {
	// The gateway reads the BODY, not the status: anything written here
	// would replace the tool's result. An approval must be silent.
	rec := postToolCall(gatewayServer(&stubEvaluator{
		verdict: &api.Verdict{Outcome: "APPROVE", Reason: "within limits"},
	}, false), toolCallJSON)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("an approval must write nothing, got %q", rec.Body.String())
	}
}

func TestRefusalReturnsACallToolResultTheAgentCanRead(t *testing.T) {
	rec := postToolCall(gatewayServer(&stubEvaluator{
		verdict: &api.Verdict{
			Outcome: "REJECT", Reason: "production deletes require approval",
			DossierID: "dos-42",
		},
	}, false), toolCallJSON)

	if rec.Body.Len() == 0 {
		t.Fatal("a refusal must write a body — an empty body lets the call run")
	}
	var result gatewayResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("refusal must parse as an mcp.CallToolResult: %v", err)
	}
	if !result.IsError {
		t.Fatal("a refusal must be marked isError")
	}
	if len(result.Content) == 0 || result.Content[0].Type != "text" {
		t.Fatalf("content needs its type discriminator or the gateway cannot parse it: %+v", result.Content)
	}
	text := result.Content[0].Text
	for _, want := range []string{"delete_database", "REJECT", "production deletes require approval", "dos-42"} {
		if !strings.Contains(text, want) {
			t.Fatalf("refusal text %q is missing %q", text, want)
		}
	}
}

func TestEveryFailureRefusesRatherThanLettingTheToolRun(t *testing.T) {
	// Each of these must produce a non-empty body. An empty body is an
	// approval, so a bug that writes nothing would silently permit the call.
	cases := []struct {
		name      string
		evaluator Evaluator
		body      string
	}{
		{"control plane unreachable", &stubEvaluator{err: errors.New("connection refused")}, toolCallJSON},
		{"evaluation refused", &stubEvaluator{err: api.ErrEvaluationRefused}, toolCallJSON},
		{"review outcome", &stubEvaluator{verdict: &api.Verdict{Outcome: "REVIEW"}}, toolCallJSON},
		{"unparseable body", &stubEvaluator{}, `not json`},
		{"no tool name", &stubEvaluator{}, `{"params":{"arguments":{}}}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rec := postToolCall(gatewayServer(testCase.evaluator, false), testCase.body)
			if rec.Body.Len() == 0 {
				t.Fatal("empty body would let the tool run")
			}
			var result gatewayResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("refusal must stay parseable: %v", err)
			}
			if !result.IsError {
				t.Fatal("refusal must be marked isError")
			}
		})
	}
}

func TestArgumentValuesAreWithheldUnlessAsked(t *testing.T) {
	stub := &stubEvaluator{verdict: &api.Verdict{Outcome: "APPROVE"}}
	server := gatewayServer(stub, false)
	server.gate.evaluator = &capturingEvaluator{inner: stub}
	captured := server.gate.evaluator.(*capturingEvaluator)

	postToolCall(server, toolCallJSON)

	context := captured.last.Context
	if _, leaked := context["arguments"]; leaked {
		t.Fatalf("argument values must not be sent by default: %+v", context)
	}
	names, ok := context["argument_names"].([]string)
	if !ok || len(names) != 2 || names[0] != "force" || names[1] != "name" {
		t.Fatalf("argument names should describe the shape, got %+v", context["argument_names"])
	}
	if context["tool_name"] != "delete_database" {
		t.Fatalf("tool name must reach the control plane, got %+v", context["tool_name"])
	}
}

func TestArgumentValuesAreSentWhenAsked(t *testing.T) {
	stub := &stubEvaluator{verdict: &api.Verdict{Outcome: "APPROVE"}}
	server := gatewayServer(stub, true)
	server.gate.evaluator = &capturingEvaluator{inner: stub}
	captured := server.gate.evaluator.(*capturingEvaluator)

	postToolCall(server, toolCallJSON)

	arguments, ok := captured.last.Context["arguments"].(map[string]any)
	if !ok || arguments["name"] != "prod" {
		t.Fatalf("opting in must forward values, got %+v", captured.last.Context["arguments"])
	}
}

type capturingEvaluator struct {
	inner Evaluator
	last  api.ActionDescriptor
}

func (c *capturingEvaluator) EvaluateDecision(ctx context.Context, action api.ActionDescriptor) (*api.Verdict, error) {
	c.last = action
	return c.inner.EvaluateDecision(ctx, action)
}
