import { UsersIcon } from "lucide-react";

import { EmptyState } from "@/components/app/states";
import { Badge } from "@/components/ui/badge";
import type { LanguageStat } from "@/lib/api";

import { choiceLanguages, formatCount, formatPercent } from "./language-stats";

/**
 * What students actually choose. The share is over every classified student
 * submission; the second line also shows the choice rate inside the exercises
 * that allowed the language, which is the number to compare when a language is
 * offered less often.
 */
export function LanguageChoiceCard({
  languages,
  classifiedSubmissions,
  unclassifiedSubmissions,
}: {
  languages: readonly LanguageStat[];
  classifiedSubmissions: number;
  unclassifiedSubmissions: number;
}) {
  const rows = choiceLanguages(languages);

  return (
    <div className="space-y-3">
      {rows.length === 0 ? (
        <EmptyState
          icon={UsersIcon}
          title="Nenhuma submissão classificada"
          description="Quando os alunos enviarem soluções, a distribuição por linguagem aparece aqui."
        />
      ) : (
        <div className="max-h-[45dvh] space-y-5 overflow-y-auto pr-1">
          {rows.map((language) => (
            <article
              key={language.file_type_id}
              className="space-y-2"
              aria-label={`${language.name}: ${formatCount(language.submissions)} submissões`}
            >
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <span className="truncate font-medium">{language.name}</span>
                  <code className="bg-muted text-muted-foreground rounded px-1.5 py-0.5 font-mono text-xs">
                    .{language.extension}
                  </code>
                  <Badge
                    variant={language.is_compilable ? "info" : "secondary"}
                  >
                    {language.is_compilable ? "Compilada" : "Interpretada"}
                  </Badge>
                  {language.is_available ? null : (
                    <Badge variant="secondary">Oculta</Badge>
                  )}
                </div>
                <p className="text-sm tabular-nums">
                  <span className="font-semibold">
                    {formatCount(language.submissions)}
                  </span>{" "}
                  <span className="text-muted-foreground">
                    ({formatPercent(language.submission_share)})
                  </span>
                </p>
              </div>

              <div
                className="bg-muted h-2 overflow-hidden rounded-full"
                aria-hidden
              >
                <div
                  className="bg-primary h-full rounded-full"
                  style={{
                    width: `${String(language.submission_share)}%`,
                  }}
                />
              </div>

              <p className="text-muted-foreground text-xs leading-relaxed">
                {formatCount(language.students)}{" "}
                {language.students === 1 ? "aluno" : "alunos"} ·{" "}
                {formatCount(language.exercises_chosen)}{" "}
                {language.exercises_chosen === 1 ? "exercício" : "exercícios"}
                {language.pick_rate === null
                  ? " · sem entregas nos exercícios que a ofertam"
                  : ` · ${formatPercent(language.pick_rate)} das entregas quando ofertada, em ${formatCount(language.pick_rate_samples)} ${
                      language.pick_rate_samples === 1
                        ? "exercício"
                        : "exercícios"
                    }`}
              </p>
            </article>
          ))}
        </div>
      )}

      {unclassifiedSubmissions > 0 ? (
        <p className="text-muted-foreground text-xs leading-relaxed">
          {formatCount(unclassifiedSubmissions)}{" "}
          {unclassifiedSubmissions === 1
            ? "submissão não pôde ser classificada"
            : "submissões não puderam ser classificadas"}{" "}
          porque o exercício não tinha tipos de arquivo configurados ou a
          extensão não está mais no catálogo. Esses envios ficam fora das taxas
          acima.
        </p>
      ) : null}

      <p className="text-muted-foreground text-xs">
        Total classificado: {formatCount(classifiedSubmissions)}{" "}
        {classifiedSubmissions === 1 ? "submissão" : "submissões"}.
      </p>
    </div>
  );
}
