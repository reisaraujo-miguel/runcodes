import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import fs, { type Dirent } from "node:fs";
import path from "path";
import { optimize, type Config as SvgoConfig } from "svgo";
import { defineConfig, type Plugin } from "vite";
import svgr from "vite-plugin-svgr";

import svgoConfig from "./svgo.config";

/**
 * Optimises the SVGs that reach `dist/` as files: assets imported by URL and
 * files copied verbatim from `public/`. The SVGR plugin only handles
 * `*.svg?react` imports, and neither Vite's asset graph nor `generateBundle`
 * sees `public/` files, so the emitted `.svg` files are rewritten once the
 * build has finished writing them.
 */
function svgoAssets(config: SvgoConfig): Plugin {
  let outDir = "";
  return {
    name: "svgo-assets",
    apply: "build",
    configResolved(resolved) {
      outDir = path.resolve(resolved.root, resolved.build.outDir);
    },
    async closeBundle() {
      for (const file of await collectSvgFiles(outDir)) {
        const source = await fs.promises.readFile(file, "utf8");
        const { data } = optimize(source, config);
        if (data !== source) {
          await fs.promises.writeFile(file, data);
        }
      }
    },
  };
}

async function collectSvgFiles(dir: string): Promise<string[]> {
  let entries: Dirent[];
  try {
    entries = await fs.promises.readdir(dir, { withFileTypes: true });
  } catch {
    return [];
  }
  const found = await Promise.all(
    entries.map(async (entry) => {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        return collectSvgFiles(full);
      }
      return entry.name.endsWith(".svg") ? [full] : [];
    }),
  );
  return found.flat();
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    babel({ presets: [reactCompilerPreset()] }),
    tailwindcss(),
    svgr({
      svgrOptions: {
        plugins: ["@svgr/plugin-svgo", "@svgr/plugin-jsx"],
        svgoConfig,
        memo: true,
        //icon: true,
        replaceAttrValues: {
          "#000": "currentColor",
          "#000000": "currentColor",
        },
      },
    }),
    svgoAssets(svgoConfig),
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
            const [urlPath = ""] = req.url.split("?");
            pathname = decodeURIComponent(urlPath);
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
          // index.html, which Firefox DevTools fails to JSON.parse. Serve a
          // valid stub map instead. It must declare at least one source,
          // otherwise Firefox warns "No sources are declared in this source
          // map".
          const mapFile = pathname.slice(pathname.lastIndexOf("/") + 1);
          const sourceName = mapFile.replace(/\.map$/, "") || "source";

          res.statusCode = 200;
          res.setHeader("Content-Type", "application/json");
          res.end(
            JSON.stringify({
              version: 3,
              sources: [sourceName],
              sourcesContent: [""],
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
