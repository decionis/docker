import { describe, expect, it } from "vitest";

import { summarizeDemoResults } from "../src/features/demo/DemoPolicyResults";
import type { DemoEvaluationResult } from "../src/services/BackendClient";

let sequence = 0;

function result(outcome: string): DemoEvaluationResult {
  sequence += 1;
  return {
    scenario_id: `scenario-${sequence}`,
    label: "test",
    lane: "APPROVE",
    outcome,
    mode: "ENFORCEMENT",
    policy_version: "docker-desktop-starter-v1",
    evaluation_id: `evaluation-${sequence}`,
    dossier_id: `dossier-${sequence}`,
    confidence: 1,
  };
}

describe("summarizeDemoResults", () => {
  it("counts each returned outcome in exactly one published-vocabulary bucket", () => {
    const results = [
      result("APPROVE"),
      result("APPROVE"),
      result("APPROVE"),
      result("REJECT"),
      result("REJECT"),
      result("REJECT"),
      result("ESCALATE"),
      result("ESCALATE"),
      result("ESCALATE"),
      result("REVIEW"),
    ];
    const counts = summarizeDemoResults(results);

    expect(counts).toEqual({ approve: 3, reject: 3, escalate: 3, review: 1 });
    expect(counts.approve + counts.reject + counts.escalate + counts.review).toBe(results.length);
  });

  it("does not treat an intended lane as a completed result", () => {
    expect(summarizeDemoResults([])).toEqual({ approve: 0, reject: 0, escalate: 0, review: 0 });
  });

  it("leaves an unknown outcome from a newer plane uncounted rather than guessed", () => {
    expect(summarizeDemoResults([result("DEFER")])).toEqual({
      approve: 0,
      reject: 0,
      escalate: 0,
      review: 0,
    });
  });
});
