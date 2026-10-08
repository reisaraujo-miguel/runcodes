import { useState } from "react";

import { FileCode2Icon } from "lucide-react";

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
  createCompilationFile,
  deleteCompilationFile,
  getCompilationFiles,
  type CompilationFile,
} from "@/lib/api";
import { errorMessage } from "@/lib/errors";

/** The display name of a compilation file, tolerant of the loose API type. */
function fileName(file: CompilationFile): string {
  if (typeof file.filename === "string" && file.filename.trim() !== "") {
    return file.filename;
  }
  if (typeof file.name === "string" && file.name.trim() !== "") {
    return file.name;
  }
  return `Arquivo #${String(file.id)}`;
}

/** The storage path of a compilation file, when the API reports one. */
function filePath(file: CompilationFile): string | null {
  const path = file.path;
  return typeof path === "string" && path.trim() !== "" ? path : null;
}

/**
 * Manages the auxiliary files used to build a submission. Uploading a file whose
 * basename already exists replaces the stored one, so no duplicate cleanup is
 * needed.
 */
export function CompilationFilesCard({ exerciseId }: { exerciseId: number }) {
  const files = useAsync(
    () => getCompilationFiles(exerciseId),
    `compilation-${String(exerciseId)}`,
    "Não foi possível carregar os arquivos de compilação.",
  );
  const [selected, setSelected] = useState<File | null>(null);
  const [inputKey, setInputKey] = useState(0);
  const upload = useSubmit(
    "Não foi possível enviar o arquivo de compilação.",
  );
  const [actionError, setActionError] = useState<string | null>(null);

  function handleUpload() {
    if (!selected) return;
    void upload.run(async () => {
      await createCompilationFile(exerciseId, selected);
      setSelected(null);
      setInputKey((key) => key + 1);
      files.reload();
    }, "Arquivo de compilação enviado.");
  }

  async function handleDelete(fileId: number) {
    setActionError(null);
    try {
      await deleteCompilationFile(exerciseId, fileId);
      files.reload();
    } catch (error) {
      setActionError(errorMessage(error, "Erro ao remover o arquivo."));
    }
  }

  const list = files.data ?? [];

  return (
    <SectionCard
      title="Arquivos de compilação"
      description="Arquivos auxiliares usados na compilação (headers, bibliotecas). Um envio com o mesmo nome substitui o anterior."
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-64 flex-1 space-y-2">
            <Input
              key={inputKey}
              type="file"
              aria-label="Arquivo de compilação"
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
            Enviar arquivo
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
            icon={FileCode2Icon}
            title="Nenhum arquivo de compilação"
            description="Envie os arquivos auxiliares que a compilação das soluções precisa."
          />
        ) : (
          <ul className="divide-border divide-y rounded-lg border">
            {list.map((file) => {
              const path = filePath(file);
              return (
                <li
                  key={file.id}
                  className="flex items-center justify-between gap-3 px-3 py-2"
                >
                  <div className="min-w-0">
                    <p className="truncate font-mono text-sm">{fileName(file)}</p>
                    {path ? (
                      <p className="text-muted-foreground truncate text-xs">
                        {path}
                      </p>
                    ) : null}
                  </div>
                  <ConfirmButton
                    size="sm"
                    destructive
                    variant="destructive"
                    title="Remover arquivo"
                    description={`Remover “${fileName(file)}” dos arquivos de compilação?`}
                    confirmLabel="Remover"
                    onConfirm={() => handleDelete(file.id)}
                  >
                    Remover
                  </ConfirmButton>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </SectionCard>
  );
}
