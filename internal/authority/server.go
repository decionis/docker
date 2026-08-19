package authority

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/decionis/docker/internal/api"
)

// maxRequestBody bounds every request body (rules/security.rules.md 3.3).
const maxRequestBody = 100 << 10

// Server exposes the gate over HTTP. Callers are SDKs and middleware that
// describe an action and honour the answer; the gate does not sit in a
// network path and cannot see traffic it was not told about.
type Server struct {
	gate    *Gate
	logger  *slog.Logger
	version string

	// Gateway-adapter behaviour: which decision type intercepted tool calls
	// are evaluated as, and whether their argument values are forwarded.
	decisionType     string
	forwardArguments bool
}

func NewServer(gate *Gate, logger *slog.Logger, version string) *Server {
	return &Server{gate: gate, logger: logger, version: version, decisionType: DefaultToolCallDecisionType}
}

// DefaultToolCallDecisionType is how an intercepted MCP tool call is
// described to the control plane unless configured otherwise.
const DefaultToolCallDecisionType = "agent-action"

// WithGatewayOptions configures the Docker MCP Gateway adapter.
func (s *Server) WithGatewayOptions(decisionType string, forwardArguments bool) *Server {
	if decisionType != "" {
		s.decisionType = decisionType
	}
	s.forwardArguments = forwardArguments
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /v1/gate", s.handleGate)
	// Docker MCP Gateway before:http: interceptor (different dialect — see gateway.go).
	mux.HandleFunc("POST /v1/mcp/before-tool-call", s.handleGatewayBefore)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, Decision{
			Allowed: false, Code: "not_found",
			Reason: "Unknown route; nothing was authorized.",
		})
	})
	return s.recoverPanics(mux)
}

// recoverPanics answers a denial, never a bare 500 that a caller might read
// as inconclusive-but-fine.
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic recovered", "route", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, Decision{
					Allowed: false, Code: "internal_error",
					Reason: "The gate failed internally, so the action is not authorized.",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

// handleGate evaluates one action.
//
// The HTTP status carries the answer as well as the body: 200 only when the
// action is authorized, 403 whenever it is not, whatever the cause. A caller
// that reads nothing but the status still fails closed.
func (s *Server) handleGate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var action api.ActionDescriptor
	if err := decoder.Decode(&action); err != nil {
		var maxBytesErr *http.MaxBytesError
		code, reason := "invalid_action", "The action descriptor could not be read, so nothing was authorized."
		if errors.As(err, &maxBytesErr) {
			code, reason = "payload_too_large", "The action descriptor exceeds the 100 KB limit; nothing was authorized."
		}
		writeJSON(w, http.StatusForbidden, Decision{Allowed: false, Code: code, Reason: reason})
		return
	}

	decision := s.gate.Decide(r.Context(), action)
	status := http.StatusForbidden
	if decision.Allowed {
		status = http.StatusOK
	}
	// Outcome and reason only; never the descriptor's context, which may
	// carry the caller's own data.
	s.logger.Info("gate decision",
		"decision_type", action.DecisionType,
		"allowed", decision.Allowed,
		"code", decision.Code,
		"outcome", decision.Outcome)
	writeJSON(w, status, decision)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
