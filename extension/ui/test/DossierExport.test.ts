import { describe, expect, it, vi } from "vitest";

import {
  copyDossierExport,
  createDossierExport,
  downloadDossierExport,
} from "../src/features/dossiers/DossierExport";
import type { DossierPayload } from "../src/services/BackendClient";

const payload: DossierPayload = {
  dossier_id: "dossier/with spaces",
  jwks_url: "https://api.decionis.com/.well-known/jwks.json",
  verification: {
    verified: true,
    available: true,
    key_id: "key-1",
    artifacts_checked: 2,
    checks: [
      { key: "signature", label: "Signature", verified: true, severity: "pass", detail: "valid" },
    ],
  },
  reproducibility: {
    posture: "reproduction_ready",
    detail: "All inputs are present.",
    inputs: [{ key: "policy", label: "Policy", present: true, value: "v2" }],
  },
  payload: { outcome: "ESCALATE", nested: { evidence_hash: "abc123" } },
};

describe("local dossier export", () => {
  it("serializes the complete dossier, verification, and reproducibility for copy", async () => {
    const writeText = vi.fn(async (_json: string) => undefined);
    const feedback = await copyDossierExport(payload, writeText);

    expect(feedback.severity).toBe("success");
    expect(writeText).toHaveBeenCalledOnce();
    expect(JSON.parse(writeText.mock.calls[0][0])).toEqual(payload);
  });

  it("uses the same complete JSON and a safe filename for download", () => {
    const saveFile = vi.fn();
    const feedback = downloadDossierExport(payload, saveFile);
    const exported = createDossierExport(payload);

    expect(feedback.severity).toBe("success");
    expect(saveFile).toHaveBeenCalledWith(exported);
    expect(exported.filename).toBe("decionis-dossier-with-spaces.json");
    expect(JSON.parse(exported.json)).toEqual(payload);
  });

  it("returns accessible error feedback when the clipboard write fails", async () => {
    const feedback = await copyDossierExport(payload, async () => {
      throw new Error("permission denied");
    });

    expect(feedback).toEqual({
      severity: "error",
      message: "Dossier JSON could not be copied.",
    });
  });
});
