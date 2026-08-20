import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import {
  ApprovalsPanel,
  inspectApprovalDossier,
} from "../src/features/decisions/ApprovalsPanel";
import type { PendingApproval } from "../src/services/BackendClient";

function approval(overrides: Partial<PendingApproval> = {}): PendingApproval {
  return {
    evaluation_id: "eval-approval",
    decision_type: "deploy",
    outcome: "ESCALATE",
    mode: "ENFORCEMENT",
    policy_version: "policy-v2",
    amount: null,
    channel: "mcp",
    dossier_id: "dossier-approval",
    created_at: "2026-08-20T10:00:00Z",
    override_status: null,
    ...overrides,
  };
}

describe("approval dossier investigation", () => {
  it("navigates an approval with a dossier ID through the supplied inspector callback", () => {
    const onInspect = vi.fn();
    inspectApprovalDossier(approval(), onInspect);
    expect(onInspect).toHaveBeenCalledWith("dossier-approval");
  });

  it("does not offer or trigger dossier navigation without a dossier ID", () => {
    const onInspect = vi.fn();
    const withoutDossier = approval({ evaluation_id: "eval-no-dossier", dossier_id: null });
    inspectApprovalDossier(withoutDossier, onInspect);
    expect(onInspect).not.toHaveBeenCalled();

    const html = renderToStaticMarkup(
      createElement(ApprovalsPanel, {
        approvals: [withoutDossier],
        error: null,
        onInspect,
      }),
    );
    expect(html).not.toContain("Inspect dossier");
    expect(html).not.toContain("Approve");
    expect(html).not.toContain("Reject");
  });

  it("renders one inspect action for each approval that has a dossier", () => {
    const html = renderToStaticMarkup(
      createElement(ApprovalsPanel, {
        approvals: [approval(), approval({ evaluation_id: "eval-no-dossier", dossier_id: null })],
        error: null,
        onInspect: vi.fn(),
      }),
    );
    expect(html.match(/Inspect dossier/g)).toHaveLength(1);
  });
});
