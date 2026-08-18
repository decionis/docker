import RefreshIcon from "@mui/icons-material/Refresh";
import SettingsIcon from "@mui/icons-material/Settings";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import IconButton from "@mui/material/IconButton";
import Stack from "@mui/material/Stack";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import type { DaemonStatus } from "../../services/BackendClient";

export function StatusHeader(props: {
  status: DaemonStatus | null;
  onOpenSettings: () => void;
  onRefresh: () => void;
}) {
  const { status } = props;
  const connected = Boolean(status?.connected);

  return (
    <Stack direction="row" alignItems="center" spacing={2}>
      <Box sx={{ flex: 1 }}>
        <Typography variant="h3">Decionis</Typography>
        <Typography variant="body2" color="text.secondary">
          Execution authority for AI agents — decisions, verdicts, and signed Decision Dossiers.
        </Typography>
      </Box>
      {connected ? (
        <Tooltip title={`Org ${status?.org_id ?? ""} · ${status?.base_url ?? ""}`}>
          <Chip color="success" label="Connected" size="small" />
        </Tooltip>
      ) : (
        <Chip label="Not connected" size="small" />
      )}
      {status?.last_error && <Chip color="warning" size="small" label={status.last_error} />}
      <Tooltip title="Refresh">
        <IconButton onClick={props.onRefresh} size="small" aria-label="Refresh">
          <RefreshIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Button variant="outlined" size="small" startIcon={<SettingsIcon />} onClick={props.onOpenSettings}>
        Settings
      </Button>
    </Stack>
  );
}
