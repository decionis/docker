package daemon

import (
	"context"
	"net/http"
	"testing"

	"github.com/decionis/docker/internal/api"
)

func TestDockerDemoScenariosCoverTenFixedPolicyChecks(t *testing.T) {
	if len(dockerDemoScenarios) != 10 {
		t.Fatalf("expected 10 scenarios, got %d", len(dockerDemoScenarios))
	}
	lanes := map[string]int{}
	ids := map[string]bool{}
	for _, scenario := range dockerDemoScenarios {
		if ids[scenario.ID] {
			t.Fatalf("duplicate scenario id %q", scenario.ID)
		}
		ids[scenario.ID] = true
		lanes[scenario.Lane]++
		if scenario.Action.Channel != "docker_desktop_demo" {
			t.Fatalf("scenario %q must be attributable to the Docker demo", scenario.ID)
		}
	}
	if lanes["APPROVE"] != 3 || lanes["BLOCK"] != 3 || lanes["ESCALATE"] != 4 {
		t.Fatalf("unexpected lane distribution: %#v", lanes)
	}
	rm, ok := findDemoScenario("rm-rf")
	if !ok || rm.Action.Context["command"] != "rm -rf /workspace/cache" {
		t.Fatalf("rm-rf scenario must carry the exact destructive proposal: %#v", rm.Action)
	}
}

func TestDemoEvaluateReturnsExactVerdictAndInvalidatesReportsCache(t *testing.T) {
	var evaluated api.ActionDescriptor
	fake := &fakeHosted{
		listReports: emptyReports,
		evaluateDecision: func(_ context.Context, action api.ActionDescriptor) (*api.Verdict, error) {
			evaluated = action
			return &api.Verdict{
				Outcome: "REJECT", ExecutionAction: "BLOCK", Mode: "ENFORCEMENT",
				PolicyVersion: "docker-desktop-starter-v1", EvaluationID: "evaluation-1",
				DossierID: "dossier-1", Confidence: 1,
			}, nil
		},
	}
	rig := newRig(t, fake)
	response, _ := rig.do(t, http.MethodPut, "/api/connection", connectBody())
	if response.StatusCode != http.StatusOK {
		t.Fatalf("connect failed: %d", response.StatusCode)
	}
	rig.do(t, http.MethodGet, "/api/decisions", "")
	callsBefore := fake.listCalls

	response, body := rig.do(t, http.MethodPost, "/api/demo/evaluate", `{"scenario_id":"rm-rf"}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("demo evaluation failed: %d (%v)", response.StatusCode, body)
	}
	if body["outcome"] != "REJECT" || body["execution_action"] != "BLOCK" || body["policy_version"] != "docker-desktop-starter-v1" {
		t.Fatalf("response must preserve the control-plane verdict: %#v", body)
	}
	if evaluated.DecisionType != "filesystem.delete_recursive" || evaluated.Context["command"] != "rm -rf /workspace/cache" {
		t.Fatalf("wrong action descriptor: %#v", evaluated)
	}

	rig.do(t, http.MethodGet, "/api/decisions", "")
	if fake.listCalls != callsBefore+1 {
		t.Fatalf("a successful evaluation must invalidate cached reports")
	}
}

func TestDemoEvaluateRefusalFailsClosed(t *testing.T) {
	fake := &fakeHosted{
		listReports: emptyReports,
		evaluateDecision: func(context.Context, api.ActionDescriptor) (*api.Verdict, error) {
			return nil, api.ErrEvaluationRefused
		},
	}
	rig := newRig(t, fake)
	rig.do(t, http.MethodPut, "/api/connection", connectBody())
	response, body := rig.do(t, http.MethodPost, "/api/demo/evaluate", `{"scenario_id":"database-drop"}`)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("refusal must surface as 403, got %d (%v)", response.StatusCode, body)
	}
}

func TestDemoEvaluateRejectsUnknownScenario(t *testing.T) {
	rig := newRig(t, &fakeHosted{listReports: emptyReports})
	response, _ := rig.do(t, http.MethodPost, "/api/demo/evaluate", `{"scenario_id":"shell-from-user"}`)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown scenario must be rejected, got %d", response.StatusCode)
	}
}
