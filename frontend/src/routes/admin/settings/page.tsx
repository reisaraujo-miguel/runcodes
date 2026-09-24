import { useEffect, useState, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  adminGetSettings,
  adminUpdateSettings,
  type PlatformSettings,
} from "@/lib/api";

/** The API message of a failed request, or a fallback for unexpected errors. */
function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

/**
 * Admin page that edits the platform-wide contact settings shown on the login
 * page: the contact address and the disclaimer rendered below the form.
 */
export function AdminSettingsPage() {
  const [settings, setSettings] = useState<PlatformSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [email, setEmail] = useState("");
  const [disclaimer, setDisclaimer] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadSettings() {
      try {
        const data = await adminGetSettings();
        if (cancelled) return;
        setSettings(data);
        setEmail(data.contact_email);
        setDisclaimer(data.contact_disclaimer_html);
        setLoadError(null);
      } catch (error) {
        if (!cancelled) {
          setLoadError(
            errorMessage(error, "Não foi possível carregar as configurações."),
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadSettings();

    return () => {
      cancelled = true;
    };
  }, []);

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const contactEmail = email.trim();
    if (contactEmail === "") {
      setError("O email de contato é obrigatório.");
      setSuccess(null);
      return;
    }

    setSubmitting(true);
    setError(null);
    setSuccess(null);

    void (async () => {
      try {
        const updated = await adminUpdateSettings({
          contact_email: contactEmail,
          contact_disclaimer_html: disclaimer,
        });
        setSettings(updated);
        setEmail(updated.contact_email);
        setDisclaimer(updated.contact_disclaimer_html);
        setSuccess("Configurações salvas.");
      } catch (submitError) {
        setError(errorMessage(submitError, "Erro ao salvar as configurações."));
      } finally {
        setSubmitting(false);
      }
    })();
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-5xl space-y-4 p-6">
        <div className="flex justify-center p-12">
          <div
            aria-label="Carregando"
            className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
            role="status"
          />
        </div>
      </div>
    );
  }

  if (settings === null) {
    return (
      <div className="mx-auto max-w-5xl space-y-4 p-6">
        <Card>
          <CardHeader>
            <CardTitle>Não foi possível carregar as configurações</CardTitle>
            <CardDescription>
              {loadError ?? "Tente novamente mais tarde."}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl space-y-4 p-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">Configurações</CardTitle>
          <CardDescription>
            Dados de contato exibidos na página de login.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <Label htmlFor="admin-settings-email">Email de contato</Label>
              <Input
                id="admin-settings-email"
                type="email"
                value={email}
                disabled={submitting}
                onChange={(event) => {
                  setEmail(event.target.value);
                }}
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="admin-settings-disclaimer">
                Aviso de contato (HTML)
              </Label>
              <Textarea
                id="admin-settings-disclaimer"
                value={disclaimer}
                disabled={submitting}
                className="min-h-40"
                onChange={(event) => {
                  setDisclaimer(event.target.value);
                }}
                placeholder="<p>Em caso de dúvidas, escreva para o contato acima.</p>"
              />
              <p className="text-muted-foreground text-sm">
                O HTML é renderizado na página de login e sanitizado no
                navegador antes de ser exibido.
              </p>
            </div>

            {error && <p className="text-destructive text-sm">{error}</p>}
            {success && (
              <p className="text-sm text-emerald-600 dark:text-emerald-400">
                {success}
              </p>
            )}

            <Button type="submit" disabled={submitting}>
              {submitting ? "Salvando…" : "Salvar"}
            </Button>
          </form>

          <p className="text-muted-foreground text-sm">
            Estes dois valores substituem os padrões definidos no build
            (`VITE_CONTACT_EMAIL` e `VITE_CONTACT_DISCLAIMER_HTML`), usados
            apenas enquanto a API não responde.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
