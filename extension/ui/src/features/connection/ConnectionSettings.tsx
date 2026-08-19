import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Accordion from "@mui/material/Accordion";
import AccordionDetails from "@mui/material/AccordionDetails";
import AccordionSummary from "@mui/material/AccordionSummary";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useEffect, useRef, useState } from "react";

import { BackendClient, BackendError, type DaemonStatus } from "../../services/BackendClient";

const DEFAULT_API_HOST = "api.decionis.com";

/**
 * The host an account sign-in would actually reach. Shown next to the
 * password field so the destination is visible at the moment credentials
 * are typed: a custom base URL is a legitimate self-hosting feature, but it
 * must never quietly become somewhere else to send a password.
 */
function credentialDestination(baseUrl: string): string {
  const trimmed = baseUrl.trim();
  if (!trimmed) return DEFAULT_API_HOST;
  try {
    return new URL(/^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`).host;
  } catch {
    return trimmed; // unparseable: show exactly what was typed, claim nothing
  }
}

const BROWSER_POLL_MS = 2_000;
const BROWSER_WAIT_LIMIT_MS = 10 * 60_000; // matches the daemon's pending TTL

/**
 * Connect options, in the order most people need them. New users never open
 * this dialog at all — the extension provisions a workspace on first run.
 * Here: one click through the browser, a pasted single-use enrollment token,
 * or an account email + password that Decionis resolves into a workspace and
 * a freshly minted scoped key. No workspace UUID, no API key to copy.
 *
 * Nothing secret is persisted UI-side (rules/security.rules.md Rule 2.3):
 * the password lives in component state only until its request returns.
 */
export function ConnectionSettings(props: {
  open: boolean;
  backend: BackendClient;
  status: DaemonStatus | null;
  onClose: () => void;
  onChanged: (status: DaemonStatus) => void;
}) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [enrollmentToken, setEnrollmentToken] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [waitingBrowser, setWaitingBrowser] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<number | null>(null);

  const connected = Boolean(props.status?.connected);
  const usingToken = enrollmentToken.trim() !== "";
  const usingAccount = email.trim() !== "" && password !== "";
  const destinationHost = credentialDestination(baseUrl);
  const isCustomDestination = destinationHost !== DEFAULT_API_HOST;
  const canConnect = !busy && !waitingBrowser && (usingToken ? !usingAccount : usingAccount);

  const stopWaiting = () => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
    setWaitingBrowser(false);
  };

  // Never leave a poll running once the dialog is gone.
  useEffect(() => {
    if (!props.open) stopWaiting();
    return stopWaiting;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.open]);

  const continueInBrowser = async () => {
    setError(null);
    try {
      const started = await props.backend.connectStart(baseUrl);
      props.backend.openExternal(started.authorize_url);
      setWaitingBrowser(true);
      const deadline = Date.now() + BROWSER_WAIT_LIMIT_MS;
      pollRef.current = window.setInterval(() => {
        void (async () => {
          try {
            const next = await props.backend.status();
            if (next.connected) {
              stopWaiting();
              props.onChanged(next);
              props.onClose();
              return;
            }
          } catch {
            // daemon briefly unreachable — keep waiting
          }
          if (Date.now() > deadline) {
            stopWaiting();
            setError("The browser sign-in didn't finish. Try again, or paste an enrollment token.");
          }
        })();
      }, BROWSER_POLL_MS);
    } catch (raw) {
      setError(raw instanceof BackendError ? raw.message : "The Decionis daemon is not reachable.");
    }
  };

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const base = baseUrl.trim() ? { base_url: baseUrl.trim() } : {};
      const next = usingToken
        ? await props.backend.connect({ enrollment_token: enrollmentToken.trim(), ...base })
        : await props.backend.connect({ email: email.trim(), password, ...base });
      setEnrollmentToken("");
      // The password exists in this component only until the request returns.
      setPassword("");
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
            Sign in with your browser — your workspace is created on first sign-in and Docker
            Desktop connects itself. The credential is minted server-side and held only by the
            extension backend.
          </Typography>
          {waitingBrowser ? (
            <Stack direction="row" spacing={1.5} alignItems="center">
              <CircularProgress size={18} />
              <Typography variant="body2">Waiting for the browser sign-in to finish…</Typography>
              <Button size="small" onClick={stopWaiting}>
                Cancel
              </Button>
            </Stack>
          ) : (
            <Button variant="contained" onClick={() => void continueInBrowser()} disabled={busy}>
              Continue in browser
            </Button>
          )}

          <Typography variant="body2" color="text.secondary" sx={{ pt: 1 }}>
            Or paste a single-use <strong>enrollment token</strong> from your Decionis
            organization. It is exchanged once for a scoped credential that only the extension
            backend holds — nothing is stored in this UI.
          </Typography>
          <TextField
            label="Enrollment token"
            placeholder="dcn_enroll_…"
            value={enrollmentToken}
            onChange={(event) => setEnrollmentToken(event.target.value)}
            autoComplete="off"
            fullWidth
            disabled={usingAccount}
            inputProps={{ style: { fontFamily: "monospace" } }}
          />

          <Accordion
            variant="outlined"
            expanded={advancedOpen}
            onChange={(_event, expanded) => setAdvancedOpen(expanded)}
            disableGutters
          >
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="body2">Already have a Decionis account? Sign in here</Typography>
            </AccordionSummary>
            <AccordionDetails>
              <Stack spacing={2}>
                <Typography variant="body2" color="text.secondary">
                  Decionis finds your workspace and issues the extension its own
                  credential — there is no key to copy. Your password is used for
                  this one request and is never stored.
                </Typography>
                <Alert severity={isCustomDestination ? "warning" : "info"}>
                  {isCustomDestination
                    ? `Your password will be sent to ${destinationHost} — not to ${DEFAULT_API_HOST}. Only continue if you run Decionis there.`
                    : `Your password is sent only to ${destinationHost}.`}
                </Alert>
                <TextField
                  label="Email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="username"
                  fullWidth
                  disabled={usingToken}
                />
                <TextField
                  label="Password"
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete="current-password"
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
