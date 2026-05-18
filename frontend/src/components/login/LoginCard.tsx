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
import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Navigate } from "react-router";
import { z } from "zod";

const formSchema = z.object({
  Email: z.string().min(1, "email é obrigatório"),
  Password: z.string().min(8, "senha é obrigatória"),
});

const apiErrorSchema = z.object({
  error_msg: z.string(),
});

const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT as string;

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
      const response = await fetch(`${API_BASE_URL}/api/v1/user/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          email: data.Email,
          password: data.Password,
        }),
      });
      if (response.ok) {
        setIsLogged(true);
      } else {
        let message = "Erro desconhecido";
        try {
          const parsed = apiErrorSchema.safeParse(await response.json());
          if (parsed.success) {
            message = parsed.data.error_msg;
          }
        } catch {
          // Non-JSON response, use default message
        }
        console.error("Erro ao logar:", message);
      }
    } catch (error) {
      console.error("Erro ao logar:", error);
      return;
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
            <CardDescription>
              Digite seu email para entrar na sua conta
            </CardDescription>
            <CardAction>
              <Button variant="link">Cadastre-se</Button>
            </CardAction>
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
