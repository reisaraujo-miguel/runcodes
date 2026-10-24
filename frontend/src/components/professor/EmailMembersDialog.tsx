import { useState } from "react";

import { MailIcon } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { useSubmit } from "@/hooks/use-submit";
import { notifyOfferingMembers } from "@/lib/api";

/**
 * Compose one email for every member of a class. Delivery happens in the
 * background on the server, so the dialog reports how many recipients were
 * queued rather than waiting for the last delivery.
 */
export function EmailMembersDialog({
  offeringId,
  offeringName,
  onClose,
}: {
  offeringId: number;
  offeringName: string;
  onClose: () => void;
}) {
  const [subject, setSubject] = useState("");
  const [message, setMessage] = useState("");
  const [recipients, setRecipients] = useState<number | null>(null);
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível enviar o email.",
  );

  function handleSubmit() {
    if (!subject.trim()) {
      setError("Informe o assunto do email.");
      return;
    }
    if (!message.trim()) {
      setError("Escreva a mensagem do email.");
      return;
    }

    void run(async () => {
      const result = await notifyOfferingMembers(
        offeringId,
        subject.trim(),
        message,
      );
      setRecipients(result.recipients);
    });
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Enviar email para a turma</DialogTitle>
          <DialogDescription>
            A mensagem vai para todos os participantes de {offeringName},
            incluindo alunos, monitores e professores.
          </DialogDescription>
        </DialogHeader>

        {recipients === null ? (
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              handleSubmit();
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="offering-email-subject">Assunto</Label>
              <Input
                id="offering-email-subject"
                value={subject}
                maxLength={200}
                onChange={(event) => {
                  setSubject(event.target.value);
                }}
                placeholder="Ex.: Alteração no prazo do exercício 3"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="offering-email-message">Mensagem</Label>
              <Textarea
                id="offering-email-message"
                value={message}
                rows={8}
                onChange={(event) => {
                  setMessage(event.target.value);
                }}
                placeholder="Escreva o aviso para a turma…"
                required
              />
              <p className="text-muted-foreground text-xs">
                A mensagem é enviada como texto simples com uma versão em HTML.
                Não é possível anexar arquivos.
              </p>
            </div>

            {error ? (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                disabled={pending}
                onClick={onClose}
              >
                Cancelar
              </Button>
              <Button type="submit" disabled={pending}>
                {pending ? <Spinner className="size-4" /> : <MailIcon />}
                Enviar para a turma
              </Button>
            </DialogFooter>
          </form>
        ) : (
          <div className="space-y-4">
            <Alert variant="success">
              <AlertDescription>
                Email enfileirado para {recipients} participante
                {recipients === 1 ? "" : "s"}. A entrega pode levar alguns
                minutos.
              </AlertDescription>
            </Alert>
            <DialogFooter>
              <Button type="button" onClick={onClose}>
                Fechar
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
