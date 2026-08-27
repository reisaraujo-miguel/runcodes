import { Button } from "@/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { useAuth } from "@/hooks/use-auth";
import { login, signUp } from "@/lib/api/auth";
import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Link, Navigate } from "react-router";
import { z } from "zod";

const formSchema = z
  .object({
    Name: z
      .string()
      .min(1, "nome é obrigatório")
      .max(100, "nome deve ter no máximo 100 caracteres"),
    Email: z
      .string()
      .min(1, "email é obrigatório")
      .pipe(z.email("email inválido")),
    Password: z
      .string()
      .min(8, "a senha deve ter no mínimo 8 caracteres")
      .regex(/[A-Z]/, "a senha deve conter pelo menos uma letra maiúscula")
      .regex(/[a-z]/, "a senha deve conter pelo menos uma letra minúscula")
      .regex(/[0-9]/, "a senha deve conter pelo menos um dígito")
      .regex(
        /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?~`]/,
        "a senha deve conter pelo menos um caractere especial",
      ),
    PasswordConfirmation: z
      .string()
      .min(1, "confirmação de senha é obrigatória"),
  })
  .refine((data) => data.Password === data.PasswordConfirmation, {
    message: "as senhas não conferem",
    path: ["PasswordConfirmation"],
  });

export function SignInCard() {
  const { setAuthenticated } = useAuth();
  const [isLogged, setIsLogged] = useState(false);
  const [apiError, setApiError] = useState<string | null>(null);

  const form = useForm({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Name: "",
      Email: "",
      Password: "",
      PasswordConfirmation: "",
    },
  });

  const onSubmit = async (data: z.infer<typeof formSchema>) => {
    try {
      setApiError(null);
      await signUp({
        name: data.Name,
        email: data.Email,
        password: data.Password,
        password_confirmation: data.PasswordConfirmation,
      });
      // Auto-login after successful sign-up
      await login({ email: data.Email, password: data.Password });
      // The backend set the session cookie — mark the app as authenticated
      // and let ProtectedRoute grant access.
      setAuthenticated(true);
      setIsLogged(true);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Erro ao cadastrar";
      setApiError(message);
    }
  };

  if (isLogged) {
    return <Navigate to="/" replace />;
  }

  return (
    <Card className="w-full max-w-sm">
      <form
        onSubmit={(...args) => void form.handleSubmit(onSubmit)(...args)}
        className="space-y-4"
      >
        <FieldGroup>
          <CardHeader>
            <CardTitle>Criar sua conta</CardTitle>
            <CardAction>
              <Link
                to="."
                className="inline-block text-sm underline-offset-4 hover:underline"
              >
                Já tem uma conta? Entrar
              </Link>
            </CardAction>
            <CardDescription className="col-span-full">
              Digite seus dados para criar uma conta
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col gap-6">
              <div className="grid gap-2">
                <Field>
                  <FieldLabel htmlFor="Name">Nome</FieldLabel>
                  <Input
                    id="Name"
                    type="text"
                    placeholder="Seu nome"
                    {...form.register("Name")}
                    required
                  />
                  {form.formState.errors.Name && (
                    <p className="text-sm text-destructive">
                      {form.formState.errors.Name.message}
                    </p>
                  )}
                </Field>
              </div>
              <div className="grid gap-2">
                <Field>
                  <FieldLabel htmlFor="Email">Email</FieldLabel>
                  <Input
                    id="Email"
                    type="email"
                    placeholder="m@example.com"
                    {...form.register("Email")}
                    required
                  />
                  {form.formState.errors.Email && (
                    <p className="text-sm text-destructive">
                      {form.formState.errors.Email.message}
                    </p>
                  )}
                </Field>
              </div>
              <div className="grid gap-2">
                <Field>
                  <FieldLabel htmlFor="Password">Senha</FieldLabel>
                  <Input
                    id="Password"
                    type="password"
                    required
                    {...form.register("Password")}
                  />
                  {form.formState.errors.Password && (
                    <p className="text-sm text-destructive">
                      {form.formState.errors.Password.message}
                    </p>
                  )}
                </Field>
              </div>
              <div className="grid gap-2">
                <Field>
                  <FieldLabel htmlFor="PasswordConfirmation">
                    Confirmar Senha
                  </FieldLabel>
                  <Input
                    id="PasswordConfirmation"
                    type="password"
                    required
                    {...form.register("PasswordConfirmation")}
                  />
                  {form.formState.errors.PasswordConfirmation && (
                    <p className="text-sm text-destructive">
                      {form.formState.errors.PasswordConfirmation.message}
                    </p>
                  )}
                </Field>
              </div>
              {apiError && (
                <p className="text-sm text-destructive text-center">
                  {apiError}
                </p>
              )}
            </div>
          </CardContent>
          <CardFooter className="flex-col gap-2">
            <Button type="submit" className="w-full">
              Cadastrar
            </Button>
          </CardFooter>
        </FieldGroup>
      </form>
    </Card>
  );
}
