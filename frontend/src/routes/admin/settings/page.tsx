import { SettingsIcon } from "lucide-react";

import { SettingsForm } from "@/components/admin/SettingsForm";
import { PageHeader } from "@/components/app/PageHeader";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { useAsync } from "@/hooks/use-async";
import { adminGetSettings } from "@/lib/api";

/**
 * Edits the platform-wide contact settings shown on the login page: the contact
 * address and the optional HTML disclaimer rendered below it.
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
        description="Dados de contato exibidos publicamente na página de login."
      />

      <div className="max-w-2xl">
        <SectionCard
          title="Contato"
          description="O email e o aviso mostrados na página de login."
        >
          {settings.loading ? (
            <LoadingState />
          ) : settings.error ? (
            <ErrorState
              description={settings.error}
              onRetry={settings.reload}
            />
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
    </div>
  );
}
