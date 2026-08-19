import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import LinearProgress from "@mui/material/LinearProgress";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useState } from "react";

import { BackendClient, BackendError, type WorkspaceState } from "../../services/BackendClient";

/**
 * The enforcement switch and what it costs.
 *
 * Shadow is the default and stays free: Decionis watches and records, and
 * gates nothing. Turning enforcement on is the moment the product starts
 * refusing real actions, so it is a deliberate click — and the free
 * allowance it draws on is shown next to it rather than discovered when it
 * runs out.
 */
export function EnforcementControl(props: {
  backend: BackendClient;
  workspace: WorkspaceState;
  onChanged: (next: WorkspaceState) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { workspace } = props;
  const limit = workspace.governed_limit;
  const metered = limit !== null;
  const used = workspace.governed_used;

  const toggle = async (enabled: boolean) => {
    setBusy(true);
    setError(null);
    try {
      props.onChanged(await props.backend.setEnforcement(enabled));
    } catch (raw) {
      if (raw instanceof BackendError && raw.code === "governed_limit_reached") {
        setError(raw.message);
        // Re-read so the allowance shown matches what the plane just said.
        props.backend.workspace().then(props.onChanged).catch(() => {});
      } else {
        setError(
          raw instanceof BackendError ? raw.message : "The Decionis daemon is not reachable.",
        );
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Stack spacing={1.5}>
      <FormControlLabel
        control={
          <Checkbox
            checked={workspace.enforcement_enabled}
            disabled={busy || (!workspace.enforcement_available && !workspace.enforcement_enabled)}
            onChange={(event) => void toggle(event.target.checked)}
          />
        }
        label={
          <Box>
            <Typography variant="body2">Enforce decisions</Typography>
            <Typography variant="caption" color="text.secondary">
              {workspace.enforcement_enabled
                ? "Decionis is gating governed actions in this workspace."
                : "Shadow mode: decisions are recorded and nothing is gated."}
            </Typography>
          </Box>
        }
      />

      {workspace.enforcement_reverted && (
        <Alert severity="warning">
          Enforcement was turned off because this workspace used its {limit} free governed
          decisions. Decisions are still being recorded, but nothing is being gated.
        </Alert>
      )}

      {metered && (workspace.warn || workspace.at_cap) && (
        <Alert
          severity={workspace.at_cap ? "warning" : "info"}
          action={
            <Button
              size="small"
              variant={workspace.at_cap ? "contained" : "text"}
              onClick={() => props.backend.openExternal(workspace.subscribe_url)}
            >
              Subscribe
            </Button>
          }
        >
          {workspace.at_cap
            ? `This workspace has used all ${limit} free governed decisions.`
            : `${used} of ${limit} free governed decisions used.`}
        </Alert>
      )}

      {metered && (
        <Box>
          <LinearProgress
            variant="determinate"
            value={Math.min(100, (used / (limit || 1)) * 100)}
            color={workspace.at_cap ? "warning" : "primary"}
          />
          <Typography variant="caption" color="text.secondary">
            {used} of {limit} free governed decisions · shadow evaluations are unlimited
          </Typography>
        </Box>
      )}

      {error && <Alert severity="error">{error}</Alert>}
    </Stack>
  );
}
