import { zodResolver } from "@hookform/resolvers/zod";
import { CheckCircle2Icon } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useSearchParams } from "react-router";
import { z } from "zod";

import { AuthShell } from "@/components/login/AuthShell";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { PasswordInput } from "@/components/ui/password-input";
import { Spinner } from "@/components/ui/spinner";
import { resetPassword } from "@/lib/api/auth";
import { ApiRequestError } from "@/lib/api/client";
import { errorMessage } from "@/lib/errors";
import { passwordSchema } from "@/lib/password";

const resetPasswordSchema = z
  .object({
    password: passwordSchema,
    passwordConfirmation: z.string().min(1, "Confirme a sua senha"),
  })
  .refine((data) => data.password === data.passwordConfirmation, {
    message: "As senhas não conferem",
    path: ["passwordConfirmation"],
  });

type ResetPasswordValues = z.infer<typeof resetPasswordSchema>;

/**
 * Target of the link in a password recovery email. The token arrives in the
 * query string and is only sent to the API when the user submits a new
 * password, so an email client that prefetches the URL does not consume it.
 */
export function ResetPasswordPage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token")?.trim() ?? "";
  const [done, setDone] = useState(false);
  const [apiError, setApiError] = useState<string | null>(null);
  const form = useForm<ResetPasswordValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { password: "", passwordConfirmation: "" },
  });

  async function onSubmit(values: ResetPasswordValues) {
    if (!token) return;

    setApiError(null);
    try {
      await resetPassword({
        token,
        password: values.password,
        password_confirmation: values.passwordConfirmation,
      });
      setDone(true);
    } catch (error) {
      setApiError(
        error instanceof ApiRequestError && error.status === 400
          ? "O link de recuperação é inválido ou expirou. Peça um novo link."
          : errorMessage(
              error,
              "Não foi possível redefinir a sua senha. Tente novamente.",
            ),
      );
    }
  }

  if (done) {
    return (
      <AuthShell>
        <Card className="w-full max-w-sm">
          <CardHeader>
            <CardTitle>Senha redefinida</CardTitle>
            <CardDescription>A sua nova senha já está valendo.</CardDescription>
          </CardHeader>
          <CardContent>
            <Alert variant="success">
              <CheckCircle2Icon aria-hidden />
              <AlertTitle>Tudo certo</AlertTitle>
              <AlertDescription>
                Entre com o seu email e a nova senha. Se a sua conta ainda não
                tiver o email confirmado, reenvie o link de verificação na tela
                de login.
              </AlertDescription>
            </Alert>
          </CardContent>
          <CardFooter>
            <Button size="lg" className="w-full" render={<Link to="/login" />}>
              Ir para o login
            </Button>
          </CardFooter>
        </Card>
      </AuthShell>
    );
  }

  if (!token) {
    return (
      <AuthShell>
        <Card className="w-full max-w-sm">
          <CardHeader>
            <CardTitle>Link de recuperação</CardTitle>
            <CardDescription>
              Não encontramos um código neste endereço.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Alert variant="destructive">
              <AlertTitle>Link incompleto</AlertTitle>
              <AlertDescription>
                Este link não contém um código de recuperação. Peça um novo link
                para redefinir a sua senha.
              </AlertDescription>
            </Alert>
          </CardContent>
          <CardFooter className="flex-col gap-3">
            <Button
              size="lg"
              className="w-full"
              render={<Link to="/forgot-password" />}
            >
              Pedir novo link
            </Button>
            <Button
              size="lg"
              variant="ghost"
              className="w-full"
              render={<Link to="/login" />}
            >
              Voltar para o login
            </Button>
          </CardFooter>
        </Card>
      </AuthShell>
    );
  }

  const errors = form.formState.errors;

  return (
    <AuthShell>
      <form
        className="w-full max-w-sm"
        onSubmit={(...args) => void form.handleSubmit(onSubmit)(...args)}
        noValidate
      >
        <Card>
          <CardHeader>
            <CardTitle>Criar uma nova senha</CardTitle>
            <CardDescription>
              Escolha uma senha nova para a sua conta RunCodes.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Field data-invalid={errors.password ? true : undefined}>
              <FieldLabel htmlFor="password">Nova senha</FieldLabel>
              <PasswordInput
                id="password"
                autoComplete="new-password"
                autoFocus
                aria-invalid={errors.password ? true : undefined}
                {...form.register("password")}
              />
              <FieldError errors={[errors.password]} />
            </Field>

            <Field
              data-invalid={errors.passwordConfirmation ? true : undefined}
            >
              <FieldLabel htmlFor="passwordConfirmation">
                Confirmar nova senha
              </FieldLabel>
              <PasswordInput
                id="passwordConfirmation"
                autoComplete="new-password"
                aria-invalid={errors.passwordConfirmation ? true : undefined}
                {...form.register("passwordConfirmation")}
              />
              <FieldError errors={[errors.passwordConfirmation]} />
            </Field>

            {apiError ? (
              <Alert variant="destructive">
                <AlertDescription>{apiError}</AlertDescription>
              </Alert>
            ) : null}
          </CardContent>
          <CardFooter className="flex-col gap-3">
            <Button
              type="submit"
              size="lg"
              className="w-full"
              disabled={form.formState.isSubmitting}
            >
              {form.formState.isSubmitting ? (
                <Spinner className="size-4" />
              ) : null}
              Redefinir senha
            </Button>
            <p className="text-muted-foreground text-sm">
              O link expirou?{" "}
              <Link
                to="/forgot-password"
                className="text-foreground font-medium underline-offset-4 hover:underline"
              >
                Peça outro
              </Link>
            </p>
          </CardFooter>
        </Card>
      </form>
    </AuthShell>
  );
}
