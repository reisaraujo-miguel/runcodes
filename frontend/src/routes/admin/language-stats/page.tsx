import { FileCode2Icon, RefreshCwIcon } from "lucide-react";
import { Link } from "react-router";

import { PageHeader } from "@/components/app/PageHeader";
import { ErrorState, LoadingState } from "@/components/app/states";
import { LanguageStatsReport } from "@/components/language-stats/LanguageStatsReport";
import { Button } from "@/components/ui/button";
import { useAsync } from "@/hooks/use-async";
import { adminGetLanguageStatistics } from "@/lib/api";

/**
 * The platform-wide admin language report. The metrics and their fairness
 * caveats live in LanguageStatsReport, which the class page reuses with a
 * different scope.
 */
export function AdminLanguageStatsPage() {
  const report = useAsync(
    () => adminGetLanguageStatistics(),
    "admin-language-stats",
    "Não foi possível carregar as estatísticas de linguagens.",
  );

  const data = report.data;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Linguagens"
        description="O que os alunos escolhem, o que os professores permitem e quanto tempo de CPU cada linguagem usa."
        actions={
          <>
            <Button
              variant="outline"
              size="sm"
              render={<Link to="/admin/file-types" />}
            >
              <FileCode2Icon />
              Tipos de arquivo
            </Button>
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
          </>
        }
      />

      {report.loading && !data ? (
        <LoadingState label="Calculando o relatório de linguagens…" />
      ) : report.error ? (
        <ErrorState description={report.error} onRetry={report.reload} />
      ) : !data ? (
        <ErrorState
          description="Não foi possível carregar as estatísticas de linguagens."
          onRetry={report.reload}
        />
      ) : (
        <LanguageStatsReport data={data} scope="platform" />
      )}
    </div>
  );
}
