import type { DecisionReport } from "../../services/BackendClient";

export interface DecisionFilters {
  search: string;
  outcome: string;
  executionAction: string;
  policyVersion: string;
  domainOrType: string;
}

export const EMPTY_DECISION_FILTERS: DecisionFilters = {
  search: "",
  outcome: "",
  executionAction: "",
  policyVersion: "",
  domainOrType: "",
};

export type DomainOrTypeOption = {
  value: string;
  label: string;
};

function normalized(value: unknown): string {
  if (value === null || value === undefined) return "";
  return String(value).trim().toLocaleLowerCase();
}

function matches(value: unknown, filter: string): boolean {
  return filter === "" || normalized(value) === normalized(filter);
}

/** Client-side investigation only: no report fields or verdicts are changed. */
export function filterDecisionReports(
  reports: readonly DecisionReport[],
  filters: DecisionFilters,
): DecisionReport[] {
  const search = normalized(filters.search);

  return reports.filter((report) => {
    const searchable = [
      report.decision_type,
      report.decision_domain,
      report.reason,
      report.policy_version,
      report.selected_rule_id,
      report.evaluation_id,
      report.dossier_id,
    ]
      .map(normalized)
      .join("\n");

    const domainOrTypeMatches =
      filters.domainOrType === "" ||
      (filters.domainOrType.startsWith("domain:")
        ? matches(report.decision_domain, filters.domainOrType.slice("domain:".length))
        : matches(report.decision_type, filters.domainOrType.slice("type:".length)));

    return (
      (search === "" || searchable.includes(search)) &&
      matches(report.outcome, filters.outcome) &&
      matches(report.execution_action, filters.executionAction) &&
      matches(report.policy_version, filters.policyVersion) &&
      domainOrTypeMatches
    );
  });
}

export function uniqueReportValues(
  reports: readonly DecisionReport[],
  select: (report: DecisionReport) => unknown,
): string[] {
  const values = new Map<string, string>();
  for (const report of reports) {
    const raw = select(report);
    if (raw === null || raw === undefined) continue;
    const value = String(raw).trim();
    if (value === "") continue;
    const key = normalized(value);
    if (!values.has(key)) values.set(key, value);
  }
  return [...values.values()].sort((left, right) => left.localeCompare(right));
}

export function domainOrTypeOptions(reports: readonly DecisionReport[]): DomainOrTypeOption[] {
  return [
    ...uniqueReportValues(reports, (report) => report.decision_domain).map((value) => ({
      value: `domain:${value}`,
      label: `Domain: ${value}`,
    })),
    ...uniqueReportValues(reports, (report) => report.decision_type).map((value) => ({
      value: `type:${value}`,
      label: `Type: ${value}`,
    })),
  ];
}
