import type { UpdateInfo } from "./BackendClient";

const DISMISSED_KEY = "decionis.dismissed-update-version";

/**
 * The banner shows only for a checked, genuinely newer release the user has
 * not dismissed — dismissal is per version, so the next release surfaces
 * again. Nothing here is secret; localStorage is fine
 * (rules/security.rules.md Rule 2.2 concerns credentials only).
 */
export function shouldShowUpdateBanner(
  info: UpdateInfo | null,
  dismissedVersion: string | null,
): boolean {
  if (!info || !info.checked || !info.update_available || !info.latest_version) return false;
  return info.latest_version !== dismissedVersion;
}

export function readDismissedVersion(): string | null {
  try {
    return window.localStorage.getItem(DISMISSED_KEY);
  } catch {
    return null;
  }
}

export function dismissUpdateVersion(version: string): void {
  try {
    window.localStorage.setItem(DISMISSED_KEY, version);
  } catch {
    // Storage unavailable — the banner simply reappears next session.
  }
}
