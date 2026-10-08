import { useState } from "react";

import { PaperclipIcon } from "lucide-react";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
import { useSubmit } from "@/hooks/use-submit";
import {
  createAttachedFile,
  deleteAttachedFile,
  listAttachedFiles,
} from "@/lib/api";
import { errorMessage } from "@/lib/errors";

/**
 * Manages the materials attached to an exercise: the statement, datasets and
 * starter files students read on the submission page. Uploading a same-named
 * file replaces the stored one. The platform exposes no download endpoint, so
 * only the filenames and upload/removal are offered here.
 */
export function AttachedFilesCard({ exerciseId }: { exerciseId: number }) {
  const files = useAsync(
    () => listAttachedFiles(exerciseId),
    `attached-files-${String(exerciseId)}`,
    "Não foi possível carregar os materiais anexados.",
  );
  const [selected, setSelected] = useState<File | null>(null);
  const [inputKey, setInputKey] = useState(0);
  const upload = useSubmit("Não foi possível anexar o material.");
  const [actionError, setActionError] = useState<string | null>(null);

  function handleUpload() {
    if (!selected) return;
    void upload.run(async () => {
      await createAttachedFile(exerciseId, selected);
      setSelected(null);
      setInputKey((key) => key + 1);
      files.reload();
    }, "Material anexado.");
  }

  async function handleDelete(fileId: number) {
    setActionError(null);
    try {
      await deleteAttachedFile(exerciseId, fileId);
      files.reload();
    } catch (error) {
      setActionError(errorMessage(error, "Erro ao remover o material."));
    }
  }

  const list = files.data ?? [];

  return (
    <SectionCard
      title="Materiais anexados"
      description="Enunciados, dados e arquivos de apoio mostrados aos alunos. A plataforma não disponibiliza download; aqui você só envia e remove."
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-64 flex-1 space-y-2">
            <Input
              key={inputKey}
              type="file"
              aria-label="Material do exercício"
              className="h-auto py-2"
              onChange={(event) => {
                setSelected(event.target.files?.item(0) ?? null);
              }}
            />
          </div>
          <Button
            type="button"
            disabled={!selected || upload.pending}
            onClick={handleUpload}
          >
            {upload.pending ? <Spinner className="size-4" /> : null}
            Anexar material
          </Button>
        </div>

        {upload.error ? (
          <Alert variant="destructive">
            <AlertDescription>{upload.error}</AlertDescription>
          </Alert>
        ) : null}
        {upload.success ? (
          <Alert variant="success">
            <AlertDescription>{upload.success}</AlertDescription>
          </Alert>
        ) : null}
        {actionError ? (
          <Alert variant="destructive">
            <AlertDescription>{actionError}</AlertDescription>
          </Alert>
        ) : null}

        {files.loading && files.data === null ? (
          <LoadingState className="py-8" />
        ) : files.error ? (
          <ErrorState description={files.error} onRetry={files.reload} />
        ) : list.length === 0 ? (
          <EmptyState
            icon={PaperclipIcon}
            title="Nenhum material anexado"
            description="Anexe o enunciado ou arquivos de apoio para os alunos."
          />
        ) : (
          <ul className="divide-border divide-y rounded-lg border">
            {list.map((file) => (
              <li
                key={file.id}
                className="flex items-center justify-between gap-3 px-3 py-2"
              >
                <div className="min-w-0">
                  <p className="truncate font-mono text-sm">{file.filename}</p>
                  {file.path ? (
                    <p className="text-muted-foreground truncate text-xs">
                      {file.path}
                    </p>
                  ) : null}
                </div>
                <ConfirmButton
                  size="sm"
                  destructive
                  variant="destructive"
                  title="Remover material"
                  description={`Remover “${file.filename}” dos materiais do exercício?`}
                  confirmLabel="Remover"
                  onConfirm={() => handleDelete(file.id)}
                >
                  Remover
                </ConfirmButton>
              </li>
            ))}
          </ul>
        )}
      </div>
    </SectionCard>
  );
}
