import { useState } from "react";

import {
  BarChart3Icon,
  DownloadIcon,
  GraduationCapIcon,
  TrendingUpIcon,
  UsersIcon,
} from "lucide-react";

import { Section } from "@/components/app/Section";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { StatStrip } from "@/components/app/StatStrip";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAsync } from "@/hooks/use-async";
import { downloadOfferingGradesCsv, getOfferingGradebook } from "@/lib/api";
import { downloadBlob } from "@/lib/download";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";

function scoreLabel(value: number | null): string {
  return value === null ? "—" : value.toFixed(2);
}

/**
 * The professor's grade view of one class: the class metrics, the table of every
 * student against every exercise, the per-exercise statistics and the CSV
 * export. The grade on an exercise is the best score among the student's
 * submissions; the class grade averages those best scores over every exercise,
 * counting a missing submission as zero.
 */
export function OfferingGradesPanel({ offeringId }: { offeringId: number }) {
  const gradebook = useAsync(
    () => getOfferingGradebook(offeringId),
    `offering-grades-${String(offeringId)}`,
    "Não foi possível carregar as notas desta turma.",
  );

  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState<string | null>(null);

  const data = gradebook.data;

  async function handleDownloadCsv() {
    setDownloading(true);
    setDownloadError(null);
    try {
      const file = await downloadOfferingGradesCsv(offeringId);
      downloadBlob(file.filename, file.blob);
    } catch (error) {
      setDownloadError(
        errorMessage(error, "Não foi possível baixar as notas."),
      );
    } finally {
      setDownloading(false);
    }
  }

  const csvButton = (
    <Button
      size="sm"
      variant="outline"
      disabled={downloading || !data}
      onClick={() => {
        void handleDownloadCsv();
      }}
    >
      <DownloadIcon />
      {downloading ? "Baixando…" : "Baixar CSV"}
    </Button>
  );

  const content =
    gradebook.loading && !data ? (
      <LoadingState />
    ) : gradebook.error ? (
      <ErrorState description={gradebook.error} onRetry={gradebook.reload} />
    ) : !data ? (
      <ErrorState
        description="Não foi possível carregar as notas desta turma."
        onRetry={gradebook.reload}
      />
    ) : (
      <div className="space-y-8">
        {downloadError ? (
          <Alert variant="destructive">
            <AlertDescription>{downloadError}</AlertDescription>
          </Alert>
        ) : null}

        <StatStrip
          items={[
            {
              icon: UsersIcon,
              label: "Alunos",
              value: data.summary.student_count,
              hint: `${String(data.summary.students_with_submission)} com entrega`,
            },
            {
              icon: GraduationCapIcon,
              label: "Média da turma",
              value: data.summary.global_average.toFixed(2),
              hint: "Média das notas finais",
              tone: "info",
            },
            {
              icon: TrendingUpIcon,
              label: "Acima de 50",
              value: data.summary.students_above_50,
              hint: "Alunos com média maior que 50",
              tone: "success",
            },
            {
              icon: BarChart3Icon,
              label: "Submissões",
              value: data.summary.total_submissions,
              hint: `${String(data.exercises.length)} exercício(s)`,
              tone: "warning",
            },
          ]}
        />

        <Section
          title="Tabela de notas"
          description="Melhor nota de cada aluno por exercício. A média final considera todos os exercícios, inclusive os sem entrega."
          headingLevel={3}
          className="space-y-3"
        >
          {data.students.length === 0 ? (
            <EmptyState
              title="Nenhum aluno matriculado"
              description="Quando houver alunos na turma, as notas aparecem aqui."
            />
          ) : data.exercises.length === 0 ? (
            <EmptyState
              title="Nenhum exercício publicado"
              description="Crie um exercício para começar a acompanhar as notas."
            />
          ) : (
            <Table containerClassName="max-h-[32rem]">
              <TableHeader className="sticky top-0 z-10">
                <TableRow>
                  <TableHead className="sticky left-0 z-20 min-w-56">
                    Aluno
                  </TableHead>
                  {data.exercises.map((exercise, index) => (
                    <TableHead
                      key={exercise.id}
                      className="min-w-28 text-center"
                      title={formatDateTime(exercise.deadline)}
                    >
                      <span className="block max-w-40 truncate">
                        {index + 1}. {exercise.title}
                      </span>
                    </TableHead>
                  ))}
                  <TableHead className="sticky right-0 z-20 text-center">
                    Média
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.students.map((student) => (
                  <TableRow key={student.user_id}>
                    <TableCell className="sticky left-0 z-10 min-w-56 bg-card">
                      <p className="font-medium">{student.name}</p>
                      <p className="text-xs text-muted-foreground">
                        {student.email}
                      </p>
                    </TableCell>
                    {student.scores.map((cell) => (
                      <TableCell
                        key={cell.exercise_id}
                        className="text-center tabular-nums"
                      >
                        {cell.score === null ? (
                          <span className="text-muted-foreground">—</span>
                        ) : (
                          <span
                            className={cn(
                              cell.score >= 50
                                ? "font-medium text-success"
                                : "font-medium text-destructive",
                            )}
                          >
                            {scoreLabel(cell.score)}
                          </span>
                        )}
                        {cell.submission_count > 1 ? (
                          <span className="block text-[10px] text-muted-foreground">
                            {cell.submission_count} envios
                          </span>
                        ) : null}
                      </TableCell>
                    ))}
                    <TableCell className="sticky right-0 z-10 bg-muted text-center font-semibold tabular-nums">
                      {scoreLabel(student.average)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Section>

        <Section
          title="Estatísticas por exercício"
          description="A média considera todos os alunos; a coluna “entre os que entregaram” ignora quem não submeteu."
          headingLevel={3}
          className="space-y-3"
        >
          {data.exercise_statistics.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Nenhum exercício para calcular estatísticas.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Exercício</TableHead>
                  <TableHead className="text-center">Entregaram</TableHead>
                  <TableHead className="text-center">Acima de 50</TableHead>
                  <TableHead className="text-center">Média</TableHead>
                  <TableHead className="text-center">
                    Média dos que entregaram
                  </TableHead>
                  <TableHead className="text-center">Aprovação</TableHead>
                  <TableHead className="text-center">Melhor / pior</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.exercise_statistics.map((statistics) => (
                  <TableRow key={statistics.exercise_id}>
                    <TableCell>
                      <p className="font-medium">{statistics.title}</p>
                      <p className="text-xs text-muted-foreground">
                        Prazo {formatDateTime(statistics.deadline)}
                      </p>
                    </TableCell>
                    <TableCell className="text-center tabular-nums">
                      {statistics.submitted_students}/{statistics.student_count}
                    </TableCell>
                    <TableCell className="text-center tabular-nums">
                      {statistics.students_above_50}
                    </TableCell>
                    <TableCell className="text-center tabular-nums">
                      {statistics.average.toFixed(2)}
                    </TableCell>
                    <TableCell className="text-center tabular-nums">
                      {statistics.average_submitted === null
                        ? "—"
                        : statistics.average_submitted.toFixed(2)}
                    </TableCell>
                    <TableCell className="text-center">
                      <Badge
                        variant={
                          statistics.approval_rate >= 50 ? "success" : "warning"
                        }
                      >
                        {statistics.approval_rate.toFixed(0)}%
                      </Badge>
                    </TableCell>
                    <TableCell className="text-center tabular-nums">
                      {scoreLabel(statistics.best_score)} /{" "}
                      {scoreLabel(statistics.lowest_score)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Section>
      </div>
    );

  return (
    <Section
      title="Notas e estatísticas"
      description="Acompanhe o desempenho da turma e exporte a tabela em CSV."
      action={csvButton}
    >
      {content}
    </Section>
  );
}
