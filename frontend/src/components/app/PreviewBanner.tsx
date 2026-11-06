import { EyeIcon, Undo2Icon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useAuth } from "@/hooks/use-auth";
import { usePreview } from "@/hooks/use-preview";
import { platformRoleLabel } from "@/lib/roles";

/**
 * A persistent notice while an administrator previews the app with a reduced
 * role. The account is still the admin's own; only the role used for
 * authorization changed. The button restores the real admin session.
 */
export function PreviewBanner() {
  const { user } = useAuth();
  const { pending, error, stop } = usePreview();

  const previewRole = user?.previewing_role;
  if (!user || !previewRole) return null;

  return (
    <div
      role="status"
      className="mb-6 flex flex-col gap-3 rounded-xl border border-info/40 bg-info/10 p-3 text-sm sm:flex-row sm:items-center sm:justify-between"
    >
      <p className="flex items-start gap-2">
        <EyeIcon className="mt-0.5 size-4 shrink-0 text-info" aria-hidden />
        <span>
          Você está visualizando a plataforma como{" "}
          <strong className="font-medium">
            {platformRoleLabel(previewRole)}
          </strong>
          . Os recursos exclusivos de administrador ficam ocultos e o painel
          administrativo não pode ser acessado enquanto a pré-visualização
          estiver ativa.
          {error ? (
            <span className="mt-1 block text-destructive">{error}</span>
          ) : null}
        </span>
      </p>
      <Button
        type="button"
        size="sm"
        variant="outline"
        disabled={pending !== null}
        onClick={() => {
          void stop();
        }}
        className="shrink-0"
      >
        {pending === "stop" ? <Spinner className="size-4" /> : <Undo2Icon />}
        Voltar para administrador
      </Button>
    </div>
  );
}
