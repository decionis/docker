import { describe, expect, it } from "vitest";

import {
  EXECUTION_ACTIONS,
  PROTOCOL_OUTCOMES,
  REPORT_MODES,
  presentOutcome,
} from "../src/protocol/VerdictLabels";
import enums from "./fixtures/ProtocolEnums.json";

// The drift gate of rules/discovery.rules.md Rules 1.4 and 2.6: the UI's
// vocabulary module must cover exactly the enums recorded from the published
// OpenAPI contract — no repo-local synonyms, nothing missing, nothing extra.
describe("VerdictLabels drift gate", () => {
  it("covers exactly the protocol outcome enum", () => {
    expect([...PROTOCOL_OUTCOMES].sort()).toEqual([...enums.outcomes].sort());
  });

  it("covers exactly the execution action enum", () => {
    expect([...EXECUTION_ACTIONS].sort()).toEqual([...enums.execution_actions].sort());
  });

  it("covers exactly the report mode enum", () => {
    expect([...REPORT_MODES].sort()).toEqual([...enums.modes].sort());
  });

  it("renders every known outcome verbatim", () => {
    for (const outcome of enums.outcomes) {
      expect(presentOutcome(outcome).label).toBe(outcome);
    }
  });

  it("renders unknown outcomes verbatim with a neutral tone instead of guessing", () => {
    const presentation = presentOutcome("FUTURE_OUTCOME");
    expect(presentation.label).toBe("FUTURE_OUTCOME");
    expect(presentation.tone).toBe("info");
  });
});
