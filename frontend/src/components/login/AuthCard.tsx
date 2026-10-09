import { zodResolver } from "@hookform/resolvers/zod";
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
import { login, signUp } from "@/lib/api/auth";
import { errorMessage } from "@/lib/errors";

const SPECIAL_CHARACTER = /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?~`]/;

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
    password: z
      .string()
      .min(8, "A senha deve ter no mínimo 8 caracteres")
      .regex(/[A-Z]/, "Inclua ao menos uma letra maiúscula")
      .regex(/[a-z]/, "Inclua ao menos uma letra minúscula")
      .regex(/[0-9]/, "Inclua ao menos um dígito")
      .regex(SPECIAL_CHARACTER, "Inclua ao menos um caractere especial"),
    passwordConfirmation: z.string().min(1, "Confirme a sua senha"),
  })
  .refine((data) => data.password === data.passwordConfirmation, {
    message: "As senhas não conferem",
    path: ["passwordConfirmation"],
  });

type LoginValues = z.infer<typeof loginSchema>;
type SignUpValues = z.infer<typeof signUpSchema>;

/** The sign-in form. */
function LoginForm() {
  const { refreshAuth } = useAuth();
  const [done, setDone] = useState(false);
  const [apiError, setApiError] = useState<string | null>(null);
  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  async function onSubmit(values: LoginValues) {
    setApiError(null);
    try {
      await login(values);
      // The backend set the session cookie; load the user (and role) so the
      // route guard can grant access.
      await refreshAuth();
      setDone(true);
    } catch (error) {
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
            <FieldLabel htmlFor="password">Senha</FieldLabel>
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
  const { refreshAuth } = useAuth();
  const [done, setDone] = useState(false);
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
      // Auto-login after a successful sign-up.
      await login({ email: values.email, password: values.password });
      await refreshAuth();
      setDone(true);
    } catch (error) {
      setApiError(errorMessage(error, "Não foi possível criar a conta."));
    }
  }

  if (done) return <Navigate to="/" replace />;

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
