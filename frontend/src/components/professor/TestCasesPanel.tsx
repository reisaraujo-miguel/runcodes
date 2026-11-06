import { useState } from "react";

import {
  CheckIcon,
  DownloadIcon,
  FlaskConicalIcon,
  PlusIcon,
  UploadIcon,
  XIcon,
} from "lucide-react";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { Section } from "@/components/app/Section";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { BatchTestCasesDialog } from "@/components/professor/BatchTestCasesDialog";
import { EditTestCaseForm } from "@/components/professor/EditTestCaseForm";
import {
  formatLimitMiB,
  formatLimitSeconds,
} from "@/components/professor/language-limits-format";
import { NewTestCaseDialog } from "@/components/professor/NewTestCaseDialog";
import {
  testCaseFileKey,
  testCaseFileName,
} from "@/components/professor/test-case-draft";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
  deleteTestCase,
  downloadExerciseTestCases,
  getExerciseTestCases,
  type TestCase,
} from "@/lib/api";
import { downloadBlob } from "@/lib/download";
import { errorMessage } from "@/lib/errors";
import { cn } from "@/lib/utils";

/** Spells out the ignored root files, counting the ones that do not fit. */
function ignoredSummary(names: readonly string[]): string {
  const shown = names.slice(0, 6).join(", ");
  return names.length > 6
    ? `${shown}, … (+${String(names.length - 6)})`
    : shown;
}

const FILE_INPUT = "file";

/** A compact on/off indicator for a test case's visibility flag. */
function VisibilityFlag({ on, label }: { on: boolean; label: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1",
        on ? "text-foreground" : "text-muted-foreground/70",
      )}
    >
      {on ? (
        <CheckIcon className="size-3" aria-hidden />
      ) : (
        <XIcon className="size-3" aria-hidden />
      )}
      {label}
    </span>
  );
}

/** A one-line, clipped preview of a text value. */
function TextPreview({ value }: { value: string }) {
  return (
    <div
      title={value}
      className="max-h-16 max-w-56 overflow-y-auto font-mono text-xs wrap-break-word whitespace-pre-wrap"
    >
      {value}
    </div>
  );
}

/**
 * Lists an exercise's test cases and hosts the create / edit / delete flows.
 * Editing opens a dialog, so a case with long input does not push the table
 * around.
 */
export function TestCasesPanel({ exerciseId }: { exerciseId: number }) {
  const testCases = useAsync(
    () => getExerciseTestCases(exerciseId),
    `test-cases-${String(exerciseId)}`,
    "Não foi possível carregar os casos de teste.",
  );

  const [showNew, setShowNew] = useState(false);
  const [showBatch, setShowBatch] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  async function handleExport() {
    setExporting(true);
    setActionError(null);
    setNotice(null);
    try {
      const file = await downloadExerciseTestCases(exerciseId);
      downloadBlob(file.filename, file.blob);
      setNotice("Casos de teste exportados.");
    } catch (error) {
      setActionError(
        errorMessage(error, "Erro ao exportar os casos de teste."),
      );
    } finally {
      setExporting(false);
    }
  }

  async function handleDelete(caseId: number) {
    setActionError(null);
    try {
      await deleteTestCase(exerciseId, caseId);
      setEditingId((previous) => (previous === caseId ? null : previous));
      testCases.reload();
    } catch (error) {
      setActionError(errorMessage(error, "Erro ao remover o caso de teste."));
    }
  }

  const list = testCases.data ?? [];
  const description = "Entradas e saídas esperadas usadas na correção.";
  const actions = (
    <div className="flex flex-wrap items-center gap-2">
      <Button
        size="sm"
        variant="outline"
        disabled={exporting || list.length === 0}
        onClick={() => {
          void handleExport();
        }}
      >
        <DownloadIcon />
        {exporting ? "Exportando…" : "Exportar .zip"}
      </Button>
      <Button
        size="sm"
        variant={showBatch ? "secondary" : "outline"}
        onClick={() => {
          setShowBatch((previous) => !previous);
        }}
      >
        <UploadIcon />
        Importar em lote
      </Button>
      <Button
        size="sm"
        onClick={() => {
          setShowNew(true);
        }}
      >
        <PlusIcon />
        Novo caso
      </Button>
    </div>
  );

  const content = (
    <div className="space-y-4">
      {showNew ? (
        <NewTestCaseDialog
          exerciseId={exerciseId}
          onClose={() => {
            setShowNew(false);
          }}
          onCreated={() => {
            testCases.reload();
          }}
        />
      ) : null}

      {showBatch ? (
        <BatchTestCasesDialog
          exerciseId={exerciseId}
          onClose={() => {
            setShowBatch(false);
          }}
          onImported={(created, ignored) => {
            const base = `${String(created)} caso(s) importado(s).`;
            setNotice(
              ignored.length === 0
                ? base
                : `${base} Arquivo(s) ignorado(s) na raiz: ${ignoredSummary(ignored)}.`,
            );
            testCases.reload();
          }}
        />
      ) : null}

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

      {testCases.loading && testCases.data === null ? (
        <LoadingState className="py-8" />
      ) : testCases.error ? (
        <ErrorState description={testCases.error} onRetry={testCases.reload} />
      ) : list.length === 0 ? (
        <EmptyState
          icon={FlaskConicalIcon}
          title="Nenhum caso de teste"
          description="Adicione casos com a entrada e a saída esperada para corrigir as soluções."
        />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Caso</TableHead>
              <TableHead>Entrada</TableHead>
              <TableHead>Saída esperada</TableHead>
              <TableHead>Visibilidade</TableHead>
              <TableHead>Limites</TableHead>
              <TableHead>Arquivos</TableHead>
              <TableHead className="text-right">Ações</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.map((testCase, index) => {
              // Test cases are numbered per exercise, from 1, in display order.
              // Their database id is global and would make a second exercise's
              // first case appear as #2.
              const number = index + 1;
              return (
                <TestRow
                  key={testCase.id}
                  testCase={testCase}
                  number={number}
                  onEdit={() => {
                    setEditingId(testCase.id);
                  }}
                  onDelete={() => handleDelete(testCase.id)}
                />
              );
            })}
          </TableBody>
        </Table>
      )}
    </div>
  );

  const editingCase = list.find((testCase) => testCase.id === editingId);
  const editingNumber = editingCase ? list.indexOf(editingCase) + 1 : 0;

  return (
    <>
      <Section
        title="Casos de teste"
        description={description}
        action={actions}
      >
        {content}
      </Section>

      {editingCase ? (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) setEditingId(null);
          }}
        >
          <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-4xl">
            <DialogHeader>
              <DialogTitle>Editar caso #{String(editingNumber)}</DialogTitle>
              <DialogDescription>
                As saídas e os limites mudam neste caso; os demais casos de
                teste não são afetados.
              </DialogDescription>
            </DialogHeader>
            <EditTestCaseForm
              exerciseId={exerciseId}
              testCase={editingCase}
              onCancel={() => {
                setEditingId(null);
              }}
              onSaved={() => {
                setEditingId(null);
                testCases.reload();
              }}
            />
          </DialogContent>
        </Dialog>
      ) : null}
    </>
  );
}

/** One test case row, with its type, flags, limits and actions. */
function TestRow({
  testCase,
  number,
  onEdit,
  onDelete,
}: {
  testCase: TestCase;
  /** 1-based position within the exercise, not the global test case id. */
  number: number;
  onEdit: () => void;
  onDelete: () => Promise<void>;
}) {
  const files = testCase.files;

  return (
    <TableRow>
      <TableCell className="font-medium whitespace-nowrap">
        #{String(number)}
      </TableCell>
      <TableCell>
        <Badge variant="outline">
          {testCase.input_type === FILE_INPUT ? "Arquivo" : "Texto"}
        </Badge>
        {testCase.input_type !== FILE_INPUT && testCase.input ? (
          <TextPreview value={testCase.input} />
        ) : null}
      </TableCell>
      <TableCell>
        <Badge variant="outline">
          {testCase.expected_output_type === FILE_INPUT ? "Arquivo" : "Texto"}
        </Badge>
        {testCase.expected_output_type !== FILE_INPUT &&
        testCase.expected_output ? (
          <TextPreview value={testCase.expected_output} />
        ) : null}
      </TableCell>
      <TableCell>
        <div className="flex flex-col gap-0.5 text-xs">
          <VisibilityFlag on={testCase.show_input} label="Entrada" />
          <VisibilityFlag
            on={testCase.show_expected_output}
            label="Saída esperada"
          />
          <VisibilityFlag
            on={testCase.show_user_output}
            label="Saída do aluno"
          />
        </div>
      </TableCell>
      <TableCell>
        <div className="space-y-1">
          {testCase.limits.map((language) => {
            const sources = [
              language.sources.cpu_time_limit_seconds,
              language.sources.mem_usage_limit_bytes,
              language.sources.stack_limit_bytes,
              language.sources.file_size_limit_bytes,
            ];
            const layer = sources.includes("case")
              ? "caso"
              : sources.includes("exercise")
                ? "exercício"
                : "plataforma";
            return (
              <p
                key={language.allowed_file_type_id}
                className="text-xs whitespace-nowrap text-muted-foreground"
              >
                <span className="font-medium text-foreground/80">
                  {language.name}
                </span>
                {`: ${formatLimitSeconds(language.limits.cpu_time_limit_seconds)} · ${formatLimitMiB(language.limits.mem_usage_limit_bytes)} · ${formatLimitMiB(language.limits.stack_limit_bytes)} · ${formatLimitMiB(language.limits.file_size_limit_bytes)} `}
                <span className="text-muted-foreground/80">({layer})</span>
              </p>
            );
          })}
        </div>
      </TableCell>
      <TableCell>
        {files.length === 0 ? (
          <span className="text-xs text-muted-foreground">Nenhum</span>
        ) : (
          <details>
            <summary className="cursor-pointer text-xs select-none">
              {String(files.length)} arquivo(s)
            </summary>
            <ul className="mt-1 space-y-0.5">
              {files.map((file, index) => (
                <li
                  key={testCaseFileKey(file, index)}
                  className="font-mono text-xs break-all"
                >
                  {testCaseFileName(file, index)}
                </li>
              ))}
            </ul>
          </details>
        )}
      </TableCell>
      <TableCell>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button size="sm" variant="outline" onClick={onEdit}>
            Editar
          </Button>
          <ConfirmButton
            size="sm"
            destructive
            variant="destructive"
            title="Remover caso de teste"
            description={`Remover o caso #${String(number)}? Esta ação não pode ser desfeita.`}
            confirmLabel="Remover"
            onConfirm={onDelete}
          >
            Remover
          </ConfirmButton>
        </div>
      </TableCell>
    </TableRow>
  );
}
