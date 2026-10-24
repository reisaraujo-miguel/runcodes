import { useState } from "react";

import { FileCode2Icon, PencilIcon, PlusIcon, PowerIcon } from "lucide-react";

import { FileTypeFormDialog } from "@/components/admin/FileTypeFormDialog";
import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { PageHeader } from "@/components/app/PageHeader";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAsync } from "@/hooks/use-async";
import {
  adminDeleteFileType,
  adminListFileTypes,
  adminUpdateFileType,
  type AdminFileType,
} from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";
import { errorMessage } from "@/lib/errors";

/**
 * The runtime catalog of file types the platform accepts. An admin registers a
 * new language here and it appears in the professor's exercise form on the next
 * load; no redeploy is involved.
 */
export function AdminFileTypesPage() {
  const list = useAsync(
    () => adminListFileTypes(),
    "admin-file-types",
    "Não foi possível carregar os tipos de arquivo.",
  );

  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<AdminFileType | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);

  const types = list.data ?? [];

  function applySaved(saved: AdminFileType) {
    setCreating(false);
    setEditing(null);
    setActionError(null);
    setNotice(`Tipo de arquivo ${saved.name} salvo.`);
    list.reload();
  }

  async function handleToggle(fileType: AdminFileType) {
    setBusyId(fileType.id);
    setNotice(null);
    setActionError(null);
    try {
      await adminUpdateFileType(fileType.id, {
        is_available: !fileType.is_available,
      });
      setNotice(
        fileType.is_available
          ? `${fileType.name} foi ocultado dos exercícios.`
          : `${fileType.name} voltou a ficar disponível.`,
      );
      list.reload();
    } catch (error) {
      setActionError(
        errorMessage(error, "Não foi possível alterar a disponibilidade."),
      );
    } finally {
      setBusyId(null);
    }
  }

  async function handleDelete(fileType: AdminFileType) {
    setNotice(null);
    setActionError(null);
    try {
      await adminDeleteFileType(fileType.id);
      setNotice(`${fileType.name} foi excluído.`);
      list.reload();
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 409) {
        setActionError(
          "Esse tipo é usado por pelo menos um exercício. Desative-o em vez de excluir.",
        );
        return;
      }
      setActionError(
        errorMessage(error, "Não foi possível excluir o tipo de arquivo."),
      );
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Tipos de arquivo"
        description="Cadastre linguagens e formatos aceitos pela plataforma. A alteração vale imediatamente, sem redeploy."
        actions={
          <Button
            onClick={() => {
              setCreating(true);
            }}
          >
            <PlusIcon />
            Novo tipo
          </Button>
        }
      />

      <SectionCard
        title="Catálogo da plataforma"
        description="Tipos indisponíveis ficam ocultos no formulário de exercícios, mas continuam válidos para os exercícios que já os usam."
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

          {list.loading && list.data === null ? (
            <LoadingState />
          ) : list.error ? (
            <ErrorState description={list.error} onRetry={list.reload} />
          ) : types.length === 0 ? (
            <EmptyState
              icon={FileCode2Icon}
              title="Nenhum tipo de arquivo cadastrado"
              description="Cadastre a primeira extensão para que os professores possam usá-la nos exercícios."
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nome</TableHead>
                  <TableHead>Extensão</TableHead>
                  <TableHead>Execução</TableHead>
                  <TableHead>Situação</TableHead>
                  <TableHead className="text-right">Ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {types.map((fileType) => (
                  <TableRow key={fileType.id}>
                    <TableCell className="font-medium">
                      {fileType.name}
                    </TableCell>
                    <TableCell>
                      <code className="bg-muted rounded px-1.5 py-0.5 font-mono text-xs">
                        .{fileType.extension}
                      </code>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={fileType.is_compilable ? "info" : "secondary"}
                      >
                        {fileType.is_compilable ? "Compilado" : "Interpretado"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          fileType.is_available ? "success" : "secondary"
                        }
                      >
                        {fileType.is_available ? "Disponível" : "Oculto"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={busyId === fileType.id}
                          onClick={() => {
                            void handleToggle(fileType);
                          }}
                        >
                          <PowerIcon />
                          {fileType.is_available ? "Ocultar" : "Disponibilizar"}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setEditing(fileType);
                          }}
                        >
                          <PencilIcon />
                          Editar
                        </Button>
                        <ConfirmButton
                          variant="destructive"
                          size="sm"
                          destructive
                          title="Excluir tipo de arquivo"
                          confirmLabel="Excluir"
                          description={
                            <>
                              Excluir {fileType.name} só é possível se nenhum
                              exercício o usa. Esta ação não pode ser desfeita.
                            </>
                          }
                          onConfirm={() => handleDelete(fileType)}
                        >
                          Excluir
                        </ConfirmButton>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </SectionCard>

      {creating ? (
        <FileTypeFormDialog
          onClose={() => {
            setCreating(false);
          }}
          onSaved={applySaved}
        />
      ) : null}

      {editing ? (
        <FileTypeFormDialog
          key={editing.id}
          fileType={editing}
          onClose={() => {
            setEditing(null);
          }}
          onSaved={applySaved}
        />
      ) : null}
    </div>
  );
}
