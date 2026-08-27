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
import { PasswordInput } from "@/components/ui/password-input";
import { useAuth } from "@/hooks/use-auth";
import { login } from "@/lib/api/auth";
import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Link, Navigate } from "react-router";
import { z } from "zod";

const formSchema = z.object({
  Email: z.string().min(1, "email é obrigatório"),
  Password: z.string().min(8, "senha é obrigatória"),
});

export function LoginCard() {
  const { refreshAuth } = useAuth();
  const [isLogged, setIsLogged] = useState(false);
  const [apiError, setApiError] = useState<string | null>(null);

  const form = useForm({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Email: "",
      Password: "",
    },
  });

  const onSubmit = async (data: z.infer<typeof formSchema>) => {
    try {
      setApiError(null);
      await login({ email: data.Email, password: data.Password });
      // The backend set the session cookie — load the user (and role) into
      // the auth context and let ProtectedRoute grant access.
      await refreshAuth();
      setIsLogged(true);
    } catch (error) {
      console.error("Erro ao logar:", error);
      const message = error instanceof Error ? error.message : "Erro ao logar";
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
            <CardTitle>Entrar na sua conta</CardTitle>
            <CardAction>
              <Link
                to="?mode=signup"
                className="inline-block text-sm underline-offset-4 hover:underline"
              >
                Cadastre-se
              </Link>
            </CardAction>
            <CardDescription className="col-span-full">
              Digite seu email para entrar na sua conta
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col gap-6">
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
                </Field>
              </div>
              <div className="grid gap-2">
                <div className="flex items-center">
                  <Field>
                    <FieldLabel htmlFor="Password">Senha</FieldLabel>
                    <a
                      href="#"
                      className="ml-auto inline-block text-sm underline-offset-4 hover:underline"
                    >
                      Esqueceu sua senha?
                    </a>
                    <PasswordInput
                      id="Password"
                      required
                      {...form.register("Password")}
                    />
                  </Field>
                </div>
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
              Login
            </Button>
          </CardFooter>
        </FieldGroup>
      </form>
    </Card>
  );
}
