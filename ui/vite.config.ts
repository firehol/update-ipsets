import { defineConfig } from "vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import babel from "@rolldown/plugin-babel";
import path from "node:path";

// https://vite.dev/config/
//
// During development the React app talks to the running update-ipsets daemon
// at http://localhost:18888 — Vite proxies /api/* and /static/* through so the
// browser can use relative URLs and CORS is a non-issue.
//
// In production the React bundle is embedded into the Go binary under
// pkg/web/static/. The base path is /static/ so every JS/CSS asset
// reference inside the built index.html points at /static/assets/* —
// matching the daemon's existing /static/* file handler with no extra
// route plumbing.
export default defineConfig({
  base: "/static/",
  plugins: [
    react(),
    babel({ presets: [reactCompilerPreset({ compilationMode: "annotation" })] }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:18888",
        changeOrigin: true,
      },
      "/static": {
        target: "http://localhost:18888",
        changeOrigin: true,
      },
      // Per-feed JSON files (e.g. /dshield.json, /dshield_geolite2_country.json)
      // are served by the daemon directly from its web dir. We proxy any
      // top-level path that ends in .json so the optimized fetch path keeps
      // working in dev.
      "^/[a-z0-9_]+\\.json$": {
        target: "http://localhost:18888",
        changeOrigin: true,
      },
      "^/[a-z0-9_]+\\.csv$": {
        target: "http://localhost:18888",
        changeOrigin: true,
      },
    },
  },
});
