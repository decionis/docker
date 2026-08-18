/**
 * Hand-authored preview theme, used ONLY outside Docker Desktop (vite
 * dev/preview) so local previews approximate the native Desktop look.
 *
 * Inside Docker Desktop this module never loads: Desktop injects its real
 * theme bag as `window.__ddMuiV5Themes` before extension code runs, and
 * Main.tsx keeps any existing bag. This file intentionally contains no
 * content extracted from Docker Desktop — it is a small, original
 * approximation (Docker-blue primary, dense controls, Roboto-first stack).
 */
import type { ThemeOptions } from "@mui/material/styles";

const typography: ThemeOptions["typography"] = {
  fontFamily: '"Roboto", system-ui, -apple-system, "Segoe UI", sans-serif',
  h3: { fontSize: "1.5rem", fontWeight: 500, letterSpacing: 0 },
  h4: { fontSize: "1.25rem", fontWeight: 500 },
  h5: { fontSize: "1.125rem", fontWeight: 500 },
  h6: { fontSize: "1rem", fontWeight: 500 },
  body1: { fontSize: "0.875rem" },
  body2: { fontSize: "0.8125rem" },
  caption: { fontSize: "0.75rem" },
};

const components: ThemeOptions["components"] = {
  MuiButton: {
    defaultProps: { size: "small", disableElevation: true },
    styleOverrides: { root: { textTransform: "none", borderRadius: 4 } },
  },
  MuiChip: { styleOverrides: { root: { borderRadius: 4, fontWeight: 500 } } },
  MuiTextField: { defaultProps: { size: "small" } },
  MuiCard: { styleOverrides: { root: { backgroundImage: "none" } } },
  MuiTableCell: { styleOverrides: { root: { fontSize: "0.8125rem" } } },
};

const dark: ThemeOptions = {
  palette: {
    mode: "dark",
    primary: { main: "#2986FF" },
    background: { default: "#0E1318", paper: "#141A21" },
    divider: "rgba(255, 255, 255, 0.10)",
    text: { primary: "#E8EDF2", secondary: "#93A5B5" },
    success: { main: "#4CAF7D" },
    warning: { main: "#E0A63E" },
    error: { main: "#E56A64" },
  },
  typography,
  shape: { borderRadius: 4 },
  components,
};

const light: ThemeOptions = {
  palette: {
    mode: "light",
    primary: { main: "#0B6CD8" },
    background: { default: "#FAFBFC", paper: "#FFFFFF" },
    divider: "rgba(16, 21, 27, 0.12)",
    text: { primary: "#17191E", secondary: "#556678" },
    success: { main: "#2E7D57" },
    warning: { main: "#9A6B10" },
    error: { main: "#C4394B" },
  },
  typography,
  shape: { borderRadius: 4 },
  components,
};

/** Shape matches the `window.__ddMuiV5Themes` contract: one MUI
 * ThemeOptions per color scheme. */
export const desktopThemeFallback = { dark, light };
