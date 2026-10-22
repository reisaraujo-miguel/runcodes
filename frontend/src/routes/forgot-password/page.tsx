import { zodResolver } from "@hookform/resolvers/zod";
import { MailCheckIcon } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useSearchParams } from "react-router";
import { z } from "zod";

import { AuthShell } from "@/components/login/AuthShell";
import { Alert, AlertDescription } from "@/components/ui/alert";
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
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { forgotPassword } from "@/lib/api/auth";
import { errorMessage } from "@/lib/errors";

const forgotPasswordSchema = z.object({
  email: z
    .string()
    .min(1, "Informe o seu email")
    .pipe(z.email("Email inválido")),
});

type ForgotPasswordValues = z.infer<typeof forgotPasswordSchema>;

/**
 * Asks the API to email a recovery link. The API answers the same way for an
 * unknown address, so the success copy never claims that an account exists.
 */
export function ForgotPasswordPage() {
  const [searchParams] = useSearchParams();
  const [sentEmail, setSentEmail] = useState<string | null>(null);
  const [apiError, setApiError] = useState<string | null>(null);
  const form = useForm<ForgotPasswordValues>({
    resolver: zodResolver(forgotPasswordSchema),
    defaultValues: { email: searchParams.get("email")?.trim() ?? "" },
  });

  async function onSubmit(values: ForgotPasswordValues) {
    setApiError(null);
    try {
      await forgotPassword(values.email.trim());
      setSentEmail(values.email.trim());
    } catch (error) {
      setApiError(errorMessage(error, "Não foi possível enviar o link."));
    }
  }

  if (sentEmail) {
    return (
      <AuthShell>
        <Card className="w-full max-w-sm">
          <CardHeader>
            <CardTitle>Verifique o seu email</CardTitle>
            <CardDescription>
              Se o endereço tiver uma conta, o link de recuperação chegou.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Alert variant="info">
              <MailCheckIcon aria-hidden />
              <AlertDescription>
                Se houver uma conta para{" "}
                <span className="text-foreground font-medium">{sentEmail}</span>
                , enviamos um link para redefinir a senha.
              </AlertDescription>
            </Alert>
            <p className="text-muted-foreground text-sm">
              O link expira em 1 hora e só pode ser usado uma vez. Confira a
              caixa de spam se não encontrar a mensagem.
            </p>
          </CardContent>
          <CardFooter className="flex-col gap-3">
            <Button size="lg" className="w-full" render={<Link to="/login" />}>
              Voltar para o login
            </Button>
            <Button
              type="button"
              variant="ghost"
              className="w-full"
              onClick={() => {
                setSentEmail(null);
              }}
            >
              Usar outro email
            </Button>
          </CardFooter>
        </Card>
      </AuthShell>
    );
  }

  return (
    <AuthShell>
      <form
        className="w-full max-w-sm"
        onSubmit={(...args) => void form.handleSubmit(onSubmit)(...args)}
        noValidate
      >
        <Card>
          <CardHeader>
            <CardTitle>Recuperar senha</CardTitle>
            <CardDescription>
              Informe o email da sua conta e enviaremos um link para você criar
              uma nova senha.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Field
              data-invalid={form.formState.errors.email ? true : undefined}
            >
              <FieldLabel htmlFor="email">Email</FieldLabel>
              <Input
                id="email"
                type="email"
                placeholder="voce@exemplo.com"
                autoComplete="email"
                autoFocus
                aria-invalid={form.formState.errors.email ? true : undefined}
                {...form.register("email")}
              />
              <FieldError errors={[form.formState.errors.email]} />
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
              Enviar link de recuperação
            </Button>
            <p className="text-muted-foreground text-sm">
              Lembrou a senha?{" "}
              <Link
                to="/login"
                className="text-foreground font-medium underline-offset-4 hover:underline"
              >
                Voltar para o login
              </Link>
            </p>
          </CardFooter>
        </Card>
      </form>
    </AuthShell>
  );
}
