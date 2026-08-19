package authority

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/decionis/docker/internal/api"
)

// Docker MCP Gateway `before:http:` interceptor adapter.
//
// The gateway decides allow/deny from the response BODY and ignores the
// status code entirely (verified in docker/mcp-gateway
// pkg/interceptors/interceptors.go — see docs/mcp-gateway-interceptor.md):
//
//	empty body      -> the tool call proceeds
//	non-empty body  -> parsed as an mcp.CallToolResult and returned to the
//	                   caller INSTEAD of running the tool
//
// That is the inverse of this service's own /v1/gate, which always answers
// with a decision document. Wiring the gateway at /v1/gate directly would
// therefore block every call, approvals included. This adapter exists to
// speak the gateway's dialect and nothing else.

// gatewayToolCall is the subset of the intercepted tools/call request this
// adapter reads. Docker marshals the whole MCP request; unknown fields are
// ignored rather than rejected, because the gateway owns that schema and
// may extend it.
type gatewayToolCall struct {
	Params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments,omitempty"`
	} `json:"params"`
}

// gatewayResult is an mcp.CallToolResult. `type` is set on content items:
// the MCP content type is a discriminator, and omitting it risks the
// gateway failing to parse the refusal — which would surface as a generic
// error instead of the reason the action was refused.
type gatewayResult struct {
	Content []gatewayContent `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

type gatewayContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// handleGatewayBefore evaluates one intercepted tool call.
//
// Allowing writes NOTHING: an empty body is the only way to let the call
// through. Refusing writes a CallToolResult carrying the outcome and reason,
// which the agent sees in place of the tool's output.
func (s *Server) handleGatewayBefore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	var call gatewayToolCall
	if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
		s.logger.Info("gateway interceptor: unreadable tool call")
		writeGatewayRefusal(w, "Decionis could not read this tool call, so it was not run.")
		return
	}
	if call.Params.Name == "" {
		s.logger.Info("gateway interceptor: tool call without a name")
		writeGatewayRefusal(w, "Decionis received a tool call with no tool name, so it was not run.")
		return
	}

	decision := s.gate.Decide(r.Context(), s.describeToolCall(call))

	s.logger.Info("gateway decision",
		"tool", call.Params.Name,
		"allowed", decision.Allowed,
		"code", decision.Code,
		"outcome", decision.Outcome)

	if decision.Allowed {
		// The empty body IS the approval. Writing anything here would
		// replace the tool's result with it.
		w.WriteHeader(http.StatusOK)
		return
	}
	writeGatewayRefusal(w, refusalText(call.Params.Name, decision))
}

// describeToolCall turns an intercepted call into an action descriptor.
//
// Argument VALUES are withheld by default: they are the agent's payload and
// can carry anything the caller passed, so forwarding them to the control
// plane is opt-in (-forward-arguments). Argument names are always sent —
// they describe the shape of the action without carrying its contents.
func (s *Server) describeToolCall(call gatewayToolCall) api.ActionDescriptor {
	context := map[string]any{
		"source":    "docker-mcp-gateway",
		"tool_name": call.Params.Name,
	}
	if s.forwardArguments {
		context["arguments"] = call.Params.Arguments
	} else if len(call.Params.Arguments) > 0 {
		names := make([]string, 0, len(call.Params.Arguments))
		for name := range call.Params.Arguments {
			names = append(names, name)
		}
		sort.Strings(names)
		context["argument_names"] = names
	}
	return api.ActionDescriptor{
		DecisionType: s.decisionType,
		Channel:      call.Params.Name,
		Context:      context,
	}
}

// refusalText is what the agent reads instead of the tool's output. It
// names the tool, the outcome the control plane returned, and its reason —
// never a generic failure, so the agent can tell "policy refused this" from
// "something broke".
func refusalText(tool string, decision Decision) string {
	text := "Decionis did not authorize " + tool + "."
	if decision.Outcome != "" {
		text += " Outcome: " + decision.Outcome + "."
	}
	if decision.Reason != "" {
		text += " " + decision.Reason
	}
	if decision.DossierID != "" {
		text += " Decision dossier: " + decision.DossierID + "."
	}
	return text
}

func writeGatewayRefusal(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(gatewayResult{
		Content: []gatewayContent{{Type: "text", Text: text}},
		IsError: true,
	})
}
