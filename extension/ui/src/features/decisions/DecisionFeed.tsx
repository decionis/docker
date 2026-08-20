import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import Chip from "@mui/material/Chip";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Divider from "@mui/material/Divider";
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
import { useMemo, useState } from "react";

import { REPORT_MODES, presentOutcome, type ReportMode } from "../../protocol/VerdictLabels";
import type {
  DecisionReport,
  DecisionsPayload,
} from "../../services/BackendClient";
import {
  domainOrTypeOptions,
  EMPTY_DECISION_FILTERS,
  filterDecisionReports,
  uniqueReportValues,
  type DecisionFilters,
} from "./DecisionFilters";

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

function displayValue(value: unknown): string {
  if (value === null || value === undefined || value === "") return "—";
  return String(value);
}

function displayTime(value: string): string {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? displayValue(value) : parsed.toLocaleString();
}

function DetailField(props: { label: string; value: unknown; wide?: boolean }) {
  return (
    <Box sx={{ minWidth: 0, gridColumn: props.wide ? "1 / -1" : undefined }}>
      <Typography variant="caption" color="text.secondary">
        {props.label}
      </Typography>
      <Typography variant="body2" sx={{ overflowWrap: "anywhere", whiteSpace: "pre-wrap" }}>
        {displayValue(props.value)}
      </Typography>
    </Box>
  );
}

function DecisionDetails(props: {
  report: DecisionReport | null;
  onClose: () => void;
  onInspect: (dossierId: string) => void;
}) {
  const report = props.report;
  return (
    <Dialog
      open={report !== null}
      onClose={props.onClose}
      fullWidth
      maxWidth="md"
      aria-labelledby="decision-details-title"
    >
      <DialogTitle id="decision-details-title">Decision details</DialogTitle>
      {report && (
        <>
          <DialogContent dividers>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))" },
                gap: 2,
              }}
            >
              <DetailField label="Evaluation ID" value={report.evaluation_id} />
              <DetailField label="Dossier ID" value={report.dossier_id} />
              <DetailField label="Created" value={displayTime(report.created_at)} />
              <DetailField label="Mode" value={report.mode} />
              <DetailField label="Decision type" value={report.decision_type} />
              <DetailField label="Decision domain" value={report.decision_domain} />
              <DetailField label="Outcome" value={report.outcome} />
              <DetailField label="Execution action" value={report.execution_action} />
              <DetailField label="Policy version" value={report.policy_version} />
              <DetailField label="Selected rule ID" value={report.selected_rule_id} />
              <DetailField label="Risk score" value={report.risk_score} />
              <DetailField label="Confidence" value={report.confidence} />
              <DetailField label="Channel" value={report.channel} />
              <DetailField label="Amount" value={report.amount} />
              <DetailField
                label="Would execute"
                value={report.would_execute ? "Yes" : "No"}
              />
              <Box />
              <DetailField label="Reason" value={report.reason} wide />
              <DetailField label="Policy guard reason" value={report.policy_guard_reason} wide />
              <DetailField
                label="Policy evaluation resolution"
                value={report.policy_evaluation_resolution}
                wide
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={props.onClose}>Close</Button>
            {report.dossier_id && (
              <Button
                variant="contained"
                onClick={() => {
                  props.onClose();
                  props.onInspect(report.dossier_id);
                }}
              >
                Inspect dossier
              </Button>
            )}
          </DialogActions>
        </>
      )}
    </Dialog>
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
  const [filters, setFilters] = useState<DecisionFilters>(EMPTY_DECISION_FILTERS);
  const [selectedReport, setSelectedReport] = useState<DecisionReport | null>(null);

  const reports = props.decisions?.response.reports ?? [];
  const filteredReports = useMemo(() => filterDecisionReports(reports, filters), [reports, filters]);
  const outcomes = useMemo(() => uniqueReportValues(reports, (report) => report.outcome), [reports]);
  const executionActions = useMemo(
    () => uniqueReportValues(reports, (report) => report.execution_action),
    [reports],
  );
  const policyVersions = useMemo(
    () => uniqueReportValues(reports, (report) => report.policy_version),
    [reports],
  );
  const domainsAndTypes = useMemo(() => domainOrTypeOptions(reports), [reports]);
  const filtersActive = Object.values(filters).some((value) => value !== "");

  function setFilter<Key extends keyof DecisionFilters>(key: Key, value: DecisionFilters[Key]) {
    setFilters((current) => ({ ...current, [key]: value }));
  }

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

  return (
    <>
      <Card variant="outlined">
        <Stack direction="row" alignItems="center" spacing={2} sx={{ p: 2, pb: 1 }}>
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

        <Box
          sx={{
            px: 2,
            pb: 1.5,
            display: "grid",
            gridTemplateColumns: "minmax(220px, 2fr) repeat(4, minmax(130px, 1fr)) auto",
            gap: 1,
            alignItems: "center",
            "@media (max-width: 1000px)": {
              gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
            },
          }}
        >
          <TextField
            size="small"
            label="Search decisions"
            placeholder="Type, reason, rule or ID"
            value={filters.search}
            onChange={(event) => setFilter("search", event.target.value)}
          />
          <TextField
            select
            size="small"
            label="Outcome"
            value={filters.outcome}
            onChange={(event) => setFilter("outcome", event.target.value)}
          >
            <MenuItem value="">All outcomes</MenuItem>
            {outcomes.map((outcome) => (
              <MenuItem key={outcome} value={outcome}>
                {outcome}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            size="small"
            label="Execution"
            value={filters.executionAction}
            onChange={(event) => setFilter("executionAction", event.target.value)}
          >
            <MenuItem value="">All actions</MenuItem>
            {executionActions.map((action) => (
              <MenuItem key={action} value={action}>
                {action}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            size="small"
            label="Policy version"
            value={filters.policyVersion}
            onChange={(event) => setFilter("policyVersion", event.target.value)}
          >
            <MenuItem value="">All policies</MenuItem>
            {policyVersions.map((version) => (
              <MenuItem key={version} value={version}>
                {version}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            size="small"
            label="Domain or type"
            value={filters.domainOrType}
            onChange={(event) => setFilter("domainOrType", event.target.value)}
          >
            <MenuItem value="">All domains and types</MenuItem>
            {domainsAndTypes.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </TextField>
          <Button
            size="small"
            disabled={!filtersActive}
            onClick={() => setFilters(EMPTY_DECISION_FILTERS)}
          >
            Clear filters
          </Button>
        </Box>

        <Typography variant="caption" color="text.secondary" sx={{ display: "block", px: 2, pb: 1 }}>
          Showing {filteredReports.length} of {reports.length} loaded decisions
        </Typography>
        <Divider />

        {reports.length === 0 ? (
          <Box sx={{ p: 4, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              No {props.mode} decision evaluations reported for this org yet.
            </Typography>
          </Box>
        ) : filteredReports.length === 0 ? (
          <Box sx={{ p: 4, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              No loaded decisions match these filters.
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
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredReports.map((report) => (
                  <TableRow
                    key={report.evaluation_id}
                    hover
                    tabIndex={0}
                    aria-label={`View details for decision ${report.evaluation_id}`}
                    sx={{ cursor: "pointer" }}
                    onClick={() => setSelectedReport(report)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        setSelectedReport(report);
                      }
                    }}
                  >
                    <TableCell sx={{ whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>
                      {displayTime(report.created_at)}
                    </TableCell>
                    <TableCell>{displayValue(report.decision_type)}</TableCell>
                    <TableCell>
                      <OutcomeChip outcome={report.outcome} />
                    </TableCell>
                    <TableCell>{displayValue(report.execution_action)}</TableCell>
                    <TableCell>{displayValue(report.policy_version)}</TableCell>
                    <TableCell sx={{ maxWidth: 320 }}>
                      <Typography variant="body2" noWrap title={report.reason}>
                        {displayValue(report.reason)}
                      </Typography>
                    </TableCell>
                    <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                      <Button
                        size="small"
                        onClick={(event) => {
                          event.stopPropagation();
                          setSelectedReport(report);
                        }}
                      >
                        Details
                      </Button>
                      {report.dossier_id && (
                        <Button
                          size="small"
                          onClick={(event) => {
                            event.stopPropagation();
                            props.onInspect(report.dossier_id);
                          }}
                        >
                          Inspect
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Card>

      <DecisionDetails
        report={selectedReport}
        onClose={() => setSelectedReport(null)}
        onInspect={props.onInspect}
      />
    </>
  );
}
