import { CodeIcon, Loader2Icon } from "lucide-react";

import { SectionCard } from "@/components/app/SectionCard";
import { Checkbox } from "@/components/ui/checkbox";
import { useCodeEditorPreference } from "@/hooks/use-code-editor-preference";

/**
 * Profile preference for the in-browser code editor. It is saved immediately,
 * because it is a standalone toggle rather than a field of the account form.
 */
export function EditorPreferenceCard() {
  const preference = useCodeEditorPreference();

  return (
    <SectionCard
      title="Preferências"
      description="Escolha como você prefere enviar as suas soluções."
    >
      <div className="space-y-3">
        <label className="flex cursor-pointer items-start gap-3">
          <Checkbox
            className="mt-0.5"
            checked={preference.enabled}
            disabled={preference.saving || preference.loading}
            onCheckedChange={(checked) => {
              void preference.setEnabled(checked);
            }}
          />
          <span className="space-y-1">
            <span className="flex items-center gap-2 text-sm font-medium">
              <CodeIcon className="size-4" aria-hidden />
              Usar editor de código na página do exercício
            </span>
            <span className="text-muted-foreground block text-sm">
              Quando ativado, você pode escrever e enviar a solução diretamente
              na página do exercício. Desative para enviar apenas um arquivo.
            </span>
          </span>
          {preference.saving ? (
            <Loader2Icon
              className="text-muted-foreground mt-1 size-4 animate-spin"
              aria-label="Salvando preferência"
            />
          ) : null}
        </label>

        {preference.error ? (
          <p className="text-destructive text-sm">{preference.error}</p>
        ) : null}
      </div>
    </SectionCard>
  );
}
