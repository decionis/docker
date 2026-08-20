import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ConnectionSettings } from "./features/connection/ConnectionSettings";
import { StatusHeader } from "./features/connection/StatusHeader";
import { DecisionFeed } from "./features/decisions/DecisionFeed";
import { SummaryStrip } from "./features/decisions/SummaryStrip";
import { DossierInspector } from "./features/dossiers/DossierInspector";
import { EnforcementControl } from "./features/connection/EnforcementControl";
import { ApprovalsPanel } from "./features/decisions/ApprovalsPanel";
import { DemoPolicyPack } from "./features/demo/DemoPolicyPack";
import type { ReportMode } from "./protocol/VerdictLabels";
import {
  BackendClient,
  BackendError,
  type DaemonStatus,
  type DecisionsPayload,
  type UpdateInfo,
  type PendingApproval,
  type WorkspaceState,
} from "./services/BackendClient";
import {
  dismissUpdateVersion,
  readDismissedVersion,
  shouldShowUpdateBanner,
} from "./services/UpdateBanner";

const POLL_INTERVAL_MS = 10_000;

export function App() {
  const backend = useMemo(() => new BackendClient(), []);
  const [status, setStatus] = useState<DaemonStatus | null>(null);
  const [decisions, setDecisions] = useState<DecisionsPayload | null>(null);
  const [mode, setMode] = useState<ReportMode>("ENFORCEMENT");
  const [feedError, setFeedError] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [inspectedDossierId, setInspectedDossierId] = useState<string | null>(null);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [workspace, setWorkspace] = useState<WorkspaceState | null>(null);
  const [approvals, setApprovals] = useState<PendingApproval[] | null>(null);
  const [approvalsError, setApprovalsError] = useState<string | null>(null);
  const [dismissedUpdate, setDismissedUpdate] = useState<string | null>(() => readDismissedVersion());
  const pollRef = useRef<number | null>(null);
  const autoSignupTried = useRef(false);

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

  // First run: create a workspace so the extension is useful immediately —
  // no account, no sign-in, no fields. Runs once per extension install, only
  // when the daemon reports it has never been connected; a failure is silent
  // here and simply leaves the normal connect options in the disconnected
  // view. The daemon refuses a second mint, so this can never replace a
  // working connection.
  useEffect(() => {
    if (status === null || status.connected || autoSignupTried.current) return;
    autoSignupTried.current = true;
    backend
      .connectAuto()
      .then(setStatus)
      .catch(() => {
        /* leave the disconnected view and its connect options in place */
      });
  }, [status, backend]);

  useEffect(() => {
    backend
      .update()
      .then(setUpdateInfo)
      .catch(() => setUpdateInfo(null)); // no banner when the daemon can't say
  }, [backend]);

  useEffect(() => {
    if (!status?.connected) {
      setWorkspace(null);
      return;
    }
    backend
      .workspace()
      .then((next) => {
        setWorkspace(next);
        setMode(next.enforcement_enabled ? "ENFORCEMENT" : "SHADOW");
      })
      .catch(() => setWorkspace(null)); // an older plane simply has nothing to say

    backend
      .approvals()
      .then((payload) => {
        setApprovals(payload.approvals ?? []);
        setApprovalsError(null);
      })
      .catch((raw) => {
        // Never fall back to an empty list: an unreadable queue must not
        // look like an empty one.
        setApprovals(null);
        setApprovalsError(
          raw instanceof BackendError ? raw.message : "The Decionis daemon is not reachable.",
        );
      });
  }, [status?.connected, backend, decisions]);

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

        {shouldShowUpdateBanner(updateInfo, dismissedUpdate) && (
          <Alert
            severity="info"
            onClose={() => {
              dismissUpdateVersion(updateInfo!.latest_version!);
              setDismissedUpdate(updateInfo!.latest_version!);
            }}
          >
            Version {updateInfo!.latest_version} is available (you have {updateInfo!.current_version}).
            Update with <code>docker extension update decionis/desktop-extension:{updateInfo!.latest_version}</code>.
          </Alert>
        )}

        {feedError && <Alert severity="error">{feedError}</Alert>}

        {status?.connected && workspace && (
          <EnforcementControl
            backend={backend}
            workspace={workspace}
            onChanged={(next) => {
              setWorkspace(next);
              setMode(next.enforcement_enabled ? "ENFORCEMENT" : "SHADOW");
            }}
          />
        )}

        {status?.connected && workspace && (
          <DemoPolicyPack
            backend={backend}
            enforcementEnabled={workspace.enforcement_enabled}
            onCompleted={async () => {
              setMode("ENFORCEMENT");
              const [nextWorkspace, nextApprovals, nextDecisions] = await Promise.all([
                backend.workspace(),
                backend.approvals(),
                backend.decisions("ENFORCEMENT", 100),
              ]);
              setWorkspace(nextWorkspace);
              setApprovals(nextApprovals.approvals ?? []);
              setApprovalsError(null);
              setDecisions(nextDecisions);
              setFeedError(null);
            }}
          />
        )}

        {status?.connected && (approvals !== null || approvalsError) && (
          <ApprovalsPanel approvals={approvals} error={approvalsError} />
        )}

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
