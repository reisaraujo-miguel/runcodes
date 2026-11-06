import { BookOpenIcon } from "lucide-react";

import { EmptyState } from "@/components/app/states";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { LanguageStat } from "@/lib/api";

import {
  allowanceLanguages,
  formatCount,
  formatPercent,
} from "./language-stats";

/**
 * What professors allow. The exercise bar is the percentage of every non-removed
 * exercise that lists the entry; the professor count is distinct exercise
 * authors, not a count of exercises they authored.
 */
export function LanguageAllowanceCard({
  languages,
  totalExercises,
  totalProfessors,
  showOfferings = true,
}: {
  languages: readonly LanguageStat[];
  totalExercises: number;
  totalProfessors: number;
  /** Hidden in the class report, where every offered language belongs to the same class. */
  showOfferings?: boolean;
}) {
  const rows = allowanceLanguages(languages);

  return (
    <div className="space-y-3">
      {rows.length === 0 ? (
        <EmptyState
          icon={BookOpenIcon}
          title="Nenhum tipo de arquivo no catálogo"
          description="Cadastre uma linguagem em Tipos de Arquivo para começar."
        />
      ) : (
        <Table aria-label="Oferta de cada linguagem pelos professores">
          <TableHeader>
            <TableRow>
              <TableHead>Linguagem</TableHead>
              <TableHead className="min-w-48">
                Exercícios que permitem
              </TableHead>
              <TableHead className="text-center">Professores</TableHead>
              {showOfferings ? (
                <TableHead className="text-center">Turmas</TableHead>
              ) : null}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((language) => (
              <TableRow key={language.file_type_id}>
                <TableCell>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{language.name}</span>
                    <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground">
                      .{language.extension}
                    </code>
                    {language.is_available ? null : (
                      <Badge variant="secondary">Oculta</Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-3">
                    <div
                      className="h-1.5 min-w-16 flex-1 overflow-hidden rounded-full bg-muted"
                      aria-hidden
                    >
                      <div
                        className="h-full rounded-full bg-info"
                        style={{
                          width: `${String(language.exercise_offer_rate)}%`,
                        }}
                      />
                    </div>
                    <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                      {formatCount(language.exercises_offered)} de{" "}
                      {formatCount(totalExercises)} (
                      {formatPercent(language.exercise_offer_rate)})
                    </span>
                  </div>
                </TableCell>
                <TableCell className="text-center tabular-nums">
                  {formatCount(language.professors_offering)}
                  <span className="block text-xs text-muted-foreground">
                    {formatPercent(language.professor_offer_rate)} de{" "}
                    {formatCount(totalProfessors)}
                  </span>
                </TableCell>
                {showOfferings ? (
                  <TableCell className="text-center tabular-nums">
                    {formatCount(language.offerings_offered)}
                  </TableCell>
                ) : null}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <p className="text-xs leading-relaxed text-muted-foreground">
        Exercícios removidos ficam fora da contagem. Uma linguagem oculta
        continua aparecendo se algum exercício já a usa, mas não pode ser
        escolhida em novos exercícios nem em novas submissões.
      </p>
    </div>
  );
}
