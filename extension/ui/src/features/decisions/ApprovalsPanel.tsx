import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";

import type { PendingApproval } from "../../services/BackendClient";

export function inspectApprovalDossier(
  approval: PendingApproval,
  onInspect: (dossierId: string) => void,
): void {
  if (approval.dossier_id) onInspect(approval.dossier_id);
}

/**
 * What is waiting on a person.
 *
 * Three states are deliberately distinct: entries waiting, nothing waiting,
 * and "we could not read the queue". Collapsing the last two would let an
 * outage read as an all-clear, which in a governance surface is the one
 * mistake that matters.
 */
export function ApprovalsPanel(props: {
  approvals: PendingApproval[] | null;
  error: string | null;
  onInspect: (dossierId: string) => void;
}) {
  if (props.error) {
    return (
      <Alert severity="warning">
        Pending approvals could not be read, so this list may be incomplete. {props.error}
      </Alert>
    );
  }
  if (props.approvals === null) return null;

  if (props.approvals.length === 0) {
    return (
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Typography variant="subtitle2">Awaiting review</Typography>
        <Typography variant="body2" color="text.secondary">
          Nothing is waiting on a person right now.
        </Typography>
      </Paper>
    );
  }

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
        <Typography variant="subtitle2">Awaiting review</Typography>
        <Chip size="small" label={props.approvals.length} color="warning" />
      </Stack>
      <Stack spacing={1.25}>
        {props.approvals.map((approval) => (
          <Box key={approval.evaluation_id}>
            <Stack direction="row" spacing={1} alignItems="center">
              {/* The protocol's own outcome word, never re-labelled here. */}
              <Chip
                size="small"
                label={approval.outcome}
                color={approval.outcome === "ESCALATE" ? "error" : "warning"}
                variant={approval.outcome === "ESCALATE" ? "filled" : "outlined"}
              />
              <Typography variant="body2">{approval.decision_type || "—"}</Typography>
              {approval.override_status && (
                <Chip size="small" variant="outlined" label={`override ${approval.override_status}`} />
              )}
              <Box sx={{ flex: 1 }} />
              {approval.dossier_id && (
                <Button size="small" onClick={() => inspectApprovalDossier(approval, props.onInspect)}>
                  Inspect dossier
                </Button>
              )}
            </Stack>
            <Typography variant="caption" color="text.secondary">
              {[
                approval.channel,
                approval.amount ? `amount ${approval.amount}` : null,
                approval.policy_version ? `policy ${approval.policy_version}` : null,
                approval.created_at,
              ]
                .filter(Boolean)
                .join(" · ")}
            </Typography>
          </Box>
        ))}
      </Stack>
    </Paper>
  );
}
