import { useState } from "react";
import {
  isRouteErrorResponse,
  Link,
  useLocation,
  useRouteError,
} from "react-router";

import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (isRouteErrorResponse(error)) {
    const data: unknown = error.data;
    if (typeof data === "string" && data) {
      return data;
    }
    return `${String(error.status)} ${error.statusText}`;
  }
  return String(error);
}

function getErrorStack(error: unknown): string {
  if (error instanceof Error && error.stack) {
    return error.stack;
  }
  return "(stack trace indisponível)";
}

/**
 * Rendered by the router when a route throws or no route matches (404).
 * Shows a friendly not-found page or, for unexpected errors, the error
 * details so users can open an issue with useful information.
 */
export function RootErrorBoundary() {
  const error = useRouteError();
  const location = useLocation();
  const [copied, setCopied] = useState(false);

  const isNotFound = isRouteErrorResponse(error) && error.status === 404;

  if (isNotFound) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-background p-6">
        <Card className="w-full max-w-md text-center">
          <CardHeader>
            <CardTitle className="text-2xl">Página não encontrada</CardTitle>
            <CardDescription>
              A página que você tentou acessar não existe ou foi movida.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex justify-center gap-2">
            <Link to="/" className={buttonVariants({ variant: "default" })}>
              Voltar para o início
            </Link>
          </CardContent>
        </Card>
      </main>
    );
  }

  const message = getErrorMessage(error);
  const stack = getErrorStack(error);

  const handleCopy = () => {
    const issueReport = [
      "**Erro inesperado no RunCodes**",
      "",
      `Mensagem: ${message}`,
      `Página: ${window.location.href}`,
      `Rota: ${location.pathname}`,
      `Data: ${new Date().toISOString()}`,
      `Navegador: ${navigator.userAgent}`,
      "",
      "```",
      stack,
      "```",
    ].join("\n");

    void navigator.clipboard.writeText(issueReport).then(
      () => {
        setCopied(true);
      },
      () => {
        // Clipboard may be unavailable (e.g. missing permissions); ignore.
      },
    );
  };

  return (
    <main className="flex min-h-svh items-center justify-center bg-background p-6">
      <Card className="w-full max-w-xl">
        <CardHeader>
          <CardTitle className="text-2xl text-destructive">
            Algo deu errado
          </CardTitle>
          <CardDescription>
            Ocorreu um erro inesperado. Você pode tentar novamente ou copiar as
            informações abaixo para abrir uma issue.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm font-medium">{message}</p>

          <details className="rounded-lg border bg-muted/50 p-3 text-xs">
            <summary className="cursor-pointer font-medium select-none">
              Detalhes técnicos
            </summary>
            <pre className="mt-3 max-h-64 overflow-auto font-mono text-muted-foreground whitespace-pre-wrap">
              {stack}
            </pre>
          </details>

          <div className="flex flex-wrap gap-2">
            <Button type="button" onClick={handleCopy} variant="outline">
              {copied ? "Copiado!" : "Copiar informações do erro"}
            </Button>
            <Button
              type="button"
              onClick={() => {
                window.location.reload();
              }}
            >
              Tentar novamente
            </Button>
            <Link to="/" className={buttonVariants({ variant: "ghost" })}>
              Voltar para o início
            </Link>
          </div>
        </CardContent>
      </Card>
    </main>
  );
}
