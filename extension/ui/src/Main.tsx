import { DockerMuiThemeProvider } from "@docker/docker-mui-theme";
import CssBaseline from "@mui/material/CssBaseline";
import React from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";

// Docker Desktop injects its native extension theme as
// `window.__ddMuiV5Themes` before this script runs. Outside Desktop (vite
// dev/preview) that global is absent, so a snapshot of the same bag is
// lazy-loaded — the preview and the real extension render identically, and
// inside Desktop the injected live theme always wins.
const themeHost = window as unknown as { __ddMuiV5Themes?: unknown };
if (!themeHost.__ddMuiV5Themes) {
  const { desktopThemeFallback } = await import("./theme/DesktopThemeFallback");
  themeHost.__ddMuiV5Themes = desktopThemeFallback;
}

const container = document.getElementById("root");
if (!container) throw new Error("root element missing");
createRoot(container).render(
  <React.StrictMode>
    <DockerMuiThemeProvider>
      <CssBaseline />
      <App />
    </DockerMuiThemeProvider>
  </React.StrictMode>,
);
