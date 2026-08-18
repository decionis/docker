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
  request(method: "GET" | "PUT" | "DELETE", path: string, body?: unknown): Promise<unknown>;
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

  connect(
    input:
      | { enrollment_token: string; base_url?: string }
      | { org_id: string; api_key: string; base_url?: string },
  ): Promise<DaemonStatus> {
    return this.transport.request("PUT", "/api/connection", input) as Promise<DaemonStatus>;
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
