import { SettingsIcon } from "lucide-react";

import { SettingsForm } from "@/components/admin/SettingsForm";
import { PageHeader } from "@/components/app/PageHeader";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { useAsync } from "@/hooks/use-async";
import { adminGetSettings } from "@/lib/api";

/**
 * Edits the platform-wide contact settings: the address used in the login terms
 * and in the error pages' issue-reporting instructions, and the optional HTML
 * disclaimer shown on the login page.
 */
export function AdminSettingsPage() {
  const settings = useAsync(
    adminGetSettings,
    "admin-settings",
    "Não foi possível carregar as configurações.",
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="Configurações"
        description="Dados de contato usados na página de login e nas orientações de reporte de erros."
      />

      <SectionCard
        title="Contato"
        description="O email é usado nos termos de uso e nas instruções de reporte de erros; o aviso é exibido na página de login."
      >
        {settings.loading ? (
          <LoadingState />
        ) : settings.error ? (
          <ErrorState description={settings.error} onRetry={settings.reload} />
        ) : settings.data ? (
          <SettingsForm settings={settings.data} />
        ) : (
          <EmptyState
            icon={SettingsIcon}
            title="Sem configurações"
            description="Não há configurações para exibir."
          />
        )}
      </SectionCard>
    </div>
  );
}
