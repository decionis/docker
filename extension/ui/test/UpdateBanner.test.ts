import { describe, expect, it } from "vitest";

import { shouldShowUpdateBanner } from "../src/services/UpdateBanner";

const info = (overrides: Partial<Parameters<typeof shouldShowUpdateBanner>[0] & object> = {}) => ({
  current_version: "0.1.2",
  latest_version: "0.1.3",
  update_available: true,
  checked: true,
  ...overrides,
});

describe("update banner gating", () => {
  it("shows for a checked, newer, undismissed release", () => {
    expect(shouldShowUpdateBanner(info(), null)).toBe(true);
  });

  it("stays hidden when the check did not complete (fail-open, honest)", () => {
    expect(shouldShowUpdateBanner(info({ checked: false }), null)).toBe(false);
  });

  it("stays hidden when no update is available", () => {
    expect(shouldShowUpdateBanner(info({ update_available: false }), null)).toBe(false);
  });

  it("stays hidden for the dismissed version but returns for the next one", () => {
    expect(shouldShowUpdateBanner(info(), "0.1.3")).toBe(false);
    expect(shouldShowUpdateBanner(info({ latest_version: "0.1.4" }), "0.1.3")).toBe(true);
  });

  it("shows nothing without data", () => {
    expect(shouldShowUpdateBanner(null, null)).toBe(false);
  });
});
