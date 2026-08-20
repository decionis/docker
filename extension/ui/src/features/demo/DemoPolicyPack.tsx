import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import LinearProgress from "@mui/material/LinearProgress";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useEffect, useMemo, useState } from "react";

import { presentOutcome } from "../../protocol/VerdictLabels";
import {
  BackendClient,
  BackendError,
  type DemoEvaluationResult,
  type DemoScenario,
} from "../../services/BackendClient";
import { summarizeDemoResults, type DemoCounts } from "./DemoPolicyResults";

const LANES: Array<{ key: DemoScenario["lane"]; label: string }> = [
  { key: "APPROVE", label: "Benign reads" },
  { key: "REJECT", label: "Destructive actions" },
  { key: "ESCALATE", label: "High-value changes" },
];

/** Summary chips per published outcome; REVIEW appears only once it happens. */
function outcomeChipEntries(counts: DemoCounts): Array<{ outcome: string; count: number }> {
  return [
    { outcome: "APPROVE", count: counts.approve },
    { outcome: "REJECT", count: counts.reject },
    { outcome: "ESCALATE", count: counts.escalate },
    { outcome: "REVIEW", count: counts.review },
  ].filter((entry) => entry.outcome !== "REVIEW" || entry.count > 0);
}

/**
 * A bounded live test of the installed starter policy. The daemon owns every
 * action descriptor; this UI sends only a scenario id and displays the exact
 * returned verdict. Nothing in this component executes the proposed action.
 */
export function DemoPolicyPack(props: {
  backend: BackendClient;
  enforcementEnabled: boolean;
  onCompleted: () => void | Promise<void>;
}) {
  const [scenarios, setScenarios] = useState<DemoScenario[]>([]);
  const [results, setResults] = useState<DemoEvaluationResult[]>([]);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    props.backend
      .demoScenarios()
      .then((payload) => setScenarios(payload.scenarios))
      .catch(() => setScenarios([]));
  }, [props.backend]);

  const byID = useMemo(() => new Map(results.map((result) => [result.scenario_id, result])), [results]);
  const counts = summarizeDemoResults(results);

  const run = async () => {
    if (running || !props.enforcementEnabled || scenarios.length === 0) return;
    setRunning(true);
    setResults([]);
    setError(null);
    try {
      const completed: DemoEvaluationResult[] = [];
      for (const scenario of scenarios) {
        const result = await props.backend.evaluateDemoScenario(scenario.id);
        completed.push(result);
        setResults([...completed]);
      }
      await props.onCompleted();
    } catch (raw) {
      setError(
        raw instanceof BackendError ? raw.message : "The Decionis daemon is not reachable.",
      );
    } finally {
      setRunning(false);
    }
  };

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack spacing={1.5}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" spacing={2}>
          <Box>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography variant="subtitle1">Docker policy pack</Typography>
              {scenarios.length > 0 && (
                <Chip size="small" variant="outlined" label={`${scenarios.length} live checks`} />
              )}
            </Stack>
            <Typography variant="caption" color="text.secondary">
              Evaluates fixed action proposals in this workspace. No command, deployment, database,
              registry, or secret action is executed.
            </Typography>
          </Box>
          <Button
            variant="contained"
            onClick={() => void run()}
            disabled={running || !props.enforcementEnabled || scenarios.length === 0}
          >
            {running
              ? `Evaluating ${results.length + 1} of ${scenarios.length}`
              : !props.enforcementEnabled
                ? "Enable enforcement to run"
                : scenarios.length > 0
                  ? `Run ${scenarios.length} policy checks`
                  : "Policy checks unavailable"}
          </Button>
        </Stack>

        {running && <LinearProgress variant="determinate" value={(results.length / scenarios.length) * 100} />}

        <Stack direction="row" spacing={1}>
          {outcomeChipEntries(counts).map((entry) => {
            const presentation = presentOutcome(entry.outcome);
            return (
              <Chip
                key={entry.outcome}
                size="small"
                color={presentation.tone}
                label={`${presentation.label} ${entry.count}`}
              />
            );
          })}
          {results.length > 0 && (
            <Typography variant="caption" color="text.secondary" sx={{ alignSelf: "center" }}>
              {results.length}/{scenarios.length} evaluated by the control plane
            </Typography>
          )}
        </Stack>

        <Stack direction={{ xs: "column", md: "row" }} spacing={1.5} alignItems="stretch">
          {LANES.map((lane) => (
            <Paper key={lane.key} variant="outlined" sx={{ p: 1.25, flex: 1, minWidth: 0 }}>
              <Typography variant="subtitle2" sx={{ mb: 0.75 }}>
                {lane.label}
              </Typography>
              <Stack spacing={0.65}>
                {scenarios
                  .filter((scenario) => scenario.lane === lane.key)
                  .map((scenario) => {
                    const result = byID.get(scenario.id);
                    return (
                      <Stack
                        key={scenario.id}
                        direction="row"
                        spacing={0.75}
                        justifyContent="space-between"
                        alignItems="center"
                      >
                        <Typography variant="caption" noWrap title={scenario.description}>
                          {scenario.label}
                        </Typography>
                        <Chip
                          size="small"
                          color={result ? presentOutcome(result.outcome).tone : "default"}
                          variant={result ? "filled" : "outlined"}
                          label={result ? presentOutcome(result.outcome).label : "READY"}
                        />
                      </Stack>
                    );
                  })}
              </Stack>
            </Paper>
          ))}
        </Stack>

        {results.length === scenarios.length && results.length > 0 && (
          <Alert severity="success">
            Live policy results:{" "}
            {outcomeChipEntries(counts)
              .map((entry) => `${entry.count} ${presentOutcome(entry.outcome).label}`)
              .join(", ")}
            . Decision reports and review queue refreshed below.
          </Alert>
        )}
        {error && <Alert severity="error">Policy checks stopped. {error}</Alert>}
      </Stack>
    </Paper>
  );
}
