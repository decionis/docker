package authority

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/decionis/docker/internal/api"
)

type stubEvaluator struct {
	verdict *api.Verdict
	err     error
	calls   int
	block   time.Duration
}

func (s *stubEvaluator) EvaluateDecision(ctx context.Context, _ api.ActionDescriptor) (*api.Verdict, error) {
	s.calls++
	if s.block > 0 {
		select {
		case <-time.After(s.block):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.verdict, s.err
}

func refund() api.ActionDescriptor { return api.ActionDescriptor{DecisionType: "refund"} }

func TestApproveIsTheOnlyThingThatPermits(t *testing.T) {
	gate := NewGate(&stubEvaluator{verdict: &api.Verdict{
		Outcome: "APPROVE", Reason: "within limits", EvaluationID: "e1", DossierID: "d1",
	}}, time.Second)

	decision := gate.Decide(context.Background(), refund())
	if !decision.Allowed {
		t.Fatalf("APPROVE must permit: %+v", decision)
	}
	if decision.EvaluationID != "e1" || decision.DossierID != "d1" {
		t.Fatalf("a permitted action must stay traceable: %+v", decision)
	}
}

func TestEveryOtherOutcomeWithholdsPermission(t *testing.T) {
	// The protocol's vocabulary, passed through untranslated. Anything that
	// is not APPROVE — including an outcome this build has never seen —
	// leaves the action unauthorized.
	for _, outcome := range []string{"REJECT", "REVIEW", "ESCALATE", "SOMETHING_NEW", ""} {
		gate := NewGate(&stubEvaluator{verdict: &api.Verdict{Outcome: outcome}}, time.Second)
		decision := gate.Decide(context.Background(), refund())
		if decision.Allowed {
			t.Fatalf("outcome %q must not permit execution", outcome)
		}
		if outcome != "" && decision.Outcome != outcome {
			t.Fatalf("outcome %q must be reported verbatim, got %q", outcome, decision.Outcome)
		}
	}
}

func TestTimeoutIsNeverAnApproval(t *testing.T) {
	stub := &stubEvaluator{block: 200 * time.Millisecond, verdict: &api.Verdict{Outcome: "APPROVE"}}
	gate := NewGate(stub, 10*time.Millisecond)

	decision := gate.Decide(context.Background(), refund())
	if decision.Allowed {
		t.Fatal("a timeout must never approve")
	}
	if decision.Code != "evaluation_timeout" {
		t.Fatalf("code = %q, want evaluation_timeout", decision.Code)
	}
}

func TestUnreachablePlaneDenies(t *testing.T) {
	gate := NewGate(&stubEvaluator{err: errors.New("connection refused")}, time.Second)
	decision := gate.Decide(context.Background(), refund())
	if decision.Allowed || decision.Code != "evaluation_unavailable" {
		t.Fatalf("unreachable must deny: %+v", decision)
	}
}

func TestRefusedEvaluationDenies(t *testing.T) {
	// An exhausted entitlement or a missing scope is a refusal to evaluate,
	// which is not permission.
	gate := NewGate(&stubEvaluator{err: api.ErrEvaluationRefused}, time.Second)
	decision := gate.Decide(context.Background(), refund())
	if decision.Allowed || decision.Code != "evaluation_refused" {
		t.Fatalf("refusal must deny: %+v", decision)
	}
}

func TestNilVerdictWithoutErrorStillDenies(t *testing.T) {
	// Defensive: an evaluator that returns nothing and no error must not
	// fall through to an approval.
	gate := NewGate(&stubEvaluator{}, time.Second)
	decision := gate.Decide(context.Background(), refund())
	if decision.Allowed {
		t.Fatalf("a nil verdict must deny: %+v", decision)
	}
}

func TestActionWithoutDecisionTypeIsNotSentUpstream(t *testing.T) {
	stub := &stubEvaluator{verdict: &api.Verdict{Outcome: "APPROVE"}}
	gate := NewGate(stub, time.Second)

	decision := gate.Decide(context.Background(), api.ActionDescriptor{})
	if decision.Allowed || decision.Code != "invalid_action" {
		t.Fatalf("an unevaluatable action must deny: %+v", decision)
	}
	if stub.calls != 0 {
		t.Fatal("an invalid descriptor must not reach the control plane")
	}
}

func TestGateNeverInventsAReason(t *testing.T) {
	gate := NewGate(&stubEvaluator{verdict: &api.Verdict{Outcome: "REJECT"}}, time.Second)
	decision := gate.Decide(context.Background(), refund())
	if decision.Reason == "" {
		t.Fatal("a denial must say something")
	}
	// It must not claim a policy reason the plane did not give.
	if decision.Reason == "Approved by policy." {
		t.Fatalf("denial reason must not read as approval: %q", decision.Reason)
	}
}
