import { Combobox as ComboboxPrimitive } from "@base-ui/react/combobox";
import { CheckIcon, ChevronDownIcon, XIcon } from "lucide-react";

import { LoadingState } from "@/components/app/states";
import {
  fileTypeOptions,
  orphanTypeIds,
  selectionFromCombobox,
  type FileTypeOption,
} from "@/components/professor/allowed-file-types";
import { Label } from "@/components/ui/label";
import { useAsync } from "@/hooks/use-async";
import { getAllowedFileTypes, type AllowedFileType } from "@/lib/api";

/**
 * The exercise's allowed submission types, as a searchable multi-select. The
 * options are the types the platform currently offers; a type the exercise
 * already selected stays listed after it stops being available, so editing
 * another field never drops it silently. Selecting none means the exercise
 * accepts every type the platform currently offers.
 */
export function AllowedFileTypesField({
  selected,
  onChange,
  idPrefix,
}: {
  selected: number[];
  onChange: (ids: number[]) => void;
  idPrefix: string;
}) {
  const types = useAsync(() => getAllowedFileTypes(), "allowed-file-types");
  const inputId = `${idPrefix}-allowed-file-types`;

  return (
    <div className="space-y-2">
      <Label htmlFor={inputId}>Tipos de arquivo permitidos</Label>
      <p className="text-xs text-muted-foreground">
        Deixe vazio para aceitar qualquer tipo disponível.
      </p>

      {types.loading ? (
        <LoadingState className="py-4" label="Carregando tipos…" />
      ) : types.error ? (
        <p className="text-sm text-destructive">{types.error}</p>
      ) : (
        <FileTypeCombobox
          id={inputId}
          types={types.data ?? []}
          selected={selected}
          onChange={onChange}
        />
      )}
    </div>
  );
}

function FileTypeCombobox({
  id,
  types,
  selected,
  onChange,
}: {
  id: string;
  types: AllowedFileType[];
  selected: number[];
  onChange: (ids: number[]) => void;
}) {
  const options = fileTypeOptions(types, selected);
  // Selected ids the API no longer lists: kept in the payload even though the
  // combobox cannot render them.
  const orphans = orphanTypeIds(options, selected);
  const value = options.filter((option) => selected.includes(option.value));

  if (options.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        Nenhum tipo de arquivo disponível no momento.
      </p>
    );
  }

  return (
    <ComboboxPrimitive.Root
      items={options}
      multiple
      value={value}
      onValueChange={(next: FileTypeOption[]) => {
        onChange(selectionFromCombobox(next, orphans));
      }}
      isItemEqualToValue={(a, b) => a.value === b.value}
      itemToStringLabel={(item) => item.label}
      autoHighlight
    >
      <ComboboxPrimitive.Chips className="flex min-h-9 w-full flex-wrap items-center gap-1 rounded-lg border border-input px-2 py-1 text-sm shadow-xs transition-colors focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50 dark:bg-input/30">
        <ComboboxPrimitive.Value>
          {(selectedOptions: FileTypeOption[]) => (
            <>
              {selectedOptions.map((option) => (
                <ComboboxPrimitive.Chip
                  key={option.value}
                  className="flex items-center gap-1 rounded-md bg-secondary py-0.5 pr-0.5 pl-1.5 text-xs text-secondary-foreground"
                >
                  {option.label}
                  <ComboboxPrimitive.ChipRemove
                    aria-label={`Remover ${option.label}`}
                    className="rounded-sm p-0.5 outline-none hover:bg-foreground/10 focus-visible:ring-[2px] focus-visible:ring-ring/50"
                  >
                    <XIcon className="size-3" />
                  </ComboboxPrimitive.ChipRemove>
                </ComboboxPrimitive.Chip>
              ))}
            </>
          )}
        </ComboboxPrimitive.Value>
        <ComboboxPrimitive.Input
          id={id}
          placeholder="Buscar tipo…"
          className="min-w-16 flex-1 bg-transparent py-0.5 text-sm outline-none placeholder:text-muted-foreground"
        />
        <ComboboxPrimitive.Trigger
          aria-label="Mostrar tipos de arquivo"
          className="flex shrink-0 items-center rounded-sm p-0.5 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-[2px] focus-visible:ring-ring/50"
        >
          <ComboboxPrimitive.Icon>
            <ChevronDownIcon className="size-4" />
          </ComboboxPrimitive.Icon>
        </ComboboxPrimitive.Trigger>
      </ComboboxPrimitive.Chips>

      <ComboboxPrimitive.Portal>
        <ComboboxPrimitive.Positioner
          className="isolate z-50 outline-none"
          align="start"
          sideOffset={4}
        >
          <ComboboxPrimitive.Popup className="max-h-(--available-height) w-(--anchor-width) origin-(--transform-origin) overflow-x-hidden overflow-y-auto rounded-lg bg-popover p-1 text-popover-foreground shadow-md ring-1 ring-foreground/10 duration-100 outline-none data-[side=bottom]:slide-in-from-top-2 data-[side=top]:slide-in-from-bottom-2 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95">
            {/* The Empty element must stay mounted (it is the live region);
                only its children toggle, so the padding lives on them — not on
                the wrapper, which would otherwise reserve a blank item's worth
                of space above the list. */}
            <ComboboxPrimitive.Empty className="text-sm text-muted-foreground">
              <p className="px-2 py-4 text-center">Nenhum tipo encontrado.</p>
            </ComboboxPrimitive.Empty>
            <ComboboxPrimitive.List>
              {(option: FileTypeOption) => (
                <ComboboxPrimitive.Item
                  key={option.value}
                  value={option}
                  className="flex cursor-default items-center justify-between gap-2 rounded-md px-2 py-1.5 text-sm outline-none select-none data-highlighted:bg-accent data-highlighted:text-accent-foreground"
                >
                  {option.label}
                  <ComboboxPrimitive.ItemIndicator className="flex shrink-0 items-center text-primary">
                    <CheckIcon className="size-4" />
                  </ComboboxPrimitive.ItemIndicator>
                </ComboboxPrimitive.Item>
              )}
            </ComboboxPrimitive.List>
          </ComboboxPrimitive.Popup>
        </ComboboxPrimitive.Positioner>
      </ComboboxPrimitive.Portal>
    </ComboboxPrimitive.Root>
  );
}
