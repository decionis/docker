import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Accordion from "@mui/material/Accordion";
import AccordionDetails from "@mui/material/AccordionDetails";
import AccordionSummary from "@mui/material/AccordionSummary";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useState } from "react";

import { BackendClient, BackendError, type DaemonStatus } from "../../services/BackendClient";

/**
 * First-run connect: one pasted single-use enrollment token (the same
 * self-provisioning mechanism Decionis connectors use — exchanged once for a
 * scoped key the backend holds). Manual org ID + API key stays available
 * under Advanced. Nothing secret is persisted UI-side
 * (rules/security.rules.md Rule 2.3).
 */
export function ConnectionSettings(props: {
  open: boolean;
  backend: BackendClient;
  status: DaemonStatus | null;
  onClose: () => void;
  onChanged: (status: DaemonStatus) => void;
}) {
  const [enrollmentToken, setEnrollmentToken] = useState("");
  const [orgId, setOrgId] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const connected = Boolean(props.status?.connected);
  const usingToken = enrollmentToken.trim() !== "";
  const usingManual = orgId.trim() !== "" && apiKey !== "";
  const canConnect = !busy && (usingToken ? !usingManual : usingManual);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const base = baseUrl.trim() ? { base_url: baseUrl.trim() } : {};
      const next = usingToken
        ? await props.backend.connect({ enrollment_token: enrollmentToken.trim(), ...base })
        : await props.backend.connect({ org_id: orgId.trim(), api_key: apiKey, ...base });
      setEnrollmentToken("");
      setApiKey("");
      props.onChanged(next);
      props.onClose();
    } catch (raw) {
      setError(raw instanceof BackendError ? raw.message : "The Decionis daemon is not reachable.");
    } finally {
      setBusy(false);
    }
  };

  const disconnect = async () => {
    setBusy(true);
    setError(null);
    try {
      await props.backend.disconnect();
      const next = await props.backend.status();
      props.onChanged(next);
      props.onClose();
    } catch (raw) {
      setError(raw instanceof BackendError ? raw.message : "The Decionis daemon is not reachable.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={props.open} onClose={props.onClose} fullWidth maxWidth="sm">
      <DialogTitle>Connect to Decionis</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {connected && (
            <Alert severity="success">
              Connected to org {props.status?.org_id} at {props.status?.base_url}.
            </Alert>
          )}
          {error && <Alert severity="error">{error}</Alert>}

          <Typography variant="body2" color="text.secondary">
            Paste a single-use <strong>enrollment token</strong> from your Decionis organization.
            It is exchanged once for a scoped credential that only the extension backend holds —
            nothing is stored in this UI.
          </Typography>
          <TextField
            label="Enrollment token"
            placeholder="dcn_enroll_…"
            value={enrollmentToken}
            onChange={(event) => setEnrollmentToken(event.target.value)}
            autoComplete="off"
            fullWidth
            disabled={usingManual}
            inputProps={{ style: { fontFamily: "monospace" } }}
          />

          <Accordion
            variant="outlined"
            expanded={advancedOpen}
            onChange={(_event, expanded) => setAdvancedOpen(expanded)}
            disableGutters
          >
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="body2">Advanced: org ID and API key, or a custom API base URL</Typography>
            </AccordionSummary>
            <AccordionDetails>
              <Stack spacing={2}>
                <TextField
                  label="Organization ID"
                  placeholder="00000000-0000-0000-0000-000000000000"
                  value={orgId}
                  onChange={(event) => setOrgId(event.target.value)}
                  fullWidth
                  disabled={usingToken}
                />
                <TextField
                  label="Org API key"
                  type="password"
                  value={apiKey}
                  onChange={(event) => setApiKey(event.target.value)}
                  autoComplete="off"
                  fullWidth
                  disabled={usingToken}
                />
                <TextField
                  label="API base URL (optional)"
                  placeholder="https://api.decionis.com"
                  value={baseUrl}
                  onChange={(event) => setBaseUrl(event.target.value)}
                  fullWidth
                />
              </Stack>
            </AccordionDetails>
          </Accordion>
        </Stack>
      </DialogContent>
      <DialogActions>
        {connected && (
          <Button color="error" onClick={() => void disconnect()} disabled={busy}>
            Disconnect
          </Button>
        )}
        <Button onClick={props.onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => void submit()} disabled={!canConnect}>
          Connect
        </Button>
      </DialogActions>
    </Dialog>
  );
}
