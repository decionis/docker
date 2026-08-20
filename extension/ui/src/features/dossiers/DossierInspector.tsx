import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Divider from "@mui/material/Divider";
import Drawer from "@mui/material/Drawer";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useEffect, useState } from "react";

import { BackendClient, BackendError, type DossierPayload } from "../../services/BackendClient";
import {
  copyDossierExport,
  downloadDossierExport,
  type DossierExportFeedback,
} from "./DossierExport";

function checkColor(severity: "pass" | "warn" | "fail"): "success" | "warning" | "error" {
  if (severity === "pass") return "success";
  if (severity === "warn") return "warning";
  return "error";
}

/**
 * Verification status renders strictly from the daemon's offline Ed25519
 * result: "Ed25519 verified" appears only when every check passed — anything
 * else is explicitly unverified (rules/security.rules.md Rule 0.3).
 */
export function DossierInspector(props: {
  backend: BackendClient;
  dossierId: string | null;
  onClose: () => void;
}) {
  const [payload, setPayload] = useState<DossierPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [exportFeedback, setExportFeedback] = useState<DossierExportFeedback | null>(null);

  useEffect(() => {
    if (!props.dossierId) {
      setPayload(null);
      setError(null);
      setExportFeedback(null);
      return;
    }
    let cancelled = false;
    setPayload(null);
    setLoading(true);
    setError(null);
    setExportFeedback(null);
    props.backend
      .dossier(props.dossierId)
      .then((next) => {
        if (!cancelled) setPayload(next);
      })
      .catch((raw) => {
        if (!cancelled) {
          setPayload(null);
          setError(raw instanceof BackendError ? raw.message : "The dossier could not be fetched.");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [props.backend, props.dossierId]);

  const verification = payload?.verification;

  async function handleCopy() {
    if (!payload) return;
    const feedback = await copyDossierExport(payload, async (json) => {
      if (!navigator.clipboard?.writeText) throw new Error("Clipboard unavailable");
      await navigator.clipboard.writeText(json);
    });
    setExportFeedback(feedback);
  }

  function handleDownload() {
    if (!payload) return;
    const feedback = downloadDossierExport(payload, (exported) => {
      const url = URL.createObjectURL(new Blob([exported.json], { type: "application/json" }));
      const anchor = document.createElement("a");
      try {
        anchor.href = url;
        anchor.download = exported.filename;
        anchor.style.display = "none";
        document.body.appendChild(anchor);
        anchor.click();
      } finally {
        anchor.remove();
        URL.revokeObjectURL(url);
      }
    });
    setExportFeedback(feedback);
  }

  return (
    <Drawer anchor="right" open={props.dossierId !== null} onClose={props.onClose}>
      <Box sx={{ width: 520, maxWidth: "90vw", p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Decision Dossier
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ wordBreak: "break-all", mb: 2 }}>
          {props.dossierId}
        </Typography>

        {loading && <CircularProgress size={24} />}
        {error && <Alert severity="error">{error}</Alert>}
        {payload && (
          <Stack direction="row" spacing={1} sx={{ mb: 2 }}>
            <Button size="small" variant="outlined" onClick={() => void handleCopy()}>
              Copy JSON
            </Button>
            <Button size="small" variant="outlined" onClick={handleDownload}>
              Download JSON
            </Button>
          </Stack>
        )}
        {exportFeedback && (
          <Alert severity={exportFeedback.severity} aria-live="polite" sx={{ mb: 2 }}>
            {exportFeedback.message}
          </Alert>
        )}

        {verification && (
          <Stack spacing={2}>
            <Stack direction="row" spacing={1} alignItems="center">
              {verification.verified ? (
                <Chip color="success" label="Ed25519 verified" />
              ) : (
                <Chip color="error" label="Unverified" />
              )}
              {verification.key_id && (
                <Typography variant="caption" color="text.secondary">
                  key {verification.key_id}
                </Typography>
              )}
            </Stack>
            {!verification.available && (
              <Alert severity="warning">
                No Ed25519 proof bundle was available for verification.
              </Alert>
            )}

            <Divider />
            <Typography variant="subtitle2">Verification checks</Typography>
            <List dense disablePadding>
              {verification.checks.map((check) => (
                <ListItem key={check.key} disableGutters>
                  <Chip
                    size="small"
                    sx={{ mr: 1, minWidth: 56 }}
                    color={checkColor(check.severity)}
                    label={check.severity}
                  />
                  <ListItemText primary={check.label} secondary={check.detail} />
                </ListItem>
              ))}
            </List>

            {payload && (
              <>
                <Divider />
                <Typography variant="subtitle2">Reproducibility</Typography>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Chip
                    size="small"
                    color={payload.reproducibility.posture === "reproduction_ready" ? "success" : "default"}
                    label={payload.reproducibility.posture}
                  />
                </Stack>
                <Typography variant="body2" color="text.secondary">
                  {payload.reproducibility.detail}
                </Typography>

                <Divider />
                <Typography variant="subtitle2">Payload</Typography>
                <Box
                  component="pre"
                  sx={{
                    fontSize: 12,
                    p: 1.5,
                    borderRadius: 1,
                    bgcolor: "action.hover",
                    overflow: "auto",
                    maxHeight: 320,
                  }}
                >
                  {JSON.stringify(payload.payload, null, 2)}
                </Box>
              </>
            )}
          </Stack>
        )}
      </Box>
    </Drawer>
  );
}
