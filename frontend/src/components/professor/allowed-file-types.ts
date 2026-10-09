import type { AllowedFileType } from "@/lib/api";

/** An option in the allowed-file-types combobox. */
export interface FileTypeOption {
  value: number;
  label: string;
}

/**
 * The combobox options: every type the platform currently offers, plus any
 * already-selected type that is no longer available, so a saved selection is
 * never hidden. Types that are unavailable and unselected are left out.
 */
export function fileTypeOptions(
  types: AllowedFileType[],
  selected: number[],
): FileTypeOption[] {
  return types
    .filter((type) => type.is_available || selected.includes(type.id))
    .map((type) => ({
      value: type.id,
      label: `${type.name} (${type.extension})`,
    }));
}

/**
 * Selected ids that are absent from `options`. The API can stop returning a
 * type after an exercise already selected it; keeping these ids means editing
 * an unrelated field never drops the exercise's existing selection.
 */
export function orphanTypeIds(
  options: FileTypeOption[],
  selected: number[],
): number[] {
  const known = new Set(options.map((option) => option.value));
  return selected.filter((id) => !known.has(id));
}

/** The combobox's new selection, with any orphaned ids folded back in. */
export function selectionFromCombobox(
  values: FileTypeOption[],
  orphans: number[],
): number[] {
  return [...values.map((value) => value.value), ...orphans];
}
