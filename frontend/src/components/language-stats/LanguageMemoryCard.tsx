import { BarChart3Icon, InfoIcon } from "lucide-react";

import { EmptyState } from "@/components/app/states";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { LanguageStat } from "@/lib/api";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

import {
  formatCaseCount,
  formatCount,
  formatPercent,
  formatRatio,
  formatRatioRange,
  isSmallSample,
  memoryLanguages,
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
 * The paired peak-memory comparison. It uses the same per-test-case baseline as
 * the CPU index, but only known values: the monitor records -1 when the cgroup
 * does not expose memory.current, and those results must not be treated as zero.
 */
export function LanguageMemoryCard({
  languages,
}: {
  languages: readonly LanguageStat[];
}) {
  const rows = memoryLanguages(languages);
  const omitted = languages.length - rows.length;

  return (
    <div className="space-y-3">
      <Alert variant="info">
        <InfoIcon />
        <AlertTitle>Como a comparação controla as diferenças</AlertTitle>
        <AlertDescription>
          <p>
            Cada caso de teste é comparado com ele mesmo, como no índice de CPU:
            mediana de cada linguagem no caso, mediana entre as linguagens, e a
            mediana dessas razões ao longo dos casos. Só entram execuções com
            memória conhecida; a cobertura ao lado mostra quanto do total isso
            representa.
          </p>
          <p>
            O monitor amostra o cgroup a cada 10 ms. O valor inclui o runtime e
            o cache de páginas do contêiner, e um pico mais curto que o
            intervalo de amostragem pode passar despercebido. Leia como uma
            comparação entre linguagens, não como RSS puro do programa.
          </p>
        </AlertDescription>
      </Alert>

      <div className="text-muted-foreground mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
        <span className="flex items-center gap-1.5">
          <span className="bg-success size-2 rounded-full" aria-hidden />
          menos memória
        </span>
        <span className="flex items-center gap-1.5">
          <span className="bg-primary size-2 rounded-full" aria-hidden />
          na mediana
        </span>
        <span className="flex items-center gap-1.5">
          <span className="bg-warning size-2 rounded-full" aria-hidden />
          mais memória
        </span>
        <span className="text-muted-foreground/80">
          a linha central marca 1,00×
        </span>
      </div>

      {rows.length === 0 ? (
        <EmptyState
          icon={BarChart3Icon}
          title="Nenhuma medição de memória ainda"
          description="As execuções antigas têm memória desconhecida (-1). A tabela aparece quando houver resultados com o pico medido."
        />
      ) : (
        <Table aria-label="Custo relativo de memória por linguagem">
          <TableHeader>
            <TableRow>
              <TableHead>Linguagem</TableHead>
              <TableHead className="min-w-40">Índice de memória</TableHead>
              <TableHead className="text-center">Faixa p25–p75</TableHead>
              <TableHead className="text-center">Casos comparados</TableHead>
              <TableHead className="text-center">Mediana por caso</TableHead>
              <TableHead className="text-center">Cobertura</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((language) => {
              const memory = language.memory;
              if (!memory) return null;

              return (
                <TableRow key={language.file_type_id}>
                  <TableCell>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{language.name}</span>
                      <code className="bg-muted text-muted-foreground rounded px-1.5 py-0.5 font-mono text-xs">
                        .{language.extension}
                      </code>
                    </div>
                  </TableCell>
                  <TableCell>
                    {memory.relative === null ? (
                      <span className="text-muted-foreground text-sm">—</span>
                    ) : (
                      <div className="flex items-center gap-3">
                        <div
                          className="bg-muted relative h-2 w-24 shrink-0 overflow-hidden rounded-full"
                          aria-hidden
                        >
                          <div
                            className={cn(
                              "absolute inset-y-0 left-0 rounded-full",
                              BAR_TONE[relativeTone(memory.relative)],
                            )}
                            style={{
                              width: `${String(
                                relativeBarWidth(memory.relative),
                              )}%`,
                            }}
                          />
                          <div className="bg-foreground/30 absolute inset-y-0 left-1/2 w-px" />
                        </div>
                        <span
                          className={cn(
                            "font-medium tabular-nums",
                            TEXT_TONE[relativeTone(memory.relative)],
                          )}
                        >
                          {formatRatio(memory.relative)}
                        </span>
                      </div>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-center text-xs tabular-nums">
                    {formatRatioRange(memory.p25, memory.p75)}
                  </TableCell>
                  <TableCell className="text-center">
                    {memory.compared_test_cases === 0 ? (
                      <span className="text-muted-foreground text-sm">—</span>
                    ) : (
                      <span
                        className={cn(
                          "tabular-nums",
                          isSmallSample(memory.compared_test_cases) &&
                            "text-warning font-medium",
                        )}
                        title={
                          isSmallSample(memory.compared_test_cases)
                            ? "Amostra pequena: interprete com cautela."
                            : undefined
                        }
                      >
                        {formatCaseCount(memory.compared_test_cases)}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className="text-center tabular-nums">
                    {formatBytes(memory.median_bytes ?? -1)}
                  </TableCell>
                  <TableCell className="text-center text-xs tabular-nums">
                    {formatPercent(memory.coverage)}
                    <span className="text-muted-foreground block">
                      {formatCount(memory.known_results)}/
                      {formatCount(memory.total_results)}
                    </span>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}

      {omitted > 0 ? (
        <p className="text-muted-foreground text-xs">
          {formatCount(omitted)}{" "}
          {omitted === 1 ? "linguagem ficou" : "linguagens ficaram"} de fora por
          não ter nenhuma execução com memória conhecida.
        </p>
      ) : null}

      <p className="text-muted-foreground text-xs leading-relaxed">
        Cobertura = resultados de casos de teste com memória medida sobre todos
        os resultados classificados no escopo. Resultados antigos com{" "}
        <code>mem_usage = -1</code> são desconhecidos e não entram no índice.
      </p>
    </div>
  );
}
