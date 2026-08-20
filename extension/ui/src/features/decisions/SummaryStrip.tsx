import Card from "@mui/material/Card";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { ReactNode } from "react";

import type { DecisionSummary } from "../../services/BackendClient";

function Stat(props: { label: string; value: ReactNode; detail?: string }) {
  return (
    <Card variant="outlined" sx={{ px: 2, py: 1, minWidth: 120 }}>
      <Typography variant="h4" sx={{ fontVariantNumeric: "tabular-nums" }}>
        {props.value}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        {props.label}
      </Typography>
      {props.detail && (
        <Typography variant="caption" color="text.secondary" sx={{ display: "block" }}>
          {props.detail}
        </Typography>
      )}
    </Card>
  );
}

export function formatSummaryRate(rate: number | null | undefined): string {
  if (rate === null || rate === undefined || !Number.isFinite(rate)) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "percent",
    maximumFractionDigits: 1,
  }).format(rate);
}

function optionalCount(count: number | null | undefined): number | string {
  return count === null || count === undefined ? "—" : count;
}

/**
 * Counts straight from the published report summary — the labels reuse the
 * summary's own field vocabulary (would approve / block / escalate, review
 * required), never repo-local synonyms.
 */
export function SummaryStrip(props: { summary: DecisionSummary }) {
  const { summary } = props;
  return (
    <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
      <Stat label="Evaluations" value={summary.total_evaluations} />
      <Stat label="Would approve" value={summary.would_approve_count} />
      <Stat label="Would block" value={summary.would_block_count} />
      <Stat label="Would escalate" value={summary.would_escalate_count} />
      <Stat label="Review required" value={summary.review_required_count} />
      <Stat
        label="Policy mismatches"
        value={optionalCount(summary.policy_mismatch_count)}
        detail={`${formatSummaryRate(summary.policy_mismatch_rate)} of evaluations`}
      />
      <Stat
        label="Near misses"
        value={optionalCount(summary.near_miss_count)}
        detail={`${formatSummaryRate(summary.near_miss_rate)} of evaluations`}
      />
    </Stack>
  );
}
