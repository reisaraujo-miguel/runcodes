import { zodResolver } from "@hookform/resolvers/zod";
import { MailCheckIcon } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Link, Navigate, useSearchParams } from "react-router";
import { z } from "zod";

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
import { PasswordInput } from "@/components/ui/password-input";
import { Spinner } from "@/components/ui/spinner";
import { useAuth } from "@/hooks/use-auth";
import { login, resendVerification, signUp } from "@/lib/api/auth";
import { ApiRequestError } from "@/lib/api/client";
import { errorMessage } from "@/lib/errors";
import { passwordSchema } from "@/lib/password";

const loginSchema = z.object({
  email: z.string().min(1, "Informe o seu email"),
  password: z.string().min(1, "Informe a sua senha"),
});

const signUpSchema = z
  .object({
    name: z
      .string()
      .min(1, "Informe o seu nome")
      .max(100, "O nome deve ter no máximo 100 caracteres"),
    email: z
      .string()
      .min(1, "Informe o seu email")
      .pipe(z.email("Email inválido")),
    password: passwordSchema,
    passwordConfirmation: z.string().min(1, "Confirme a sua senha"),
  })
  .refine((data) => data.password === data.passwordConfirmation, {
    message: "As senhas não conferem",
    path: ["passwordConfirmation"],
  });

type LoginValues = z.infer<typeof loginSchema>;
type SignUpValues = z.infer<typeof signUpSchema>;

/**
 * The resend affordance shared by the failed-login and post-signup views. The
 * endpoint answers the same way for an unknown address, so the copy never claims
 * that an account exists.
 */
function ResendVerification({ email }: { email: string }) {
  const [state, setState] = useState<"idle" | "sending" | "sent" | "error">(
    "idle",
  );

  async function handleResend() {
    setState("sending");
    try {
      await resendVerification(email);
      setState("sent");
    } catch {
      setState("error");
    }
  }

  return (
    <div className="space-y-2 rounded-lg border p-3">
      <p className="text-muted-foreground text-sm">
        Não recebeu? Confira a caixa de spam ou peça um novo link para{" "}
        <span className="text-foreground font-medium">{email}</span>.
      </p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={state === "sending"}
        onClick={() => void handleResend()}
      >
        {state === "sending" ? <Spinner className="size-4" /> : null}
        {state === "sent"
          ? "Enviar novamente"
          : "Reenviar email de verificação"}
      </Button>
      {state === "sent" ? (
        <p className="text-muted-foreground text-xs">
          Se houver uma conta pendente para esse endereço, o link chegará em
          instantes.
        </p>
      ) : null}
      {state === "error" ? (
        <p className="text-destructive text-xs">
          Não foi possível reenviar agora. Tente novamente em instantes.
        </p>
      ) : null}
    </div>
  );
}

/** The sign-in form. */
function LoginForm() {
  const { refreshAuth } = useAuth();
  const [done, setDone] = useState(false);
  const [apiError, setApiError] = useState<string | null>(null);
  const [unverifiedEmail, setUnverifiedEmail] = useState<string | null>(null);
  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  async function onSubmit(values: LoginValues) {
    setApiError(null);
    setUnverifiedEmail(null);
    try {
      await login(values);
      // The backend set the session cookie; load the user (and role) so the
      // route guard can grant access.
      await refreshAuth();
      setDone(true);
    } catch (error) {
      // 403 on this endpoint means the credentials are right but the account
      // has not confirmed its email yet.
      if (error instanceof ApiRequestError && error.status === 403) {
        setUnverifiedEmail(values.email.trim());
        setApiError(
          "Confirme o seu email antes de entrar. Abra o link que enviamos para ativar a sua conta.",
        );
        return;
      }
      setApiError(errorMessage(error, "Não foi possível entrar."));
    }
  }

  if (done) return <Navigate to="/" replace />;

  return (
    <form
      className="w-full max-w-sm"
      onSubmit={(...args) => void form.handleSubmit(onSubmit)(...args)}
      noValidate
    >
      <Card>
        <CardHeader>
          <CardTitle>Entrar na sua conta</CardTitle>
          <CardDescription>
            Use o email e a senha da sua conta RunCodes.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Field data-invalid={form.formState.errors.email ? true : undefined}>
            <FieldLabel htmlFor="email">Email</FieldLabel>
            <Input
              id="email"
              type="email"
              placeholder="voce@exemplo.com"
              autoComplete="email"
              aria-invalid={form.formState.errors.email ? true : undefined}
              {...form.register("email")}
            />
            <FieldError errors={[form.formState.errors.email]} />
          </Field>

          <Field
            data-invalid={form.formState.errors.password ? true : undefined}
          >
            <div className="flex items-center justify-between">
              <FieldLabel htmlFor="password">Senha</FieldLabel>
              <Link
                to="/forgot-password"
                className="text-muted-foreground hover:text-foreground text-sm font-medium underline-offset-4 hover:underline"
              >
                Esqueceu a senha?
              </Link>
            </div>
            <PasswordInput
              id="password"
              autoComplete="current-password"
              aria-invalid={form.formState.errors.password ? true : undefined}
              {...form.register("password")}
            />
            <FieldError errors={[form.formState.errors.password]} />
          </Field>

          {apiError ? (
            <Alert variant="destructive">
              <AlertDescription>{apiError}</AlertDescription>
            </Alert>
          ) : null}

          {unverifiedEmail ? (
            <ResendVerification key={unverifiedEmail} email={unverifiedEmail} />
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
            Entrar
          </Button>
          <p className="text-muted-foreground text-sm">
            Ainda não tem conta?{" "}
            <Link
              to="?mode=signup"
              className="text-foreground font-medium underline-offset-4 hover:underline"
            >
              Cadastre-se
            </Link>
          </p>
        </CardFooter>
      </Card>
    </form>
  );
}

/** The sign-up form. */
function SignUpForm() {
  const [createdEmail, setCreatedEmail] = useState<string | null>(null);
  const [apiError, setApiError] = useState<string | null>(null);
  const form = useForm<SignUpValues>({
    resolver: zodResolver(signUpSchema),
    defaultValues: {
      name: "",
      email: "",
      password: "",
      passwordConfirmation: "",
    },
  });

  async function onSubmit(values: SignUpValues) {
    setApiError(null);
    try {
      await signUp({
        name: values.name,
        email: values.email,
        password: values.password,
        password_confirmation: values.passwordConfirmation,
      });
      // No auto-login: the account is unverified until the emailed link is
      // opened, and the API refuses to start a session before that.
      setCreatedEmail(values.email.trim());
    } catch (error) {
      setApiError(errorMessage(error, "Não foi possível criar a conta."));
    }
  }

  if (createdEmail) {
    return (
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Confirme o seu email</CardTitle>
          <CardDescription>
            Falta só um passo para ativar a sua conta.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Alert variant="info">
            <MailCheckIcon aria-hidden />
            <AlertDescription>
              Enviamos um link de verificação para{" "}
              <span className="text-foreground font-medium">
                {createdEmail}
              </span>
              . Abra o link para ativar a conta; depois entre com a sua senha.
            </AlertDescription>
          </Alert>
          <p className="text-muted-foreground text-sm">
            O link expira em 24 horas.
          </p>
          <ResendVerification email={createdEmail} />
        </CardContent>
        <CardFooter>
          <Button size="lg" className="w-full" render={<Link to="/login" />}>
            Ir para o login
          </Button>
        </CardFooter>
      </Card>
    );
  }

  const errors = form.formState.errors;

  return (
    <form
      className="w-full max-w-sm"
      onSubmit={(...args) => void form.handleSubmit(onSubmit)(...args)}
      noValidate
    >
      <Card>
        <CardHeader>
          <CardTitle>Criar uma conta</CardTitle>
          <CardDescription>
            Preencha os dados para começar a usar o RunCodes.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Field data-invalid={errors.name ? true : undefined}>
            <FieldLabel htmlFor="name">Nome</FieldLabel>
            <Input
              id="name"
              autoComplete="name"
              aria-invalid={errors.name ? true : undefined}
              {...form.register("name")}
            />
            <FieldError errors={[errors.name]} />
          </Field>

          <Field data-invalid={errors.email ? true : undefined}>
            <FieldLabel htmlFor="email">Email</FieldLabel>
            <Input
              id="email"
              type="email"
              placeholder="voce@exemplo.com"
              autoComplete="email"
              aria-invalid={errors.email ? true : undefined}
              {...form.register("email")}
            />
            <FieldError errors={[errors.email]} />
          </Field>

          <Field data-invalid={errors.password ? true : undefined}>
            <FieldLabel htmlFor="password">Senha</FieldLabel>
            <PasswordInput
              id="password"
              autoComplete="new-password"
              aria-invalid={errors.password ? true : undefined}
              {...form.register("password")}
            />
            <FieldError errors={[errors.password]} />
          </Field>

          <Field data-invalid={errors.passwordConfirmation ? true : undefined}>
            <FieldLabel htmlFor="passwordConfirmation">
              Confirmar senha
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
            Criar conta
          </Button>
          <p className="text-muted-foreground text-sm">
            Já tem uma conta?{" "}
            <Link
              to="/login"
              className="text-foreground font-medium underline-offset-4 hover:underline"
            >
              Entrar
            </Link>
          </p>
        </CardFooter>
      </Card>
    </form>
  );
}

/** The authentication card, switching between sign-in and sign-up via `?mode`. */
export function AuthCard() {
  const [searchParams] = useSearchParams();
  const isSignUp = searchParams.get("mode") === "signup";
  return isSignUp ? <SignUpForm /> : <LoginForm />;
}
