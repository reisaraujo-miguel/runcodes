/**
 * Helpers for the file-type extensions configured on an exercise. The backend
 * stores extensions without a leading dot and may use compound ones (e.g.
 * "omp.c"), so normalization keeps a single dotted form for UI checks.
 */

/** Normalizes an extension to lowercase with a leading dot. */
export function normalizeExtension(extension: string): string {
  const trimmed = extension.trim().toLowerCase();
  if (!trimmed) return "";
  return trimmed.startsWith(".") ? trimmed : `.${trimmed}`;
}

/** Returns the dot-prefixed extension of a filename, or "" when it has none. */
export function extensionOf(filename: string): string {
  const dot = filename.lastIndexOf(".");
  return dot === -1 ? "" : filename.slice(dot).toLowerCase();
}

/**
 * Reports whether a filename matches one of the allowed extensions. Compound
 * extensions are supported (e.g. "main.omp.c" matches "omp.c"): the check is a
 * suffix match on the dotted extension rather than on the last path segment.
 */
export function matchesAllowedExtension(
  filename: string,
  allowed: readonly string[],
): boolean {
  const name = filename.trim().toLowerCase();
  return allowed.some((value) => {
    const extension = normalizeExtension(value);
    return extension !== "" && name.endsWith(extension);
  });
}

/** Validates a file against the allowed extensions, returning an error message. */
export function validateFileType(
  file: File,
  allowed: readonly string[] | null,
): string | null {
  if (!allowed) return null;
  if (matchesAllowedExtension(file.name, allowed)) return null;
  return `Extensão não permitida. Tipos aceitos: ${allowed.join(", ")}.`;
}
