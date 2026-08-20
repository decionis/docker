import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { SummaryStrip, formatSummaryRate } from "../src/features/decisions/SummaryStrip";
import type { DecisionSummary } from "../src/services/BackendClient";

function summary(overrides: Partial<DecisionSummary> = {}): DecisionSummary {
  return {
    total_evaluations: 8,
    would_approve_count: 4,
    would_block_count: 2,
    would_escalate_count: 1,
    review_required_count: 1,
    non_approve_count: 4,
    non_approve_rate: 0.5,
    outcome_counts: {},
    ...overrides,
  };
}

describe("decision summary drift visibility", () => {
  it("renders backend mismatch and near-miss counts and rates as percentages", () => {
    const html = renderToStaticMarkup(
      createElement(SummaryStrip, {
        summary: summary({
          policy_mismatch_count: 2,
          policy_mismatch_rate: 0.25,
          near_miss_count: 1,
          near_miss_rate: 0.125,
        }),
      }),
    );

    expect(html).toContain("Policy mismatches");
    expect(html).toContain("25% of evaluations");
    expect(html).toContain("Near misses");
    expect(html).toContain("12.5% of evaluations");
  });

  it("distinguishes a returned zero rate from an absent rate", () => {
    expect(formatSummaryRate(0)).toBe("0%");
    expect(formatSummaryRate(undefined)).toBe("—");

    const html = renderToStaticMarkup(
      createElement(SummaryStrip, {
        summary: summary({ policy_mismatch_count: 0, policy_mismatch_rate: 0 }),
      }),
    );
    expect(html).toContain("0% of evaluations");
    expect(html).toContain("— of evaluations");
  });
});
