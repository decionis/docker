import { describe, expect, it } from "vitest";

import { summarizeDemoResults } from "../src/features/demo/DemoPolicyResults";
import type { DemoEvaluationResult } from "../src/services/BackendClient";

function result(outcome: string, executionAction = ""): DemoEvaluationResult {
  return {
    scenario_id: outcome + executionAction,
    label: "test",
    lane: "APPROVE",
    outcome,
    execution_action: executionAction,
    mode: "ENFORCEMENT",
    policy_version: "docker-desktop-starter-v1",
    evaluation_id: "evaluation",
    dossier_id: "dossier",
    would_execute: false,
    confidence: 1,
  };
}

describe("summarizeDemoResults", () => {
  it("counts only exact returned outcomes and block actions", () => {
    expect(
      summarizeDemoResults([
        result("APPROVE"),
        result("APPROVE"),
        result("APPROVE"),
        result("REJECT", "BLOCK"),
        result("REJECT", "BLOCK"),
        result("REJECT", "BLOCK"),
        result("ESCALATE"),
        result("ESCALATE"),
        result("ESCALATE"),
        result("ESCALATE"),
      ]),
    ).toEqual({ approve: 3, block: 3, escalate: 4 });
  });

  it("does not treat an intended lane as a completed result", () => {
    expect(summarizeDemoResults([])).toEqual({ approve: 0, block: 0, escalate: 0 });
  });
});
