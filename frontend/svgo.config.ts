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
  // Providing `plugins` replaces SVGO's defaults, so `preset-default` has to be
  // listed explicitly. It is the standard bundle (metadata/comment removal, id
  // cleanup, path & style minification, group collapsing, …) and, unlike a
  // hand-picked list, keeps `viewBox` so the artwork stays resolution
  // independent.
  plugins: [
    "preset-default",
    // Not part of `preset-default`: hoists real presentation props (fill, …)
    // onto attributes. Inkscape exports keep `style="font-family:…;-inkscape-…"`
    // on vectorised text, and the leftover `style` is then dropped by the plugin
    // below so it can't become invalid JSX such as `InkscapeFontSpecification`.
    "convertStyleToAttrs",
    { name: "removeAttrs", params: { attrs: "style" } },
  ],
};

export default svgoConfig;
