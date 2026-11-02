import {
  AlertTriangleIcon,
  BarChart3Icon,
  BookOpenIcon,
  FileCode2Icon,
  GraduationCapIcon,
  InfoIcon,
} from "lucide-react";

import { StatCard } from "@/components/app/StatCard";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import type { LanguageStatistics } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

import { LanguageAllowanceCard } from "./LanguageAllowanceCard";
import { LanguageChoiceCard } from "./LanguageChoiceCard";
import { LanguagePerformanceCard } from "./LanguagePerformanceCard";
import { formatCount } from "./language-stats";

/**
 * The shared report behind the platform-wide admin page and the class-scoped
 * professor tab. The metrics are the same; only the scope of the denominators
 * changes. A class report hides catalog entries that the class neither offers
 * nor used, so the tables stay about that class.
 */
export function LanguageStatsReport({
  data,
  scope,
}: {
  data: LanguageStatistics;
  scope: "platform" | "offering";
}) {
  const offering = scope === "offering";
  const languages = offering
    ? data.languages.filter(
        (language) =>
          language.exercises_offered > 0 ||
          language.submissions > 0 ||
          language.results > 0,
      )
    : data.languages;

  return (
    <div className="space-y-6">
      <Alert variant="info">
        <InfoIcon />
        <AlertTitle>Três perguntas, três denominadores</AlertTitle>
        <AlertDescription>
          <p>
            A escolha conta apenas submissões de alunos
            {offering ? " desta turma" : ""}; a oferta conta exercícios não
            removidos e professores distintos
            {offering ? " desta turma" : ""}; o tempo de CPU compara cada
            linguagem com as outras que rodaram o mesmo caso de teste
            {offering ? " nesta turma" : ""}. Nenhum número usa o total de outro
            para parecer maior. Exercícios removidos ficam fora de todas as
            visões.
          </p>
          <p>
            O relatório segue o catálogo de tipos de arquivo: ZIP aparece como
            Makefile e formatos não executáveis, como PDF, aparecem se algum
            exercício os permitir.
          </p>
        </AlertDescription>
      </Alert>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          icon={FileCode2Icon}
          label="Submissões classificadas"
          value={formatCount(data.summary.classified_submissions)}
          hint={`${formatCount(data.summary.students)} ${
            data.summary.students === 1 ? "aluno" : "alunos"
          }`}
        />
        <StatCard
          icon={BarChart3Icon}
          label="Linguagens escolhidas"
          value={data.summary.languages_used}
          hint={`de ${formatCount(data.summary.catalog_languages)} no catálogo`}
          tone="info"
        />
        <StatCard
          icon={BookOpenIcon}
          label="Exercícios"
          value={formatCount(data.summary.exercises)}
          hint={`${formatCount(data.summary.languages_offered)} linguagens ofertadas`}
          tone="success"
        />
        <StatCard
          icon={GraduationCapIcon}
          label="Professores"
          value={formatCount(data.summary.professors)}
          hint="autores de exercícios"
          tone="warning"
        />
      </div>

      {data.summary.unclassified_submissions > 0 ? (
        <Alert variant="warning">
          <AlertTriangleIcon />
          <AlertTitle>Submissões sem classificação</AlertTitle>
          <AlertDescription>
            {formatCount(data.summary.unclassified_submissions)}{" "}
            {data.summary.unclassified_submissions === 1
              ? "submissão ficou"
              : "submissões ficaram"}{" "}
            fora das taxas por linguagem porque o exercício não tinha tipos de
            arquivo configurados ou a extensão não está mais no catálogo. Esses
            envios continuam no histórico do aluno.
          </AlertDescription>
        </Alert>
      ) : null}

      <LanguageChoiceCard
        languages={languages}
        classifiedSubmissions={data.summary.classified_submissions}
        unclassifiedSubmissions={data.summary.unclassified_submissions}
      />

      <LanguageAllowanceCard
        languages={languages}
        totalExercises={data.summary.exercises}
        totalProfessors={data.summary.professors}
        showOfferings={!offering}
      />

      <LanguagePerformanceCard languages={languages} />

      <p className="text-muted-foreground text-xs">
        Relatório calculado em {formatDateTime(data.generated_at)}. O resultado
        fica em cache por até um minuto; envios novos podem levar até um minuto
        para aparecer.
      </p>
    </div>
  );
}
