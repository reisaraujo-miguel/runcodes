import { LoadingState } from "@/components/app/states";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { useAsync } from "@/hooks/use-async";
import { getAllowedFileTypes } from "@/lib/api";

/**
 * The exercise's allowed submission types, as a checkbox group. A currently
 * selected type stays listed even when it is no longer available, so editing
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

  function toggle(id: number) {
    onChange(
      selected.includes(id)
        ? selected.filter((value) => value !== id)
        : [...selected, id],
    );
  }

  const selectable = (types.data ?? []).filter(
    (type) => type.is_available || selected.includes(type.id),
  );

  return (
    <fieldset className="space-y-2">
      <legend className="text-sm font-medium">Tipos de arquivo permitidos</legend>
      <p className="text-muted-foreground text-xs">
        Deixe todos desmarcados para aceitar qualquer tipo disponível.
      </p>

      {types.loading ? (
        <LoadingState className="py-4" label="Carregando tipos…" />
      ) : types.error ? (
        <p className="text-destructive text-sm">{types.error}</p>
      ) : selectable.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          Nenhum tipo de arquivo disponível no momento.
        </p>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2">
          {selectable.map((type) => (
            <Label
              key={type.id}
              htmlFor={`${idPrefix}-type-${String(type.id)}`}
              className="font-normal"
            >
              <Checkbox
                id={`${idPrefix}-type-${String(type.id)}`}
                checked={selected.includes(type.id)}
                onCheckedChange={() => {
                  toggle(type.id);
                }}
              />
              {type.name} ({type.extension})
            </Label>
          ))}
        </div>
      )}
    </fieldset>
  );
}
