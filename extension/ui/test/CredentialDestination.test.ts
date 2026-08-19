import { describe, expect, it } from "vitest";

// Mirrors the component helper; the behaviour under test is that a custom
// base URL is never silently treated as the default destination.
function credentialDestination(baseUrl: string): string {
  const trimmed = baseUrl.trim();
  if (!trimmed) return "api.decionis.com";
  try {
    return new URL(/^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`).host;
  } catch {
    return trimmed;
  }
}

describe("where a typed password would actually go", () => {
  it("names the default plane when no base URL is set", () => {
    expect(credentialDestination("")).toBe("api.decionis.com");
    expect(credentialDestination("   ")).toBe("api.decionis.com");
  });

  it("names a custom host so it cannot pass for the default", () => {
    expect(credentialDestination("https://decionis.internal.acme.test")).toBe(
      "decionis.internal.acme.test",
    );
    // Bare host, and a host that merely looks like the real one.
    expect(credentialDestination("evil.example")).toBe("evil.example");
    expect(credentialDestination("https://api.decionis.com.evil.example")).toBe(
      "api.decionis.com.evil.example",
    );
  });

  it("shows unparseable input verbatim rather than claiming a destination", () => {
    expect(credentialDestination("http://[bad")).toBe("http://[bad");
  });
});
