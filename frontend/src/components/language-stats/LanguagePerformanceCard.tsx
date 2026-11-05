import { Hint } from "@/components/app/Hint";
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
import { cn } from "@/lib/utils";

import {
  formatCaseCount,
  formatCount,
  formatMedianCpu,
  formatRatio,
  formatRatioRange,
  isSmallSample,
  performanceLanguages,
  relativeBarWidth,
  relativeTone,
  type RelativeTone,
} from "./language-stats";

const BAR_TONE: Record<RelativeTone, string> = {
  faster: "bg-success",
  typical: "bg-primary",
  slower: "bg-warning",
};

const TEXT_TONE: Record<RelativeTone, string> = {
  faster: "text-success",
  typical: "text-foreground",
  slower: "text-warning",
};

/**
 * The fair CPU comparison. Each test case is its own baseline: a language's
 * median is divided by the median of the languages that ran the same case, and
 * the language's index is the median of those ratios. A case with one language
 * is excluded because it offers no peer to compare against and would pull the
 * index towards 1.0 for languages used in many exclusive exercises.
 */
export function LanguagePerformanceCard({
  languages,
}: {
  languages: readonly LanguageStat[];
}) {
  const rows = performanceLanguages(languages);

  return (
    <div className="space-y-3">
      <Hint summary="Como a comparação controla as diferenças">
        <p>
          Cada caso de teste serve de régua para si mesmo. Para cada caso,
          calculamos a mediana de cada linguagem e a mediana entre as
          linguagens; o índice de uma linguagem é a mediana das razões ao longo
          dos casos. Assim, exercícios mais pesados e limites diferentes afetam
          todas as linguagens por igual.
        </p>
        <p>
          Casos em que só uma linguagem foi usada ficam de fora: não há com o
          que comparar. Linguagens com menos de cinco casos comparados aparecem
          com aviso de amostra pequena.
        </p>
        <p>
          Esta comparação usa apenas tempo de CPU; a memória tem uma tabela
          própria. Os alunos também não são distribuídos aleatoriamente entre
          linguagens: leia o índice como uma pista, não como um benchmark
          controlado.
        </p>
      </Hint>

      <div className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <span className="size-2 rounded-full bg-success" aria-hidden />
          mais rápido
        </span>
        <span className="flex items-center gap-1.5">
          <span className="size-2 rounded-full bg-primary" aria-hidden />
          na mediana
        </span>
        <span className="flex items-center gap-1.5">
          <span className="size-2 rounded-full bg-warning" aria-hidden />
          mais lento
        </span>
        <span className="text-muted-foreground/80">
          a linha central marca 1,00×
        </span>
      </div>

      <Table aria-label="Custo relativo de CPU por linguagem">
        <TableHeader>
          <TableRow>
            <TableHead>Linguagem</TableHead>
            <TableHead className="min-w-40">Índice de CPU</TableHead>
            <TableHead className="text-center">Faixa p25–p75</TableHead>
            <TableHead className="text-center">Casos comparados</TableHead>
            <TableHead className="text-center">Mediana por caso</TableHead>
            <TableHead className="text-center">Execuções</TableHead>
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
                {language.relative_cpu === null ? (
                  <span className="text-sm text-muted-foreground">—</span>
                ) : (
                  <div className="flex items-center gap-3">
                    <div
                      className="relative h-2 w-24 shrink-0 overflow-hidden rounded-full bg-muted"
                      aria-hidden
                    >
                      <div
                        className={cn(
                          "absolute inset-y-0 left-0 rounded-full",
                          BAR_TONE[relativeTone(language.relative_cpu)],
                        )}
                        style={{
                          width: `${String(
                            relativeBarWidth(language.relative_cpu),
                          )}%`,
                        }}
                      />
                      <div className="absolute inset-y-0 left-1/2 w-px bg-foreground/30" />
                    </div>
                    <span
                      className={cn(
                        "font-medium tabular-nums",
                        TEXT_TONE[relativeTone(language.relative_cpu)],
                      )}
                    >
                      {formatRatio(language.relative_cpu)}
                    </span>
                  </div>
                )}
              </TableCell>
              <TableCell className="text-center text-xs text-muted-foreground tabular-nums">
                {formatRatioRange(
                  language.relative_cpu_p25,
                  language.relative_cpu_p75,
                )}
              </TableCell>
              <TableCell className="text-center">
                {language.compared_test_cases === 0 ? (
                  <span className="text-sm text-muted-foreground">—</span>
                ) : (
                  <span
                    className={cn(
                      "tabular-nums",
                      isSmallSample(language.compared_test_cases) &&
                        "font-medium text-warning",
                    )}
                    title={
                      isSmallSample(language.compared_test_cases)
                        ? "Amostra pequena: interprete com cautela."
                        : undefined
                    }
                  >
                    {formatCaseCount(language.compared_test_cases)}
                  </span>
                )}
              </TableCell>
              <TableCell className="text-center tabular-nums">
                {formatMedianCpu(language.cpu_time_median_seconds)}
              </TableCell>
              <TableCell className="text-center tabular-nums">
                {formatCount(language.results)}
                <span className="block text-xs text-muted-foreground">
                  {formatCount(language.test_cases)}{" "}
                  {language.test_cases === 1 ? "caso" : "casos"}
                </span>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <p className="text-xs leading-relaxed text-muted-foreground">
        “Execuções” conta todos os resultados de casos de teste da linguagem;
        “casos comparados” conta só os que entraram no índice. A mediana por
        caso usa os casos comparados, não os exercícios mais pesados
        isoladamente. A tabela coloca primeiro as linguagens com pelo menos
        cinco casos comparados; amostras pequenas e sem dados vêm depois.
      </p>
    </div>
  );
}
