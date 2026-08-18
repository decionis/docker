import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ConnectionSettings } from "./features/connection/ConnectionSettings";
import { StatusHeader } from "./features/connection/StatusHeader";
import { DecisionFeed } from "./features/decisions/DecisionFeed";
import { SummaryStrip } from "./features/decisions/SummaryStrip";
import { DossierInspector } from "./features/dossiers/DossierInspector";
import type { ReportMode } from "./protocol/VerdictLabels";
import { BackendClient, BackendError, type DaemonStatus, type DecisionsPayload } from "./services/BackendClient";

const POLL_INTERVAL_MS = 10_000;

export function App() {
  const backend = useMemo(() => new BackendClient(), []);
  const [status, setStatus] = useState<DaemonStatus | null>(null);
  const [decisions, setDecisions] = useState<DecisionsPayload | null>(null);
  const [mode, setMode] = useState<ReportMode>("ENFORCEMENT");
  const [feedError, setFeedError] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [inspectedDossierId, setInspectedDossierId] = useState<string | null>(null);
  const pollRef = useRef<number | null>(null);

  const refreshStatus = useCallback(async () => {
    try {
      setStatus(await backend.status());
    } catch {
      setStatus(null);
    }
  }, [backend]);

  const refreshDecisions = useCallback(async () => {
    try {
      const payload = await backend.decisions(mode, 100);
      setDecisions(payload);
      setFeedError(null);
    } catch (raw) {
      setDecisions(null);
      if (raw instanceof BackendError && raw.code === "not_connected") {
        setFeedError(null); // the disconnected state renders instead
      } else if (raw instanceof BackendError) {
        setFeedError(raw.message);
      } else {
        setFeedError("The Decionis daemon is not reachable.");
      }
    }
  }, [backend, mode]);

  useEffect(() => {
    void refreshStatus();
  }, [refreshStatus]);

  useEffect(() => {
    if (!status?.connected) {
      setDecisions(null);
      return;
    }
    void refreshDecisions();
    pollRef.current = window.setInterval(() => void refreshDecisions(), POLL_INTERVAL_MS);
    return () => {
      if (pollRef.current !== null) window.clearInterval(pollRef.current);
    };
  }, [status?.connected, refreshDecisions]);

  return (
    <Box sx={{ p: 3, maxWidth: 1200, mx: "auto" }}>
      <Stack spacing={2}>
        <StatusHeader
          status={status}
          onOpenSettings={() => setSettingsOpen(true)}
          onRefresh={() => {
            void refreshStatus();
            if (status?.connected) void refreshDecisions();
          }}
        />

        {feedError && <Alert severity="error">{feedError}</Alert>}

        {status?.connected && decisions && <SummaryStrip summary={decisions.response.summary} />}

        <DecisionFeed
          connected={Boolean(status?.connected)}
          decisions={decisions}
          mode={mode}
          onModeChange={setMode}
          onInspect={setInspectedDossierId}
          onConnect={() => setSettingsOpen(true)}
        />
      </Stack>

      <ConnectionSettings
        open={settingsOpen}
        backend={backend}
        status={status}
        onClose={() => setSettingsOpen(false)}
        onChanged={(next) => {
          setStatus(next);
          setFeedError(null);
        }}
      />

      <DossierInspector
        backend={backend}
        dossierId={inspectedDossierId}
        onClose={() => setInspectedDossierId(null)}
      />
    </Box>
  );
}
