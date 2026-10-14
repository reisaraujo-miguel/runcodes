import type { Dirent } from "node:fs";
import { readdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { optimize } from "svgo";

import svgoConfig from "../svgo.config";

/** Directories (relative to the frontend root) that hold source SVGs. */
const roots = ["src/assets", "public"];

async function collectSvgFiles(dir: string): Promise<string[]> {
  let entries: Dirent[];
  try {
    entries = await readdir(dir, { withFileTypes: true });
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

const check = process.argv.includes("--check");

const files = (await Promise.all(roots.map((root) => collectSvgFiles(root))))
  .flat()
  .sort();

let before = 0;
let after = 0;
const unoptimized: string[] = [];

for (const file of files) {
  const source = await readFile(file, "utf8");
  const { data } = optimize(source, { ...svgoConfig, path: file });
  const sourceSize = Buffer.byteLength(source);
  const dataSize = Buffer.byteLength(data);
  before += sourceSize;
  after += dataSize;

  if (data === source) {
    if (!check) {
      console.log(`unchanged  ${file}`);
    }
    continue;
  }

  // In check mode nothing is written; the file is only reported.
  if (check) {
    unoptimized.push(file);
    console.log(
      `not optimised  ${file}  ${String(sourceSize)} → ${String(dataSize)} B`,
    );
    continue;
  }

  await writeFile(file, data);
  console.log(
    `optimized  ${file}  ${String(sourceSize)} → ${String(dataSize)} B`,
  );
}

if (!check) {
  console.log(
    `\n${String(files.length)} SVG file(s) checked, ` +
      `${String(before - after)} B saved ` +
      `(${String(before)} → ${String(after)} B)`,
  );
} else if (unoptimized.length === 0) {
  console.log(`${String(files.length)} SVG file(s) already optimised`);
} else {
  const message =
    `${String(unoptimized.length)} SVG file(s) are not optimised; ` +
    "run `bun run optimize-svg` and commit the result";
  if (process.env.GITHUB_ACTIONS) {
    console.log(`::error::${message}`);
  }
  console.log(message);
  process.exitCode = 1;
}
