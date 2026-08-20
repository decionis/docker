import type { DemoEvaluationResult } from "../../services/BackendClient";

export interface DemoCounts {
  approve: number;
  reject: number;
  escalate: number;
  review: number;
}

/**
 * Counts only exact control-plane outcomes, one bucket per published enum
 * value (APPROVE / REJECT / ESCALATE / REVIEW); intended lanes never count as
 * results, and no result is ever counted twice.
 */
export function summarizeDemoResults(results: DemoEvaluationResult[]): DemoCounts {
  const counts: DemoCounts = { approve: 0, reject: 0, escalate: 0, review: 0 };
  for (const result of results) {
    if (result.outcome === "APPROVE") counts.approve += 1;
    if (result.outcome === "REJECT") counts.reject += 1;
    if (result.outcome === "ESCALATE") counts.escalate += 1;
    if (result.outcome === "REVIEW") counts.review += 1;
  }
  return counts;
}
