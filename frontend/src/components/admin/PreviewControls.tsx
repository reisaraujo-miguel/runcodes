import { BookOpenIcon, GraduationCapIcon } from "lucide-react";

import { SectionCard } from "@/components/app/SectionCard";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useAuth } from "@/hooks/use-auth";
import { usePreview } from "@/hooks/use-preview";

/**
 * Role preview launcher for the admin dashboard. The account stays the admin's
 * own; the session simply uses a student or professor role, so the corresponding
 * navigation and limits are exercised without touching another user's account.
 */
export function PreviewControls() {
  const { user } = useAuth();
  const { pending, error, start } = usePreview();

  if (user?.role !== "admin" || user.previewing_role) return null;

  return (
    <SectionCard
      title="Visualizar como"
      description="Navegue pela plataforma com as permissões de estudante ou professor. Sua conta e seus dados continuam os mesmos; o painel administrativo fica oculto até você voltar."
    >
      <div className="space-y-3">
        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            disabled={pending !== null}
            onClick={() => {
              void start("student");
            }}
          >
            {pending === "student" ? (
              <Spinner className="size-4" />
            ) : (
              <BookOpenIcon />
            )}
            Visualizar como estudante
          </Button>
          <Button
            variant="outline"
            disabled={pending !== null}
            onClick={() => {
              void start("professor");
            }}
          >
            {pending === "professor" ? (
              <Spinner className="size-4" />
            ) : (
              <GraduationCapIcon />
            )}
            Visualizar como professor
          </Button>
        </div>
      </div>
    </SectionCard>
  );
}
