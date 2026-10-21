import { useState, type SubmitEvent } from "react";

import { MarkupEditor } from "@/components/editor/MarkupEditor";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import { adminUpdateSettings, type PlatformSettings } from "@/lib/api";
import { cn } from "@/lib/utils";

const DISCLAIMER_MAX_LENGTH = 4000;

/** A pragmatic client-side check; the API remains the source of truth. */
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * The platform contact form: an address used in the login terms and in the
 * issue-reporting instructions on the error pages, plus the optional HTML
 * disclaimer rendered below the login form.
 */
export function SettingsForm({ settings }: { settings: PlatformSettings }) {
  const submit = useSubmit("Não foi possível salvar as configurações.");
  const [email, setEmail] = useState(settings.contact_email);
  const [disclaimer, setDisclaimer] = useState(
    settings.contact_disclaimer_html,
  );

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const trimmedEmail = email.trim();
    if (!EMAIL_PATTERN.test(trimmedEmail)) {
      submit.setError("Informe um email de contato válido.");
      return;
    }
    if (disclaimer.length > DISCLAIMER_MAX_LENGTH) {
      submit.setError(
        `O aviso pode ter no máximo ${String(DISCLAIMER_MAX_LENGTH)} caracteres.`,
      );
      return;
    }

    void submit.run(async () => {
      await adminUpdateSettings({
        contact_email: trimmedEmail,
        contact_disclaimer_html: disclaimer,
      });
    }, "Configurações salvas.");
  }

  return (
    <form className="space-y-4" onSubmit={handleSubmit}>
      <div className="space-y-2">
        <Label htmlFor="admin-settings-email">Email de contato</Label>
        <Input
          id="admin-settings-email"
          type="email"
          value={email}
          disabled={submit.pending}
          onChange={(event) => {
            setEmail(event.target.value);
          }}
          required
        />
        <p className="text-muted-foreground text-sm">
          Usado nos termos de uso e nas instruções de reporte de problemas
          exibidas nas páginas de erro.
        </p>
      </div>

      <div className="space-y-2">
        <p className="text-sm font-medium">Aviso de contato (HTML)</p>
        <MarkupEditor
          format="html"
          value={disclaimer}
          onChange={setDisclaimer}
          readOnly={submit.pending}
          heightClassName="h-56"
          ariaLabel="Aviso de contato (HTML)"
          placeholder="<p>Em caso de dúvidas, escreva para o contato acima.</p>"
        />
        <div className="flex items-center justify-between gap-2">
          <p className="text-muted-foreground text-sm">
            O HTML é renderizado na página de login e sanitizado no navegador
            antes de ser exibido. Deixe vazio para ocultar o aviso.
          </p>
          <span
            className={cn(
              "text-muted-foreground shrink-0 text-xs tabular-nums",
              disclaimer.length > DISCLAIMER_MAX_LENGTH && "text-destructive",
            )}
          >
            {disclaimer.length}/{DISCLAIMER_MAX_LENGTH}
          </span>
        </div>
      </div>

      {submit.error ? (
        <Alert variant="destructive">
          <AlertDescription>{submit.error}</AlertDescription>
        </Alert>
      ) : null}
      {submit.success ? (
        <Alert variant="success">
          <AlertDescription>{submit.success}</AlertDescription>
        </Alert>
      ) : null}

      <Button type="submit" disabled={submit.pending}>
        {submit.pending ? <Spinner className="size-4" /> : null}
        Salvar
      </Button>
    </form>
  );
}
