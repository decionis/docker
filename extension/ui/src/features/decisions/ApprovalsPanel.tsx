import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Chip from "@mui/material/Chip";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";

import type { PendingApproval } from "../../services/BackendClient";

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
          <Box key={approval.id}>
            <Stack direction="row" spacing={1} alignItems="center">
              {/* Severity is the control plane's word, shown verbatim. */}
              <Chip
                size="small"
                label={approval.severity || "unspecified"}
                color={approval.severity === "urgent" ? "error" : "default"}
                variant={approval.severity === "urgent" ? "filled" : "outlined"}
              />
              <Typography variant="body2">{approval.decision_domain || "—"}</Typography>
            </Stack>
            <Typography variant="caption" color="text.secondary">
              {approval.trigger_reason || "No reason given."}
              {approval.surfaced_at ? ` · surfaced ${approval.surfaced_at}` : ""}
            </Typography>
          </Box>
        ))}
      </Stack>
    </Paper>
  );
}
