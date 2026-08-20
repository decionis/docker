import { describe, expect, it } from "vitest";

import {
  EMPTY_DECISION_FILTERS,
  filterDecisionReports,
  type DecisionFilters,
} from "../src/features/decisions/DecisionFilters";
import type { DecisionReport } from "../src/services/BackendClient";

function report(overrides: Partial<DecisionReport> = {}): DecisionReport {
  return {
    evaluation_id: "eval-001",
    dossier_id: "dossier-001",
    created_at: "2026-08-20T10:00:00Z",
    decision_type: "deploy",
    decision_domain: "production",
    amount: 125,
    risk_score: 0.7,
    channel: "mcp",
    policy_version: "policy-v2",
    mode: "ENFORCEMENT",
    outcome: "ESCALATE",
    confidence: 0.91,
    would_execute: false,
    execution_action: "HAND_OFF",
    reason: "Manual review is required",
    policy_guard_reason: "Production guard",
    policy_evaluation_resolution: "rule_selected",
    selected_rule_id: "deploy-production",
    dossier_api_path: "/v1/protocol/dossiers/dossier-001",
    ...overrides,
  };
}

function filters(overrides: Partial<DecisionFilters>): DecisionFilters {
  return { ...EMPTY_DECISION_FILTERS, ...overrides };
}

describe("decision investigation filters", () => {
  it.each([
    ["decision type", "DEPLOY"],
    ["domain", "PRODUCTION"],
    ["reason", "manual REVIEW"],
    ["policy version", "POLICY-V2"],
    ["selected rule", "DEPLOY-PRODUCTION"],
    ["evaluation ID", "EVAL-001"],
    ["dossier ID", "DOSSIER-001"],
  ])("searches %s case-insensitively", (_field, search) => {
    expect(filterDecisionReports([report()], filters({ search }))).toHaveLength(1);
  });

  it("combines outcome, execution, policy, domain/type, and text filters with AND semantics", () => {
    const matching = report();
    const wrongOutcome = report({ evaluation_id: "eval-002", outcome: "APPROVE" });
    const wrongPolicy = report({ evaluation_id: "eval-003", policy_version: "policy-v1" });

    expect(
      filterDecisionReports(
        [matching, wrongOutcome, wrongPolicy],
        filters({
          search: "manual",
          outcome: "escalate",
          executionAction: "hand_off",
          policyVersion: "POLICY-V2",
          domainOrType: "domain:PRODUCTION",
        }),
      ).map((item) => item.evaluation_id),
    ).toEqual(["eval-001"]);
  });

  it("restores every loaded report when filters are cleared", () => {
    const reports = [report(), report({ evaluation_id: "eval-002", outcome: "APPROVE" })];
    expect(filterDecisionReports(reports, filters({ outcome: "REJECT" }))).toHaveLength(0);
    expect(filterDecisionReports(reports, EMPTY_DECISION_FILTERS)).toEqual(reports);
  });

  it("safely handles reports with every optional investigation field absent", () => {
    const sparse = report({
      decision_domain: undefined,
      amount: undefined,
      risk_score: undefined,
      channel: undefined,
      selected_rule_id: undefined,
      policy_guard_reason: undefined,
      policy_evaluation_resolution: undefined,
    });

    expect(() => filterDecisionReports([sparse], filters({ search: "missing" }))).not.toThrow();
    expect(filterDecisionReports([sparse], filters({ search: "eval-001" }))).toEqual([sparse]);
    expect(filterDecisionReports([sparse], filters({ domainOrType: "domain:production" }))).toEqual([]);
  });
});
