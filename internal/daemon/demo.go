package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/decionis/docker/internal/api"
)

// DecisionEvaluator is intentionally separate from HostedAPI so the daemon's
// read-only surfaces stay easy to fake. The production *api.Client implements
// it; the UI can only select one of the fixed proposals below and can never
// submit an arbitrary command through this demo route.
type DecisionEvaluator interface {
	EvaluateDecision(ctx context.Context, action api.ActionDescriptor) (*api.Verdict, error)
}

type demoScenario struct {
	ID          string
	Label       string
	Description string
	Lane        string
	Action      api.ActionDescriptor
}

type demoScenarioSummary struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Lane        string `json:"lane"`
}

type demoEvaluateRequest struct {
	ScenarioID string `json:"scenario_id"`
}

// demoEvaluateResponse repeats only fields the evaluate-decision contract
// actually returns; the outcome vocabulary is the protocol's own
// (APPROVE / REJECT / REVIEW / ESCALATE), rendered verbatim.
type demoEvaluateResponse struct {
	ScenarioID    string  `json:"scenario_id"`
	Label         string  `json:"label"`
	Lane          string  `json:"lane"`
	Outcome       string  `json:"outcome"`
	Mode          string  `json:"mode"`
	PolicyVersion string  `json:"policy_version"`
	EvaluationID  string  `json:"evaluation_id"`
	DossierID     string  `json:"dossier_id"`
	Confidence    float64 `json:"confidence"`
}

func highRiskScore() *float64 {
	score := 0.92
	return &score
}

// These are real evaluation proposals, not simulated verdicts. No shell,
// database, deployment, registry, or secret action is executed here: each
// descriptor is sent to the connected Decionis workspace and the control
// plane's exact response is returned.
var dockerDemoScenarios = []demoScenario{
	{
		ID: "container-list", Label: "List containers", Lane: "APPROVE",
		Description: "Read the running container list.",
		Action:      api.ActionDescriptor{DecisionType: "container.list", Channel: "docker_desktop_demo", Context: map[string]any{"resource": "containers", "read_only": true}},
	},
	{
		ID: "container-inspect", Label: "Inspect container", Lane: "APPROVE",
		Description: "Inspect the payments-api container.",
		Action:      api.ActionDescriptor{DecisionType: "container.inspect", Channel: "docker_desktop_demo", Context: map[string]any{"container": "payments-api", "read_only": true}},
	},
	{
		ID: "logs-read", Label: "Read service logs", Lane: "APPROVE",
		Description: "Read recent checkout logs.",
		Action:      api.ActionDescriptor{DecisionType: "logs.read", Channel: "docker_desktop_demo", Context: map[string]any{"service": "checkout", "read_only": true}},
	},
	{
		ID: "rm-rf", Label: "Recursive delete", Lane: "REJECT",
		Description: "Request rm -rf /workspace/cache.",
		Action:      api.ActionDescriptor{DecisionType: "filesystem.delete_recursive", Channel: "docker_desktop_demo", Context: map[string]any{"command": "rm -rf /workspace/cache", "path": "/workspace/cache"}},
	},
	{
		ID: "database-drop", Label: "Drop production database", Lane: "REJECT",
		Description: "Request deletion of analytics_prod.",
		Action:      api.ActionDescriptor{DecisionType: "database.drop", Channel: "docker_desktop_demo", Context: map[string]any{"database": "analytics_prod", "environment": "production"}},
	},
	{
		ID: "secrets-export", Label: "Export production secrets", Lane: "REJECT",
		Description: "Request a production secret export.",
		Action:      api.ActionDescriptor{DecisionType: "secrets.export", Channel: "docker_desktop_demo", Context: map[string]any{"secret_scope": "production", "destination": "local-file"}},
	},
	{
		ID: "deployment-promote", Label: "Promote deployment", Lane: "ESCALATE",
		Description: "Promote checkout from staging to production.",
		Action:      api.ActionDescriptor{DecisionType: "deployment.promote", RiskScore: highRiskScore(), Channel: "docker_desktop_demo", Context: map[string]any{"service": "checkout", "from": "staging", "to": "production", "risk": "high"}},
	},
	{
		ID: "database-migrate", Label: "Run production migration", Lane: "ESCALATE",
		Description: "Apply a schema migration in production.",
		Action:      api.ActionDescriptor{DecisionType: "database.migrate", RiskScore: highRiskScore(), Channel: "docker_desktop_demo", Context: map[string]any{"database": "orders", "environment": "production", "risk": "high"}},
	},
	{
		ID: "secrets-rotate", Label: "Rotate signing secrets", Lane: "ESCALATE",
		Description: "Rotate production signing credentials.",
		Action:      api.ActionDescriptor{DecisionType: "secrets.rotate", RiskScore: highRiskScore(), Channel: "docker_desktop_demo", Context: map[string]any{"secret_scope": "signing", "environment": "production", "risk": "high"}},
	},
	{
		ID: "image-publish", Label: "Publish release image", Lane: "ESCALATE",
		Description: "Publish checkout:2026.08 to production.",
		Action:      api.ActionDescriptor{DecisionType: "image.publish", RiskScore: highRiskScore(), Channel: "docker_desktop_demo", Context: map[string]any{"image": "checkout:2026.08", "registry": "production", "risk": "high"}},
	},
}

func findDemoScenario(id string) (demoScenario, bool) {
	for _, scenario := range dockerDemoScenarios {
		if scenario.ID == id {
			return scenario, true
		}
	}
	return demoScenario{}, false
}

func (d *Daemon) demoEvaluator() (DecisionEvaluator, bool) {
	d.mu.Lock()
	client := d.client
	d.mu.Unlock()
	if client == nil {
		return nil, false
	}
	evaluator, ok := client.(DecisionEvaluator)
	return evaluator, ok
}

func (d *Daemon) handleDemoScenarios(w http.ResponseWriter, _ *http.Request) {
	scenarios := make([]demoScenarioSummary, 0, len(dockerDemoScenarios))
	for _, scenario := range dockerDemoScenarios {
		scenarios = append(scenarios, demoScenarioSummary{
			ID: scenario.ID, Label: scenario.Label, Description: scenario.Description, Lane: scenario.Lane,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scenarios": scenarios,
		"count":     len(scenarios),
		"notice":    "These checks evaluate action proposals only; they do not execute the described actions.",
	})
}

func (d *Daemon) handleDemoEvaluate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request demoEvaluateRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Body must be JSON with scenario_id.")
		return
	}
	scenario, ok := findDemoScenario(request.ScenarioID)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_scenario", "Choose one of the published demo scenarios.")
		return
	}
	evaluator, ok := d.demoEvaluator()
	if !ok {
		writeError(w, http.StatusPreconditionRequired, "not_connected", "Connect to Decionis first.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()
	verdict, err := evaluator.EvaluateDecision(ctx, scenario.Action)
	if err != nil {
		if errors.Is(err, api.ErrEvaluationRefused) {
			writeError(w, http.StatusForbidden, "evaluation_refused", "The control plane declined to evaluate this proposal.")
			return
		}
		d.logger.Info("demo evaluation failed", "scenario_id", scenario.ID)
		writeError(w, http.StatusBadGateway, "upstream_unreachable", "The control plane could not evaluate this proposal.")
		return
	}

	// The decision report just changed; never serve the pre-evaluation cache.
	// Only the cache is touched: lastSync records report syncs, and this
	// handler did not perform one.
	d.mu.Lock()
	d.cache = nil
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, demoEvaluateResponse{
		ScenarioID: scenario.ID, Label: scenario.Label, Lane: scenario.Lane,
		Outcome: verdict.Outcome, Mode: verdict.Mode,
		PolicyVersion: verdict.PolicyVersion, EvaluationID: verdict.EvaluationID,
		DossierID: verdict.DossierID, Confidence: verdict.Confidence,
	})
}
