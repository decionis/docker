package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ActionDescriptor is a structured description of an action something wants
// to take. It is supplied by an SDK or middleware that already knows what it
// is about to do — this repository never infers actions from traffic
// (rules/coding.rules.md: no packet-level interception).
type ActionDescriptor struct {
	DecisionType string         `json:"decision_type"`
	Amount       *float64       `json:"amount,omitempty"`
	RiskScore    *float64       `json:"risk_score,omitempty"`
	Channel      string         `json:"channel,omitempty"`
	Context      map[string]any `json:"context,omitempty"`
}

// Verdict is the control plane's answer, rendered exactly as returned. The
// vocabulary belongs to the protocol (APPROVE / REJECT / REVIEW / ESCALATE);
// nothing here translates or re-scores it.
type Verdict struct {
	Outcome         string  `json:"outcome"`
	Confidence      float64 `json:"confidence"`
	PolicyVersion   string  `json:"policy_version"`
	Mode            string  `json:"mode"`
	Reason          string  `json:"reason"`
	ExecutionAction string  `json:"execution_action"`
	EvaluationID    string  `json:"evaluation_id"`
	DossierID       string  `json:"dossier_id"`
	WouldExecute    bool    `json:"would_execute"`
}

// ErrEvaluationRefused means the control plane declined to evaluate — an
// exhausted entitlement, a missing scope, or a rejected request. It is never
// treated as permission.
var ErrEvaluationRefused = errors.New("the control plane declined to evaluate this action")

// EvaluateDecision asks the control plane for a verdict on one action
// (POST /v1/protocol/evaluate-decision).
//
// It returns a verdict or an error, and never a default: no path in this
// function produces an APPROVE that the control plane did not send.
func (c *Client) EvaluateDecision(
	ctx context.Context,
	action ActionDescriptor,
) (*Verdict, error) {
	payload := map[string]any{
		"org_id":        c.config.OrgID,
		"decision_type": action.DecisionType,
	}
	if action.Amount != nil {
		payload["amount"] = *action.Amount
	}
	if action.RiskScore != nil {
		payload["risk_score"] = *action.RiskScore
	}
	if action.Channel != "" {
		payload["channel"] = action.Channel
	}
	if action.Context != nil {
		payload["context"] = action.Context
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("decionis api: evaluate-decision: encode failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.config.BaseURL+"/v1/protocol/evaluate-decision", bytes.NewReader(encoded))
	if err != nil {
		return nil, errors.New("decionis api: evaluate-decision: build request failed")
	}
	request.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, errors.New("decionis api: evaluate-decision unreachable")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, errors.New("decionis api: evaluate-decision: response read failed")
	}

	switch {
	case response.StatusCode == http.StatusOK:
		var verdict Verdict
		if err := json.Unmarshal(body, &verdict); err != nil || verdict.Outcome == "" {
			return nil, errors.New("decionis api: evaluate-decision: malformed response")
		}
		return &verdict, nil
	case response.StatusCode == http.StatusPaymentRequired,
		response.StatusCode == http.StatusForbidden,
		response.StatusCode == http.StatusUnauthorized,
		response.StatusCode == http.StatusBadRequest:
		return nil, ErrEvaluationRefused
	default:
		return nil, &StatusError{Op: "evaluate-decision", StatusCode: response.StatusCode}
	}
}
