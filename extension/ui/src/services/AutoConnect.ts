import type { AutoConnectStatus } from "./BackendClient";

/**
 * The three ways an automatic signup can land, interpreted once so the app
 * shell only renders states:
 *
 * - `fresh` — a workspace was minted (or the daemon predates reuse and
 *   answered in today's shape). Nothing extra to show.
 * - `reconnected` — the control plane recognized this install and handed its
 *   existing workspace back with a fresh key; worth a quiet notice.
 * - `claimed` — that workspace belongs to an account now, so nothing was
 *   connected. The user signs in through the usual connect options;
 *   `signInUrl` is the plain-browser fallback when the daemon relayed one.
 */
export type AutoConnectOutcome =
  | { kind: "fresh" }
  | { kind: "reconnected"; notice: string }
  | { kind: "claimed"; signInUrl: string | null };

export function interpretAutoConnect(status: AutoConnectStatus): AutoConnectOutcome {
  if (status.claimed === true) {
    const signInUrl = typeof status.sign_in_url === "string" && status.sign_in_url ? status.sign_in_url : null;
    return { kind: "claimed", signInUrl };
  }
  if (status.connected && status.reused === true) {
    const orgName = typeof status.org_name === "string" ? status.org_name.trim() : "";
    return {
      kind: "reconnected",
      notice: orgName
        ? `Reconnected to your existing workspace ${orgName}.`
        : "Reconnected to your existing workspace.",
    };
  }
  return { kind: "fresh" };
}
