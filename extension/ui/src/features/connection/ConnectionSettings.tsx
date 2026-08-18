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
 * The org API key is passed straight to the daemon and cleared from component
 * state; it is never persisted UI-side (rules/security.rules.md Rule 2.3).
 */
export function ConnectionSettings(props: {
  open: boolean;
  backend: BackendClient;
  status: DaemonStatus | null;
  onClose: () => void;
  onChanged: (status: DaemonStatus) => void;
}) {
  const [orgId, setOrgId] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const connected = Boolean(props.status?.connected);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const next = await props.backend.connect({
        org_id: orgId.trim(),
        api_key: apiKey,
        ...(baseUrl.trim() ? { base_url: baseUrl.trim() } : {}),
      });
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
          <Typography variant="body2" color="text.secondary">
            The org API key is held by the extension backend only — it never leaves Docker Desktop
            except toward the Decionis control plane, and it is not stored in this UI.
          </Typography>
          {connected && (
            <Alert severity="success">
              Connected to org {props.status?.org_id} at {props.status?.base_url}.
            </Alert>
          )}
          {error && <Alert severity="error">{error}</Alert>}
          <TextField
            label="Organization ID"
            placeholder="00000000-0000-0000-0000-000000000000"
            value={orgId}
            onChange={(event) => setOrgId(event.target.value)}
            fullWidth
          />
          <TextField
            label="Org API key"
            type="password"
            value={apiKey}
            onChange={(event) => setApiKey(event.target.value)}
            autoComplete="off"
            fullWidth
          />
          <TextField
            label="API base URL (optional)"
            placeholder="https://api.decionis.com"
            value={baseUrl}
            onChange={(event) => setBaseUrl(event.target.value)}
            fullWidth
          />
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
        <Button
          variant="contained"
          onClick={() => void submit()}
          disabled={busy || orgId.trim() === "" || apiKey === ""}
        >
          Connect
        </Button>
      </DialogActions>
    </Dialog>
  );
}
