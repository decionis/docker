import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import Chip from "@mui/material/Chip";
import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { REPORT_MODES, presentOutcome, type ReportMode } from "../../protocol/VerdictLabels";
import type { DecisionsPayload } from "../../services/BackendClient";

export function OutcomeChip(props: { outcome: string }) {
  const presentation = presentOutcome(props.outcome);
  return (
    <Chip
      size="small"
      label={presentation.label}
      color={presentation.tone === "info" ? "default" : presentation.tone}
      variant={presentation.tone === "info" ? "outlined" : "filled"}
    />
  );
}

export function DecisionFeed(props: {
  connected: boolean;
  decisions: DecisionsPayload | null;
  mode: ReportMode;
  onModeChange: (mode: ReportMode) => void;
  onInspect: (dossierId: string) => void;
  onConnect: () => void;
}) {
  if (!props.connected) {
    return (
      <Card variant="outlined" sx={{ p: 4, textAlign: "center" }}>
        <Typography variant="h6" gutterBottom>
          Connect your Decionis org
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Live decision evaluations, verdicts, and signed Decision Dossiers appear here once your
          org is connected — paste a single-use enrollment token and the backend takes care of the
          credentials. No API keys touch this UI.
        </Typography>
        <Button variant="contained" onClick={props.onConnect}>
          Connect
        </Button>
      </Card>
    );
  }

  const reports = props.decisions?.response.reports ?? [];

  return (
    <Card variant="outlined">
      <Stack direction="row" alignItems="center" spacing={2} sx={{ p: 2 }}>
        <Typography variant="h6" sx={{ flex: 1 }}>
          Decision evaluations
        </Typography>
        <TextField
          select
          size="small"
          label="Mode"
          value={props.mode}
          onChange={(event) => props.onModeChange(event.target.value as ReportMode)}
          sx={{ width: 180 }}
        >
          {REPORT_MODES.map((mode) => (
            <MenuItem key={mode} value={mode}>
              {mode}
            </MenuItem>
          ))}
        </TextField>
      </Stack>

      {reports.length === 0 ? (
        <Box sx={{ p: 4, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            No {props.mode} decision evaluations reported for this org yet.
          </Typography>
        </Box>
      ) : (
        <TableContainer sx={{ maxHeight: 480 }}>
          <Table stickyHeader size="small">
            <TableHead>
              <TableRow>
                <TableCell>Time</TableCell>
                <TableCell>Decision type</TableCell>
                <TableCell>Outcome</TableCell>
                <TableCell>Execution</TableCell>
                <TableCell>Policy version</TableCell>
                <TableCell>Reason</TableCell>
                <TableCell align="right">Dossier</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {reports.map((report) => (
                <TableRow key={report.evaluation_id} hover>
                  <TableCell sx={{ whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>
                    {new Date(report.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>{report.decision_type}</TableCell>
                  <TableCell>
                    <OutcomeChip outcome={report.outcome} />
                  </TableCell>
                  <TableCell>{report.execution_action}</TableCell>
                  <TableCell>{report.policy_version}</TableCell>
                  <TableCell sx={{ maxWidth: 320 }}>
                    <Typography variant="body2" noWrap title={report.reason}>
                      {report.reason}
                    </Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Button size="small" onClick={() => props.onInspect(report.dossier_id)}>
                      Inspect
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Card>
  );
}
