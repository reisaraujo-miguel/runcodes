import { useState } from "react";

import { ArchiveIcon, DownloadIcon, RotateCcwIcon } from "lucide-react";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { List, ListRow } from "@/components/app/List";
import { Section } from "@/components/app/Section";
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

type BusyAction = "backup" | "submissions" | "recompile";

/**
 * Batch operations that act on a whole class: a reusable backup, the latest
 * submission of every student and a recompilation run. They are one list of
 * equally weighted actions, so each is a row instead of a card; every
 * destructive or expensive action is confirmed first.
 */
export function OfferingToolsPanel({ offeringId }: { offeringId: number }) {
  const [busy, setBusy] = useState<BusyAction | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

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
    <Section
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

        <List aria-label="Ferramentas da turma">
          <ListRow
            icon={ArchiveIcon}
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
              {busy === "backup" ? <Spinner /> : <DownloadIcon />}
              Exportar turma
            </Button>
          </ListRow>

          <ListRow
            icon={DownloadIcon}
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
              {busy === "submissions" ? <Spinner /> : <DownloadIcon />}
              Baixar submissões
            </Button>
          </ListRow>

          <ListRow
            icon={RotateCcwIcon}
            title="Recompilar submissões"
            description="Enfileira uma nova correção da última submissão de cada aluno em cada exercício, por exemplo depois de ajustar os casos de teste."
          >
            <ConfirmButton
              size="sm"
              variant="outline"
              disabled={busy !== null}
              title="Recompilar submissões"
              description="A última submissão de cada aluno em cada exercício será corrigida novamente. As notas atuais continuam no histórico até a nova correção terminar."
              onConfirm={handleRecompile}
            >
              {busy === "recompile" ? <Spinner /> : <RotateCcwIcon />}
              Recompilar
            </ConfirmButton>
          </ListRow>
        </List>
      </div>
    </Section>
  );
}
