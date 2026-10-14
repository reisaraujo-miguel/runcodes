import { describe, expect, test } from "bun:test";

import { optimize } from "svgo";

import svgoConfig from "./svgo.config";

// An Inkscape-ish source: a comment, `<metadata>` and a styled fill that
// `preset-default` should clean up, plus a `viewBox` that must survive.
const svg = `<?xml version="1.0" encoding="UTF-8"?>
<!-- generator: a comment the optimiser should drop -->
<svg xmlns="http://www.w3.org/2000/svg" width="48" height="24" viewBox="0 0 48 24">
  <metadata>generator</metadata>
  <path d="M0 0 L 48 0 L 48 24 L 0 24 Z" style="fill:#000000"/>
</svg>`;

describe("svgoConfig", () => {
  test("keeps viewBox so the artwork stays scalable", () => {
    // `removeViewBox` is deliberately not enabled: preset-default must leave the
    // viewBox alone, otherwise a future SVGO bump could silently break scaling.
    expect(optimize(svg, svgoConfig).data).toContain('viewBox="0 0 48 24"');
  });

  test("runs preset-default (comments and metadata are removed)", () => {
    const { data } = optimize(svg, svgoConfig);
    expect(data).not.toContain("<metadata");
    expect(data).not.toContain("generator");
  });

  test("maps black to currentColor so icons follow the text colour", () => {
    // Guards the GitHub icon regression: `removeUnknownsAndDefaults` strips an
    // explicit black `fill`, so it must be turned into `currentColor` first.
    expect(optimize(svg, svgoConfig).data).toContain('fill="currentColor"');
  });
});
