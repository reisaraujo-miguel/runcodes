import { useState, type SubmitEvent } from "react";

import { SectionCard } from "@/components/app/SectionCard";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import { changePassword } from "@/lib/api/profile";

/** The password rules the backend enforces when a password is set. */
const PASSWORD_HINT =
  "A nova senha deve ter no mínimo 8 caracteres, com pelo menos uma letra maiúscula, uma minúscula, um dígito e um caractere especial.";

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

/** Replaces the caller's password after checking the current one. */
export function PasswordForm() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const { pending, error, success, setError, run } = useSubmit(
    "Não foi possível alterar a senha.",
  );

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!currentPassword) {
      setError("Informe a sua senha atual.");
      return;
    }
    if (newPassword !== confirmation) {
      setError("As senhas não coincidem.");
      return;
    }
    if (!isValidPassword(newPassword)) {
      setError("A nova senha não atende aos requisitos de segurança.");
      return;
    }

    void run(async () => {
      await changePassword({
        current_password: currentPassword,
        new_password: newPassword,
        new_password_confirmation: confirmation,
      });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmation("");
    }, "Senha alterada com sucesso.");
  }

  return (
    <SectionCard
      title="Alterar senha"
      description="Escolha uma nova senha para a sua conta."
    >
      <form
        className="space-y-4"
        onSubmit={(event) => {
          handleSubmit(event);
        }}
      >
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

        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
        {success ? (
          <Alert variant="success">
            <AlertDescription>{success}</AlertDescription>
          </Alert>
        ) : null}

        <Button type="submit" disabled={pending}>
          {pending ? <Spinner className="size-4" /> : null}
          Alterar senha
        </Button>

        <p className="text-muted-foreground text-xs">
          A sessão atual continua válida até expirar — a nova senha passa a
          valer no próximo login.
        </p>
      </form>
    </SectionCard>
  );
}
