import { useState, type SubmitEvent } from "react";

import { CheckCircle2Icon, TicketIcon } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { enroll, type UserOffering } from "@/lib/api";
import { errorMessage } from "@/lib/errors";

/** Join a class with the enrollment code its professor shared. */
export function EnrollForm({
  onEnrolled,
}: {
  onEnrolled: (offering: UserOffering) => void;
}) {
  const [code, setCode] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [joined, setJoined] = useState<string | null>(null);

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = code.trim();
    if (trimmed === "") return;

    setPending(true);
    setError(null);
    setJoined(null);
    try {
      const offering = await enroll(trimmed);
      setCode("");
      setJoined(offering.name);
      onEnrolled(offering);
    } catch (err) {
      setError(
        errorMessage(err, "Não foi possível usar este código de matrícula."),
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={(event) => void handleSubmit(event)} className="space-y-3">
      <div className="space-y-2">
        <Label htmlFor="enrollment-code">Código da turma</Label>
        <div className="flex gap-2">
          <Input
            id="enrollment-code"
            value={code}
            onChange={(event) => {
              setCode(event.target.value);
            }}
            placeholder="Ex.: AB12CD"
            autoComplete="off"
            autoCapitalize="characters"
            className="font-mono uppercase"
          />
          <Button type="submit" disabled={pending || code.trim() === ""}>
            {pending ? <Spinner className="size-4" /> : <TicketIcon />}
            Entrar
          </Button>
        </div>
        <p className="text-muted-foreground text-xs">
          Peça o código ao professor da disciplina.
        </p>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      {joined ? (
        <Alert variant="success">
          <CheckCircle2Icon aria-hidden />
          <AlertDescription>Você entrou em “{joined}”.</AlertDescription>
        </Alert>
      ) : null}
    </form>
  );
}
