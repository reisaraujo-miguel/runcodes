import { useState, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { changePassword } from "@/lib/api/profile";

/** The password rules the backend enforces when a password is set. */
const PASSWORD_HINT =
  "A senha deve ter no mínimo 8 caracteres, com pelo menos uma letra maiúscula, uma letra minúscula, um dígito e um caractere especial.";

/** A non-alphanumeric character, matching the set the backend accepts. */
const SPECIAL_CHARACTER = /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?~`]/;

/** Mirrors validation.ValidatePassword on the backend. */
function isValidPassword(password: string): boolean {
  return (
    password.length >= 8 &&
    /[A-Z]/.test(password) &&
    /[a-z]/.test(password) &&
    /[0-9]/.test(password) &&
    SPECIAL_CHARACTER.test(password)
  );
}

/** Card that replaces the caller's password after checking the current one. */
export function PasswordForm() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSuccess(null);

    if (!currentPassword) {
      setError("Informe a sua senha atual");
      return;
    }
    if (newPassword !== confirmation) {
      setError("As senhas não coincidem");
      return;
    }
    if (!isValidPassword(newPassword)) {
      setError("A nova senha não atende aos requisitos de segurança");
      return;
    }

    setSubmitting(true);

    void (async () => {
      try {
        await changePassword({
          current_password: currentPassword,
          new_password: newPassword,
          new_password_confirmation: confirmation,
        });
        setCurrentPassword("");
        setNewPassword("");
        setConfirmation("");
        setSuccess("Senha alterada com sucesso.");
      } catch (submitError) {
        setError(
          submitError instanceof Error
            ? submitError.message
            : "Erro ao alterar a senha",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Alterar senha</CardTitle>
        <CardDescription>
          Escolha uma nova senha para a sua conta.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="current-password">Senha atual</Label>
            <PasswordInput
              id="current-password"
              value={currentPassword}
              onChange={(event) => {
                setCurrentPassword(event.target.value);
              }}
              autoComplete="current-password"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="new-password">Nova senha</Label>
            <PasswordInput
              id="new-password"
              value={newPassword}
              onChange={(event) => {
                setNewPassword(event.target.value);
              }}
              autoComplete="new-password"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="new-password-confirmation">
              Confirmar nova senha
            </Label>
            <PasswordInput
              id="new-password-confirmation"
              value={confirmation}
              onChange={(event) => {
                setConfirmation(event.target.value);
              }}
              autoComplete="new-password"
              required
            />
          </div>

          <p className="text-muted-foreground text-sm">{PASSWORD_HINT}</p>

          {error && <p className="text-destructive text-sm">{error}</p>}
          {success && (
            <p className="text-emerald-600 dark:text-emerald-400 text-sm">
              {success}
            </p>
          )}

          <Button type="submit" disabled={submitting}>
            {submitting ? "Salvando…" : "Alterar senha"}
          </Button>

          <p className="text-muted-foreground text-xs">
            A sessão atual continua com o login atual até expirar — a nova senha
            passa a valer no próximo login.
          </p>
        </form>
      </CardContent>
    </Card>
  );
}
