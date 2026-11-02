import { RefreshCwIcon } from "lucide-react";

import { Section } from "@/components/app/Section";
import { ErrorState, LoadingState } from "@/components/app/states";
import { LanguageStatsReport } from "@/components/language-stats/LanguageStatsReport";
import { Button } from "@/components/ui/button";
import { useAsync } from "@/hooks/use-async";
import { getOfferingLanguageStatistics } from "@/lib/api";

/**
 * The class-scoped language report shown in the professor's Linguagens tab.
 * It has the same metrics as the admin page, but every denominator covers only
 * this class, and the report hides catalog entries the class never offered or
 * used.
 */
export function OfferingLanguageStatsPanel({
  offeringId,
}: {
  offeringId: number;
}) {
  const report = useAsync(
    () => getOfferingLanguageStatistics(offeringId),
    `offering-language-stats-${String(offeringId)}`,
    "Não foi possível carregar as estatísticas de linguagens desta turma.",
  );

  const data = report.data;

  return (
    <Section
      title="Linguagens da turma"
      description="Adoção, oferta e custo de CPU dentro desta turma. As taxas usam apenas as submissões e os exercícios daqui."
      action={
        <Button
          variant="outline"
          size="sm"
          disabled={report.loading}
          onClick={report.reload}
        >
          <RefreshCwIcon
            className={report.loading ? "animate-spin" : undefined}
          />
          Atualizar
        </Button>
      }
    >
      {report.loading && !data ? (
        <LoadingState label="Calculando o relatório da turma…" />
      ) : report.error ? (
        <ErrorState description={report.error} onRetry={report.reload} />
      ) : !data ? (
        <ErrorState
          description="Não foi possível carregar as estatísticas de linguagens desta turma."
          onRetry={report.reload}
        />
      ) : (
        <LanguageStatsReport data={data} scope="offering" />
      )}
    </Section>
  );
}
