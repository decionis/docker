// Package authority is the action-aware execution gate behind the
// decionis/authority image.
//
// It takes a structured action descriptor from an SDK or middleware that
// already knows what it is about to do, asks the control plane for a
// verdict, and enforces the answer. It never inspects traffic, never infers
// actions from packets, and never decides anything itself: no policy is
// evaluated here, and no outcome is invented here (rules/coding.rules.md —
// the protocol is the boundary).
//
// Fail closed is structural rather than intentional: Decide starts from a
// denial and only an explicit APPROVE from the control plane replaces it. A
// timeout, an unreachable plane, a refused evaluation, or a response this
// code does not understand all leave the denial in place. A timeout is
// never an approval (rules/security.rules.md Rule 0.3).
package authority

import (
	"context"
	"errors"
	"time"

	"github.com/decionis/docker/internal/api"
)

// Evaluator is the control-plane call the gate depends on.
type Evaluator interface {
	EvaluateDecision(ctx context.Context, action api.ActionDescriptor) (*api.Verdict, error)
}

// Decision is what the gate tells the caller to do.
type Decision struct {
	// Allowed is true only when the control plane returned APPROVE.
	Allowed bool `json:"allowed"`
	// Outcome is the control plane's own word, passed through unchanged, or
	// empty when no verdict was obtained.
	Outcome string `json:"outcome,omitempty"`
	// Reason explains the denial in the caller's terms. For a real verdict
	// it is the plane's reason; for a failure it names the failure.
	Reason string `json:"reason"`
	// Code is a stable machine-readable cause.
	Code string `json:"code"`
	// EvaluationID and DossierID are present when an evaluation happened,
	// so a denial can be traced to a signed record.
	EvaluationID  string `json:"evaluation_id,omitempty"`
	DossierID     string `json:"dossier_id,omitempty"`
	PolicyVersion string `json:"policy_version,omitempty"`
	Mode          string `json:"mode,omitempty"`
}

// outcomeApprove is the only outcome that permits execution. Everything else
// the protocol can return — REJECT, REVIEW, ESCALATE — withholds it.
const outcomeApprove = "APPROVE"

// Gate asks for verdicts and enforces them.
type Gate struct {
	evaluator Evaluator
	timeout   time.Duration
}

// NewGate builds a gate. A non-positive timeout falls back to a bounded
// default rather than waiting forever.
func NewGate(evaluator Evaluator, timeout time.Duration) *Gate {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Gate{evaluator: evaluator, timeout: timeout}
}

// Decide obtains a verdict for one action and returns what the caller may do.
//
// The returned Decision starts denied. Only an explicit APPROVE flips it.
func (g *Gate) Decide(ctx context.Context, action api.ActionDescriptor) Decision {
	denied := Decision{
		Allowed: false,
		Code:    "evaluation_unavailable",
		Reason:  "No verdict was obtained, so the action is not authorized.",
	}

	if action.DecisionType == "" {
		denied.Code = "invalid_action"
		denied.Reason = "The action descriptor has no decision_type, so it cannot be evaluated."
		return denied
	}

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	verdict, err := g.evaluator.EvaluateDecision(ctx, action)
	if err != nil {
		switch {
		case errors.Is(err, api.ErrEvaluationRefused):
			denied.Code = "evaluation_refused"
			denied.Reason = "The control plane declined to evaluate this action, so it is not authorized."
		case errors.Is(err, context.DeadlineExceeded):
			denied.Code = "evaluation_timeout"
			denied.Reason = "The control plane did not answer in time. A timeout is not an approval."
		default:
			denied.Code = "evaluation_unavailable"
			denied.Reason = "The control plane could not be reached, so the action is not authorized."
		}
		return denied
	}
	if verdict == nil {
		return denied
	}

	decision := Decision{
		Allowed:       verdict.Outcome == outcomeApprove,
		Outcome:       verdict.Outcome,
		Reason:        verdict.Reason,
		EvaluationID:  verdict.EvaluationID,
		DossierID:     verdict.DossierID,
		PolicyVersion: verdict.PolicyVersion,
		Mode:          verdict.Mode,
	}
	if decision.Allowed {
		decision.Code = "approved"
		if decision.Reason == "" {
			decision.Reason = "Approved by policy."
		}
		return decision
	}
	decision.Code = "not_approved"
	if decision.Reason == "" {
		// Never dress an unknown outcome as an explanation.
		decision.Reason = "The control plane did not approve this action."
	}
	return decision
}
