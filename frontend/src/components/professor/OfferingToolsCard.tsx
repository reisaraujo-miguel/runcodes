import { useState, type ReactNode } from "react";

import {
  ArchiveIcon,
  DownloadIcon,
  MailIcon,
  RotateCcwIcon,
} from "lucide-react";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { SectionCard } from "@/components/app/SectionCard";
import { EmailMembersDialog } from "@/components/professor/EmailMembersDialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  downloadOfferingArchive,
  downloadOfferingSubmissions,
  recompileOfferingSubmissions,
} from "@/lib/api";
import { downloadBlob } from "@/lib/download";
import { errorMessage } from "@/lib/errors";

type BusyAction = "backup" | "submissions" | "recompile" | "email";

function Tool({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col justify-between gap-3 rounded-xl border p-4">
      <div className="space-y-1">
        <p className="font-medium">{title}</p>
        <p className="text-muted-foreground text-sm">{description}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">{children}</div>
    </div>
  );
}

/**
 * Batch operations that act on a whole class: a reusable backup, the latest
 * submission of every student, a recompilation run and an email to every member.
 * Every destructive or expensive action is confirmed first.
 */
export function OfferingToolsCard({
  offeringId,
  offeringName,
}: {
  offeringId: number;
  offeringName: string;
}) {
  const [busy, setBusy] = useState<BusyAction | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [emailing, setEmailing] = useState(false);

  async function handleDownload(
    action: "backup" | "submissions",
  ): Promise<void> {
    setBusy(action);
    setNotice(null);
    setActionError(null);
    try {
      const file =
        action === "backup"
          ? await downloadOfferingArchive(offeringId)
          : await downloadOfferingSubmissions(offeringId);
      downloadBlob(file.filename, file.blob);
      setNotice(
        action === "backup"
          ? "Backup da turma baixado."
          : "Últimas submissões baixadas.",
      );
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível baixar o arquivo."));
    } finally {
      setBusy(null);
    }
  }

  async function handleRecompile() {
    setBusy("recompile");
    setNotice(null);
    setActionError(null);
    try {
      const result = await recompileOfferingSubmissions(offeringId);
      setNotice(
        result.queued === 0
          ? "Não há submissões para recompilar."
          : `${String(result.queued)} submissão(ões) reenfileirada(s) para correção.`,
      );
    } catch (error) {
      setActionError(
        errorMessage(
          error,
          "Não foi possível reenfileirar as submissões. Verifique se o juiz está disponível.",
        ),
      );
    } finally {
      setBusy(null);
    }
  }

  return (
    <>
      <SectionCard
        title="Ferramentas da turma"
        description="Ações que valem para toda a turma."
      >
        <div className="space-y-4">
          {notice ? (
            <Alert variant="success">
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          ) : null}
          {actionError ? (
            <Alert variant="destructive">
              <AlertDescription>{actionError}</AlertDescription>
            </Alert>
          ) : null}

          <div className="grid gap-4 lg:grid-cols-2">
            <Tool
              title="Backup completo da turma"
              description="Baixa um .zip com a configuração, os exercícios, os casos de teste, os arquivos de compilação e os materiais. Pode ser importado como uma nova turma."
            >
              <Button
                size="sm"
                variant="outline"
                disabled={busy !== null}
                onClick={() => {
                  void handleDownload("backup");
                }}
              >
                {busy === "backup" ? (
                  <Spinner className="size-4" />
                ) : (
                  <ArchiveIcon />
                )}
                Exportar turma
              </Button>
            </Tool>

            <Tool
              title="Últimas submissões"
              description="Baixa um .zip com a submissão mais recente de cada aluno em cada exercício e um CSV com as notas dessas submissões."
            >
              <Button
                size="sm"
                variant="outline"
                disabled={busy !== null}
                onClick={() => {
                  void handleDownload("submissions");
                }}
              >
                {busy === "submissions" ? (
                  <Spinner className="size-4" />
                ) : (
                  <DownloadIcon />
                )}
                Baixar submissões
              </Button>
            </Tool>

            <Tool
              title="Recompilar submissões"
              description="Enfileira uma nova correção da última submissão de cada aluno em cada exercício, por exemplo depois de ajustar os casos de teste."
            >
              <ConfirmButton
                size="sm"
                variant="outline"
                title="Recompilar submissões"
                confirmLabel="Recompilar"
                disabled={busy !== null}
                description="A última submissão de cada aluno em cada exercício será corrigida novamente. As notas atuais continuam no histórico até a nova correção terminar."
                onConfirm={handleRecompile}
              >
                {busy === "recompile" ? (
                  <Spinner className="size-4" />
                ) : (
                  <RotateCcwIcon />
                )}
                Recompilar
              </ConfirmButton>
            </Tool>

            <Tool
              title="Email para a turma"
              description="Envia um aviso por email para todos os participantes da turma. A entrega acontece em segundo plano."
            >
              <Button
                size="sm"
                variant="outline"
                disabled={busy !== null}
                onClick={() => {
                  setEmailing(true);
                }}
              >
                <MailIcon />
                Escrever email
              </Button>
            </Tool>
          </div>
        </div>
      </SectionCard>

      {emailing ? (
        <EmailMembersDialog
          offeringId={offeringId}
          offeringName={offeringName}
          onClose={() => {
            setEmailing(false);
          }}
        />
      ) : null}
    </>
  );
}
