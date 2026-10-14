import type { Config } from "svgo";

/**
 * SVGO configuration shared by:
 * - the SVGR component transform and the asset pass in `vite.config.ts`;
 * - the `bun run optimize-svg` script that rewrites the source SVGs in place.
 *
 * Keeping a single source of truth means a `?react` import and a file optimised
 * by the script are cleaned up the same way.
 */
const svgoConfig: Config = {
  // Run the plugins until the output stops shrinking. Several of them
  // (e.g. `mergePaths`, `convertPathData`) only fully converge after a second
  // pass, so without this a single run leaves room and re-running the command
  // keeps changing the files.
  multipass: true,
  floatPrecision: 2,
  // SVGO drops its defaults as soon as `plugins` is provided, so
  // `preset-default` must be listed explicitly. Without it none of the standard
  // optimisations run (metadata/comment removal, id cleanup, path & style
  // minification, group collapsing, …).
  plugins: [
    {
      name: "preset-default",
      params: {
        // Keep `viewBox` so the artwork stays resolution-independent.
        overrides: { removeViewBox: false },
      },
    },
    // Inkscape exports keep `style="font-family:'…';-inkscape-font-…"` on
    // vectorised text. `convertStyleToAttrs` hoists the real presentation props
    // (fill, …) onto attributes; dropping the leftover `style` then removes the
    // vendor-only declarations that would otherwise turn into invalid JSX such
    // as `InkscapeFontSpecification`.
    "convertStyleToAttrs",
    { name: "removeAttrs", params: { attrs: "style" } },
  ],
};

export default svgoConfig;
