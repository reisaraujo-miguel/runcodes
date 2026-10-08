import { useState } from "react";

import { CheckIcon, FlaskConicalIcon, PlusIcon, XIcon } from "lucide-react";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { SectionCard } from "@/components/app/SectionCard";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { EditTestCaseForm } from "@/components/professor/EditTestCaseForm";
import { NewTestCaseForm } from "@/components/professor/NewTestCaseForm";
import {
  testCaseFileKey,
  testCaseFileName,
} from "@/components/professor/test-case-draft";
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
import { deleteTestCase, getExerciseTestCases, type TestCase } from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatBytes, formatCpuTime, formatLimit } from "@/lib/format";
import { cn } from "@/lib/utils";

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
    <span
      title={value}
      className="block max-w-48 overflow-hidden font-mono text-xs whitespace-pre text-ellipsis"
    >
      {value}
    </span>
  );
}

/**
 * Lists an exercise's test cases and hosts the create / edit / delete flows.
 * Editing happens inline, in place of the row being edited.
 */
export function TestCasesCard({ exerciseId }: { exerciseId: number }) {
  const testCases = useAsync(
    () => getExerciseTestCases(exerciseId),
    `test-cases-${String(exerciseId)}`,
    "Não foi possível carregar os casos de teste.",
  );

  const [showNew, setShowNew] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

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

  return (
    <SectionCard
      title="Casos de teste"
      description="Entradas e saídas esperadas usadas na correção."
      action={
        <Button
          size="sm"
          variant={showNew ? "outline" : "default"}
          onClick={() => {
            setShowNew((previous) => !previous);
          }}
        >
          {showNew ? null : <PlusIcon />}
          {showNew ? "Fechar" : "Novo caso"}
        </Button>
      }
    >
      <div className="space-y-4">
        {showNew ? (
          <NewTestCaseForm
            exerciseId={exerciseId}
            onCreated={() => {
              testCases.reload();
            }}
          />
        ) : null}

        {actionError ? (
          <Alert variant="destructive">
            <AlertDescription>{actionError}</AlertDescription>
          </Alert>
        ) : null}

        {testCases.loading && testCases.data === null ? (
          <LoadingState className="py-8" />
        ) : testCases.error ? (
          <ErrorState
            description={testCases.error}
            onRetry={testCases.reload}
          />
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
              {list.map((testCase) =>
                editingId === testCase.id ? (
                  <TableRow key={testCase.id}>
                    <TableCell colSpan={7}>
                      <EditTestCaseForm
                        exerciseId={exerciseId}
                        testCase={testCase}
                        onCancel={() => {
                          setEditingId(null);
                        }}
                        onSaved={() => {
                          setEditingId(null);
                          testCases.reload();
                        }}
                      />
                    </TableCell>
                  </TableRow>
                ) : (
                  <TestRow
                    key={testCase.id}
                    testCase={testCase}
                    onEdit={() => {
                      setEditingId(testCase.id);
                    }}
                    onDelete={() => handleDelete(testCase.id)}
                  />
                ),
              )}
            </TableBody>
          </Table>
        )}
      </div>
    </SectionCard>
  );
}

/** One test case row, with its type, flags, limits and actions. */
function TestRow({
  testCase,
  onEdit,
  onDelete,
}: {
  testCase: TestCase;
  onEdit: () => void;
  onDelete: () => Promise<void>;
}) {
  const files = testCase.files;

  return (
    <TableRow>
      <TableCell className="font-medium whitespace-nowrap">
        #{String(testCase.id)}
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
        <div className="text-muted-foreground flex flex-col gap-0.5 text-xs whitespace-nowrap">
          <span>
            Tempo: {formatLimit(testCase.cpu_time_limit_seconds, formatCpuTime)}
          </span>
          <span>
            Memória: {formatLimit(testCase.mem_usage_limit_bytes, formatBytes)}
          </span>
          <span>
            Pilha: {formatLimit(testCase.stack_limit_bytes, formatBytes)}
          </span>
          <span>
            Arquivo: {formatLimit(testCase.file_size_limit_bytes, formatBytes)}
          </span>
        </div>
      </TableCell>
      <TableCell>
        {files.length === 0 ? (
          <span className="text-muted-foreground text-xs">Nenhum</span>
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
            description={`Remover o caso #${String(testCase.id)}? Esta ação não pode ser desfeita.`}
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
