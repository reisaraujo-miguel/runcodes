import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import fs from "node:fs";
import path from "path";
import { defineConfig } from "vite";
import svgr from "vite-plugin-svgr";

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    babel({ presets: [reactCompilerPreset()] }),
    tailwindcss(),
    svgr({
      svgrOptions: {
        plugins: ["@svgr/plugin-svgo", "@svgr/plugin-jsx"],
        svgoConfig: {
          floatPrecision: 2,
          plugins: ["convertStyleToAttrs", "removeEditorsNSData"],
        },
        memo: true,
        icon: true,
        replaceAttrValues: {
          "#000": "currentColor",
          "#000000": "currentColor",
        },
      },
    }),
    {
      name: "missing-source-map-guard",
      apply: "serve",
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          if (req.method !== "GET" || !req.url) {
            next();
            return;
          }

          let pathname: string;
          try {
            pathname = decodeURIComponent(req.url.split("?")[0]);
          } catch {
            next();
            return;
          }

          if (!pathname.endsWith(".map")) {
            next();
            return;
          }

          // Let Vite serve real source maps (public/ dir or raw fs files).
          const candidates = pathname.startsWith("/@fs/")
            ? [pathname.slice("/@fs".length)]
            : [
                path.join(server.config.root, pathname),
                path.join(server.config.publicDir, pathname),
              ];
          if (candidates.some((file) => fs.existsSync(file))) {
            next();
            return;
          }

          // Browser extensions (e.g. React DevTools) reference source maps
          // such as `installHook.js.map` relative to the page origin. With
          // SPA fallback enabled Vite answers those missing files with
          // index.html, which Firefox DevTools fails to JSON.parse. Serve an
          // empty but valid source map instead.
          res.statusCode = 200;
          res.setHeader("Content-Type", "application/json");
          res.end(
            JSON.stringify({
              version: 3,
              sources: [],
              names: [],
              mappings: "",
            }),
          );
        });
      },
    },
  ],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
});
