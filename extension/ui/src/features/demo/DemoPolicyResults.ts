import type { DemoEvaluationResult } from "../../services/BackendClient";

export interface DemoCounts {
  approve: number;
  block: number;
  escalate: number;
}

/** Counts only exact control-plane answers; intended lanes never count as results. */
export function summarizeDemoResults(results: DemoEvaluationResult[]): DemoCounts {
  return results.reduce<DemoCounts>(
    (counts, result) => {
      if (result.outcome === "APPROVE") counts.approve += 1;
      if (result.outcome === "REJECT" || result.execution_action === "BLOCK") counts.block += 1;
      if (result.outcome === "ESCALATE" || result.outcome === "REVIEW") counts.escalate += 1;
      return counts;
    },
    { approve: 0, block: 0, escalate: 0 },
  );
}
