import { SectionCard } from "@/components/app/SectionCard";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { LanguageStat, PhaseStat } from "@/lib/api";
import { formatBytes } from "@/lib/format";

import { formatCount, formatDuration, formatPercent } from "./language-stats";

/**
 * The operational time breakdown. Medians of different phases do not add up to
 * a median total, so this is a table rather than a stacked bar. The compile
 * phase only applies to compiled languages; the image median excludes cold
 * pulls, which are reported separately.
 */
export function LanguagePhasesCard({
  languages,
}: {
  languages: readonly LanguageStat[];
}) {
  const rows = languages.toSorted((a, b) =>
    a.name.localeCompare(b.name, "pt-BR"),
  );

  return (
    <SectionCard
      title="Onde o tempo vai"
      description="Medianas por fase, medidas em execuções reais. As fases não somam necessariamente o tempo total; a compilação só se aplica a linguagens compiladas e pulls frios ficam fora da mediana da imagem."
    >
      <Table aria-label="Medianas de tempo por fase e linguagem">
        <TableHeader>
          <TableRow>
            <TableHead>Linguagem</TableHead>
            <TableHead
              className="text-center"
              title="Espera na fila até um worker pegar a submissão"
            >
              Fila
            </TableHead>
            <TableHead
              className="text-center"
              title="Download da fonte, arquivos e casos de teste"
            >
              Preparação
            </TableHead>
            <TableHead
              className="text-center"
              title="Verificação ou pull da imagem; mediana apenas de imagens já locais"
            >
              Imagem
            </TableHead>
            <TableHead
              className="text-center"
              title="Do start do contêiner até o primeiro marco da fase"
            >
              Startup
            </TableHead>
            <TableHead
              className="text-center"
              title="Tempo do comando de compilação no monitor"
            >
              Compilação
            </TableHead>
            <TableHead
              className="text-center"
              title="Do início ao fim da execução dos casos"
            >
              Execução
            </TableHead>
            <TableHead
              className="text-center"
              title="Leitura e comparação dos resultados"
            >
              Correção
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((language) => {
            const timings = language.timings;
            const image = timings?.image;
            const compile = timings?.compile;
            const compileCPU = compile?.cpu_time_median_seconds;
            const compileMemory = compile?.mem_peak_bytes;
            const compileDetail = [
              compileCPU != null ? `CPU ${formatDuration(compileCPU)}` : null,
              compileMemory != null
                ? `RAM ${formatBytes(compileMemory)}`
                : null,
            ]
              .filter(Boolean)
              .join(" · ");

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
                <TableCell className="text-center">
                  <PhaseCell stat={timings?.queue} />
                </TableCell>
                <TableCell className="text-center">
                  <PhaseCell stat={timings?.workspace} />
                </TableCell>
                <TableCell className="text-center">
                  <PhaseCell
                    stat={image}
                    detail={image ? imageDetail(image) : undefined}
                  />
                </TableCell>
                <TableCell className="text-center">
                  <PhaseCell stat={timings?.startup} />
                </TableCell>
                <TableCell className="text-center">
                  <PhaseCell
                    stat={compile}
                    applies={language.is_compilable}
                    detail={compileDetail || undefined}
                  />
                </TableCell>
                <TableCell className="text-center">
                  <PhaseCell stat={timings?.run} />
                </TableCell>
                <TableCell className="text-center">
                  <PhaseCell stat={timings?.grade} />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>

      <p className="text-muted-foreground mt-3 text-xs leading-relaxed">
        A porcentagem é a cobertura: execuções com a fase medida sobre as
        execuções classificadas no escopo. Submissões antigas não têm fases e
        aparecem como “sem dados”. O tempo de compilação vem do monitor, não da
        diferença entre eventos.
      </p>
    </SectionCard>
  );
}

function PhaseCell({
  stat,
  applies = true,
  detail,
}: {
  stat?: PhaseStat;
  applies?: boolean;
  detail?: string;
}) {
  if (!applies) {
    return <span className="text-muted-foreground text-xs">não se aplica</span>;
  }
  if (stat?.median_seconds == null) {
    return (
      <div className="space-y-0.5">
        <p className="text-muted-foreground text-xs">sem dados</p>
        {detail ? (
          <p className="text-muted-foreground text-[11px]">{detail}</p>
        ) : null}
      </div>
    );
  }

  return (
    <div className="space-y-0.5">
      <p className="font-medium tabular-nums">
        {formatDuration(stat.median_seconds)}
      </p>
      <p className="text-muted-foreground text-[11px] tabular-nums">
        {formatCount(stat.samples)} amostras · {formatPercent(stat.coverage)}
      </p>
      {detail ? (
        <p className="text-muted-foreground text-[11px]">{detail}</p>
      ) : null}
    </div>
  );
}

function imageDetail(stat: PhaseStat): string | undefined {
  const cold = stat.cold_samples ?? 0;
  if (cold === 0) return undefined;

  const warm = stat.warm_samples ?? stat.samples - cold;
  return `${formatCount(warm)} quentes · ${formatCount(cold)} ${
    cold === 1 ? "frio" : "frios"
  }`;
}
