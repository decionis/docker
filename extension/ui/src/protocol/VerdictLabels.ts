/**
 * The single verdict-vocabulary module (rules/discovery.rules.md Rule 2.6):
 * every rendered outcome uses the protocol's published enum values verbatim —
 * this module adds only presentation tone and the protocol's own execution
 * phrasing ("continue, stop, hand off, or review", from the reports endpoint
 * description). test/VerdictLabels.test.ts is the drift gate against the
 * enums recorded from the published OpenAPI contract.
 */

export const PROTOCOL_OUTCOMES = ["APPROVE", "ESCALATE", "REJECT", "REVIEW"] as const;
export const EXECUTION_ACTIONS = ["CONTINUE", "STOP", "HAND_OFF", "REVIEW"] as const;
export const REPORT_MODES = ["SHADOW", "PARALLEL", "ENFORCEMENT"] as const;

export type ProtocolOutcome = (typeof PROTOCOL_OUTCOMES)[number];
export type ExecutionAction = (typeof EXECUTION_ACTIONS)[number];
export type ReportMode = (typeof REPORT_MODES)[number];

export type OutcomeTone = "success" | "error" | "warning" | "info";

export interface OutcomePresentation {
  /** Rendered verbatim — never a repo-local synonym. */
  label: ProtocolOutcome;
  tone: OutcomeTone;
  /** The protocol's own execution phrasing for this outcome. */
  execution: string;
}

const OUTCOME_PRESENTATIONS: Record<ProtocolOutcome, OutcomePresentation> = {
  APPROVE: { label: "APPROVE", tone: "success", execution: "continue" },
  REJECT: { label: "REJECT", tone: "error", execution: "stop" },
  ESCALATE: { label: "ESCALATE", tone: "warning", execution: "hand off" },
  REVIEW: { label: "REVIEW", tone: "info", execution: "review" },
};

export function isProtocolOutcome(value: string): value is ProtocolOutcome {
  return (PROTOCOL_OUTCOMES as readonly string[]).includes(value);
}

/**
 * Presentation for an outcome. Unknown values (a newer control plane than
 * this build) render verbatim with a neutral tone rather than being guessed.
 */
export function presentOutcome(outcome: string): { label: string; tone: OutcomeTone; execution: string } {
  if (isProtocolOutcome(outcome)) return OUTCOME_PRESENTATIONS[outcome];
  return { label: outcome, tone: "info", execution: "" };
}

export function presentExecutionAction(action: string): string {
  // Rendered verbatim; the enum test pins the known set.
  return action;
}
