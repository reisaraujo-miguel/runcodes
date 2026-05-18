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
  const [isLogged, setIsLogged] = useState(false);

  const form = useForm({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Email: "",
      Password: "",
    },
  });

  const onSubmit = async (data: z.infer<typeof formSchema>) => {
    try {
      await login({ email: data.Email, password: data.Password });
      setIsLogged(true);
    } catch (error) {
      console.error("Erro ao logar:", error);
    }
  };

  if (isLogged) {
    return <Navigate to="/" replace />;
  }

  return (
    <Card className="w-full max-w-sm">
      <form onSubmit={void form.handleSubmit(onSubmit)} className="space-y-4">
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
                    <Input
                      id="Password"
                      type="password"
                      required
                      {...form.register("Password")}
                    />
                  </Field>
                </div>
              </div>
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
