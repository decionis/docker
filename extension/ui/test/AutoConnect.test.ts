import { describe, expect, it } from "vitest";

import { interpretAutoConnect } from "../src/services/AutoConnect";
import type { AutoConnectStatus } from "../src/services/BackendClient";

const answer = (overrides: Partial<AutoConnectStatus> = {}): AutoConnectStatus => ({
  daemon_version: "test",
  connected: true,
  base_url: "https://api.decionis.com",
  org_id: "11111111-1111-4111-8111-111111111111",
  last_sync: null,
  last_error: null,
  ...overrides,
});

describe("automatic signup outcomes", () => {
  it("treats a plain mint — today's shape, older daemons included — as fresh", () => {
    expect(interpretAutoConnect(answer())).toEqual({ kind: "fresh" });
  });

  it("turns a reused mint into a notice naming the workspace", () => {
    const outcome = interpretAutoConnect(answer({ reused: true, org_name: "My Docker Workspace" }));
    expect(outcome).toEqual({
      kind: "reconnected",
      notice: "Reconnected to your existing workspace My Docker Workspace.",
    });
  });

  it("still notices a reuse when the daemon sent no workspace name", () => {
    const outcome = interpretAutoConnect(answer({ reused: true, org_name: "   " }));
    expect(outcome).toEqual({
      kind: "reconnected",
      notice: "Reconnected to your existing workspace.",
    });
  });

  it("maps a claimed workspace to the sign-in state with its browser fallback", () => {
    const outcome = interpretAutoConnect(
      answer({
        connected: false,
        claimed: true,
        sign_in_url: "https://api.decionis.com/v1/public/connect/docker-desktop/start",
      }),
    );
    expect(outcome).toEqual({
      kind: "claimed",
      signInUrl: "https://api.decionis.com/v1/public/connect/docker-desktop/start",
    });
  });

  it("keeps the sign-in state even when the daemon relayed no browser URL", () => {
    const outcome = interpretAutoConnect(answer({ connected: false, claimed: true }));
    expect(outcome).toEqual({ kind: "claimed", signInUrl: null });
  });

  it("never reads a disconnected non-claimed answer as anything but fresh", () => {
    expect(interpretAutoConnect(answer({ connected: false }))).toEqual({ kind: "fresh" });
    expect(interpretAutoConnect(answer({ connected: false, reused: true }))).toEqual({
      kind: "fresh",
    });
  });
});
