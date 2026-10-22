import { MailCheckIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router";

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
import { Spinner } from "@/components/ui/spinner";
import { verifyEmail } from "@/lib/api/auth";
import { ApiRequestError } from "@/lib/api/client";
import { errorMessage } from "@/lib/errors";

type VerificationState =
  | { status: "verifying" }
  | { status: "done" }
  | { status: "error"; message: string };

/**
 * Target of the link in a verification email. The token arrives in the query
 * string and is exchanged for a confirmed account by a POST, so an email client
 * that prefetches the URL does not consume it by itself.
 */
export function VerifyEmailPage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token")?.trim() ?? "";
  const [state, setState] = useState<VerificationState>(() =>
    token
      ? { status: "verifying" }
      : {
          status: "error",
          message: "Este link não contém um código de verificação.",
        },
  );
  // StrictMode runs effects twice in development; without the guard the second
  // request would reuse an already consumed token.
  const started = useRef(false);

  useEffect(() => {
    // The missing-token error is the state's initial value: setting it here
    // would render once and then again for no reason.
    if (!token || started.current) return;
    started.current = true;

    verifyEmail(token)
      .then(() => {
        setState({ status: "done" });
      })
      .catch((error: unknown) => {
        setState({
          status: "error",
          message:
            error instanceof ApiRequestError && error.status === 400
              ? "O link de verificação é inválido ou expirou. Entre na sua conta e peça um novo link."
              : error instanceof ApiRequestError && error.status === 409
                ? "O endereço solicitado não está mais disponível. Entre na sua conta e peça a troca novamente."
                : errorMessage(
                    error,
                    "Não foi possível verificar o seu email. Tente novamente.",
                  ),
        });
      });
  }, [token]);

  return (
    <AuthShell>
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Verificação de email</CardTitle>
          <CardDescription>
            Estamos confirmando o endereço da sua conta RunCodes.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {state.status === "verifying" ? (
            <div className="text-muted-foreground flex items-center gap-3 text-sm">
              <Spinner className="size-5" label="Verificando o seu email" />
              Verificando o seu email…
            </div>
          ) : null}

          {state.status === "done" ? (
            <Alert variant="success">
              <MailCheckIcon aria-hidden />
              <AlertTitle>Email confirmado</AlertTitle>
              <AlertDescription>
                A sua conta está ativa. Se você acabou de alterar o email, entre
                com o novo endereço e a sua senha; as sessões antigas foram
                encerradas.
              </AlertDescription>
            </Alert>
          ) : null}

          {state.status === "error" ? (
            <Alert variant="destructive">
              <AlertTitle>Não foi possível confirmar</AlertTitle>
              <AlertDescription>{state.message}</AlertDescription>
            </Alert>
          ) : null}
        </CardContent>
        <CardFooter>
          <Button size="lg" className="w-full" render={<Link to="/login" />}>
            {state.status === "done" ? "Entrar" : "Ir para o login"}
          </Button>
        </CardFooter>
      </Card>
    </AuthShell>
  );
}
