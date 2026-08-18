/**
 * Snapshot of Docker Desktop's injected extension theme bag
 * (`window.__ddMuiV5Themes`), extracted from the installed Docker Desktop's
 * extension-preload bundle on 2026-08-18.
 *
 * Used ONLY outside Docker Desktop (vite dev/preview) so previews render the
 * native Desktop look. Inside Docker Desktop the live injected theme always
 * wins — Desktop assigns the global before extension code runs, and Main.tsx
 * never overwrites an existing bag. Re-extract on major Desktop releases if
 * the preview drifts from the real thing.
 */
import bag from "./DesktopThemeBag.json";

export const desktopThemeFallback = bag;
