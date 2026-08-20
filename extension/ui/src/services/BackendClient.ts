/**
 * BackendClient talks to the Decionis Docker daemon (the extension backend)
 * over the Docker Desktop extension socket. Outside Docker Desktop (vite
 * dev), it falls back to the daemon's loopback dev listener.
 *
 * Credentials pass through this client exactly once — from the settings form
 * to the daemon — and are never stored on the UI side
 * (rules/security.rules.md Rule 2.3).
 */
import { createDockerDesktopClient } from "@docker/extension-api-client";

export interface DaemonStatus {
  daemon_version: string;
  connected: boolean;
  base_url?: string;
  org_id?: string;
  last_sync: string | null;
  last_error: string | null;
}

export interface DecisionReport {
  evaluation_id: string;
  dossier_id: string;
  created_at: string;
  decision_type: string;
  decision_domain?: string | null;
  amount?: number | null;
  risk_score?: number | null;
  channel?: string | null;
  policy_version: string;
  mode: string;
  outcome: string;
  confidence: number;
  would_execute: boolean;
  execution_action: string;
  reason: string;
  selected_rule_id?: string | null;
  dossier_api_path: string;
}

export interface DecisionSummary {
  total_evaluations: number;
  would_approve_count: number;
  would_block_count: number;
  would_escalate_count: number;
  review_required_count: number;
  non_approve_count: number;
  non_approve_rate: number;
  outcome_counts?: Record<string, number>;
}

export interface DecisionsPayload {
  fetched_at: string;
  response: {
    generated_at: string;
    org_id: string;
    mode: string;
    count: number;
    reports: DecisionReport[];
    summary: DecisionSummary;
  };
}

export interface VerifyCheck {
  key: string;
  label: string;
  verified: boolean;
  severity: "pass" | "warn" | "fail";
  detail: string;
}

export interface VerifyResult {
  verified: boolean;
  available: boolean;
  key_id: string | null;
  artifacts_checked: number;
  checks: VerifyCheck[];
}

export interface ReproducibilityAssessment {
  posture: "reproduction_ready" | "incomplete" | "signature_only";
  detail: string;
  inputs: Array<{ key: string; label: string; present: boolean; value: string | null }>;
}

export interface DossierPayload {
  dossier_id: string;
  jwks_url?: string;
  verification: VerifyResult;
  reproducibility: ReproducibilityAssessment;
  payload: Record<string, unknown>;
}

export interface ConnectStart {
  authorize_url: string;
}

export interface WorkspaceState {
  enforcement_enabled: boolean;
  enforcement_available: boolean;
  enforcement_reverted: boolean;
  governed_used: number;
  governed_limit: number | null;
  remaining: number | null;
  warn_at: number;
  warn: boolean;
  at_cap: boolean;
  provisional: boolean;
  subscribe_url: string;
}

export interface PendingApproval {
  evaluation_id: string;
  decision_type: string;
  outcome: string;
  mode: string;
  policy_version: string;
  amount: string | null;
  channel: string | null;
  dossier_id: string | null;
  created_at: string;
  override_status: string | null;
}

export interface ApprovalsPayload {
  approvals: PendingApproval[];
  count: number;
}

export interface DemoScenario {
  id: string;
  label: string;
  description: string;
  lane: "APPROVE" | "BLOCK" | "ESCALATE";
}

export interface DemoScenariosPayload {
  scenarios: DemoScenario[];
  count: number;
  notice: string;
}

export interface DemoEvaluationResult {
  scenario_id: string;
  label: string;
  lane: DemoScenario["lane"];
  outcome: string;
  execution_action: string;
  mode: string;
  policy_version: string;
  evaluation_id: string;
  dossier_id: string;
  would_execute: boolean;
  confidence: number;
}

export interface ClaimStart {
  claim_url: string;
  expires_in: number;
}

export interface UpdateInfo {
  current_version: string;
  latest_version?: string;
  update_available: boolean;
  checked: boolean;
}

export class BackendError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

interface Transport {
  request(method: "GET" | "POST" | "PUT" | "DELETE", path: string, body?: unknown): Promise<unknown>;
}

function parseErrorBody(status: number, body: unknown): BackendError {
  if (body && typeof body === "object" && "error" in body) {
    const inner = (body as { error: { code?: string; message?: string } }).error;
    return new BackendError(status, inner.code ?? "error", inner.message ?? "Request failed.");
  }
  return new BackendError(status, "error", "Request failed.");
}

/** Runs inside Docker Desktop: requests ride the extension socket. */
function desktopTransport(): Transport | null {
  try {
    const ddClient = createDockerDesktopClient();
    const service = ddClient.extension.vm?.service;
    if (!service) return null;
    return {
      async request(method, path, body) {
        try {
          if (method === "GET") return await service.get(path);
          if (method === "POST") return await service.post(path, body);
          if (method === "PUT") return await service.put(path, body);
          return await service.delete(path);
        } catch (raw) {
          const err = raw as { statusCode?: number; message?: unknown };
          let parsed: unknown = err.message;
          if (typeof err.message === "string") {
            try {
              parsed = JSON.parse(err.message);
            } catch {
              parsed = undefined;
            }
          }
          throw parseErrorBody(err.statusCode ?? 500, parsed);
        }
      },
    };
  } catch {
    return null;
  }
}

/** vite dev fallback: same-origin /api, proxied to the daemon's loopback
 * listener by vite (see vite.config.ts). */
function devTransport(): Transport {
  const base = "";
  return {
    async request(method, path, body) {
      const response = await fetch(base + path, {
        method,
        headers: body === undefined ? undefined : { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      const text = await response.text();
      const parsed = text ? (JSON.parse(text) as unknown) : undefined;
      if (!response.ok) throw parseErrorBody(response.status, parsed);
      return parsed;
    },
  };
}

export class BackendClient {
  private readonly transport: Transport;

  constructor() {
    this.transport = desktopTransport() ?? devTransport();
  }

  status(): Promise<DaemonStatus> {
    return this.transport.request("GET", "/api/status") as Promise<DaemonStatus>;
  }

  update(): Promise<UpdateInfo> {
    return this.transport.request("GET", "/api/update") as Promise<UpdateInfo>;
  }

  connect(
    input:
      | { email: string; password: string; base_url?: string }
      | { enrollment_token: string; base_url?: string }
      | { org_id: string; api_key: string; base_url?: string },
  ): Promise<DaemonStatus> {
    return this.transport.request("PUT", "/api/connection", input) as Promise<DaemonStatus>;
  }

  /**
   * Automatic signup: asks the daemon to create a workspace with no account
   * and no input at all. Used once, on first open.
   */
  connectAuto(baseUrl?: string): Promise<DaemonStatus> {
    const body = baseUrl && baseUrl.trim() ? { base_url: baseUrl.trim() } : {};
    return this.transport.request("POST", "/api/connect/auto", body) as Promise<DaemonStatus>;
  }

  /** Starts a one-click connect; the returned URL opens in the browser. */
  connectStart(baseUrl?: string): Promise<ConnectStart> {
    const body = baseUrl && baseUrl.trim() ? { base_url: baseUrl.trim() } : {};
    return this.transport.request("POST", "/api/connect/start", body) as Promise<ConnectStart>;
  }

  /** Opens a URL in the host browser (Docker Desktop) or a new tab (dev). */
  openExternal(url: string): void {
    try {
      createDockerDesktopClient().host.openExternal(url);
      return;
    } catch {
      // Outside Docker Desktop (vite dev): plain browser behavior.
    }
    window.open(url, "_blank", "noopener");
  }

  /** Enforcement state and the free governed-decision allowance. */
  workspace(): Promise<WorkspaceState> {
    return this.transport.request("GET", "/api/workspace") as Promise<WorkspaceState>;
  }

  /** Entries awaiting a person's review. */
  approvals(): Promise<ApprovalsPayload> {
    return this.transport.request("GET", "/api/approvals") as Promise<ApprovalsPayload>;
  }

  /** The daemon's fixed, non-executing policy-check proposals. */
  demoScenarios(): Promise<DemoScenariosPayload> {
    return this.transport.request("GET", "/api/demo/scenarios") as Promise<DemoScenariosPayload>;
  }

  /** Evaluates one fixed proposal against the connected workspace. */
  evaluateDemoScenario(scenarioId: string): Promise<DemoEvaluationResult> {
    return this.transport.request("POST", "/api/demo/evaluate", {
      scenario_id: scenarioId,
    }) as Promise<DemoEvaluationResult>;
  }

  /** Mints a claim URL for this workspace; the browser finishes the claim. */
  startClaim(): Promise<ClaimStart> {
    return this.transport.request("POST", "/api/workspace/claim", {}) as Promise<ClaimStart>;
  }

  /** Turns enforcement on or off. Throws BackendError on refusal. */
  setEnforcement(enabled: boolean): Promise<WorkspaceState> {
    return this.transport.request("PUT", "/api/workspace/enforcement", {
      enabled,
    }) as Promise<WorkspaceState>;
  }

  disconnect(): Promise<unknown> {
    return this.transport.request("DELETE", "/api/connection");
  }

  decisions(mode: string, limit: number): Promise<DecisionsPayload> {
    const query = new URLSearchParams({ mode, limit: String(limit) });
    return this.transport.request("GET", `/api/decisions?${query.toString()}`) as Promise<DecisionsPayload>;
  }

  dossier(dossierId: string): Promise<DossierPayload> {
    return this.transport.request("GET", `/api/dossiers/${encodeURIComponent(dossierId)}`) as Promise<DossierPayload>;
  }
}
