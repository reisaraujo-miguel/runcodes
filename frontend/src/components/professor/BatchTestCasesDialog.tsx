import { useState } from "react";

import { UploadIcon } from "lucide-react";

import { parseLimit } from "@/components/professor/test-case-draft";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { Select } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import {
  importTestCasesBatch,
  type BatchTestCaseErrorBody,
  type BatchTestCaseIssue,
} from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";

const TEXT = "text";
const FILE = "file";

const TYPE_OPTIONS = [
  { value: TEXT, label: "Texto" },
  { value: FILE, label: "Arquivo" },
];

/**
 * Bulk test-case import. The professor uploads one zip and sets the shared
 * configuration once; the backend validates the whole archive before writing
 * anything and reports every folder that does not follow the convention.
 */
export function BatchTestCasesDialog({
  exerciseId,
  onClose,
  onImported,
}: {
  exerciseId: number;
  onClose: () => void;
  onImported: (created: number) => void;
}) {
  const [archive, setArchive] = useState<File | null>(null);
  const [inputType, setInputType] = useState(TEXT);
  const [outputType, setOutputType] = useState(TEXT);
  const [showInput, setShowInput] = useState(false);
  const [showExpectedOutput, setShowExpectedOutput] = useState(false);
  const [showUserOutput, setShowUserOutput] = useState(true);
  const [cpuTime, setCpuTime] = useState("");
  const [memLimit, setMemLimit] = useState("");
  const [stackLimit, setStackLimit] = useState("");
  const [fileSizeLimit, setFileSizeLimit] = useState("");
  const [issues, setIssues] = useState<BatchTestCaseIssue[]>([]);
  const [created, setCreated] = useState<number | null>(null);

  const { pending, error, setError, run } = useSubmit(
    "Não foi possível importar os casos de teste.",
  );

  function handleSubmit() {
    if (!archive) {
      setError("Selecione o arquivo .zip com os casos de teste.");
      return;
    }
    if (!archive.name.toLowerCase().endsWith(".zip")) {
      setError("O arquivo precisa ter a extensão .zip.");
      return;
    }

    setIssues([]);
    setCreated(null);

    void run(async () => {
      try {
        const result = await importTestCasesBatch(exerciseId, archive, {
          input_type: inputType,
          expected_output_type: outputType,
          show_input: showInput,
          show_expected_output: showExpectedOutput,
          show_user_output: showUserOutput,
          cpu_time_limit_seconds: parseLimit(cpuTime),
          mem_usage_limit_bytes: parseLimit(memLimit),
          stack_limit_bytes: parseLimit(stackLimit),
          file_size_limit_bytes: parseLimit(fileSizeLimit),
        });
        setCreated(result.created);
        onImported(result.created);
      } catch (caught) {
        if (caught instanceof ApiRequestError) {
          const body = caught.body as BatchTestCaseErrorBody | undefined;
          if (body?.invalid_folders && body.invalid_folders.length > 0) {
            setIssues(body.invalid_folders);
          }
        }
        throw caught;
      }
    });
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Importar casos de teste em lote</DialogTitle>
          <DialogDescription>
            Envie um .zip com uma pasta por caso. Cada pasta deve conter
            exatamente um arquivo <code>.in</code> e um <code>.out</code>; os
            demais arquivos são anexos do caso. A configuração abaixo vale para
            todos os casos do lote.
          </DialogDescription>
        </DialogHeader>

        <form
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            handleSubmit();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="batch-archive">Arquivo .zip</Label>
            <Input
              id="batch-archive"
              type="file"
              accept=".zip,application/zip"
              onChange={(event) => {
                setArchive(event.target.files?.item(0) ?? null);
                setIssues([]);
                setCreated(null);
              }}
              required
            />
            <p className="text-muted-foreground text-xs">
              Exemplo: <code>caso1/entrada.in</code>,{" "}
              <code>caso1/saida.out</code>, <code>caso1/ajuda.txt</code>.
            </p>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="batch-input-type">Tipo de entrada</Label>
              <Select
                id="batch-input-type"
                value={inputType}
                onChange={(event) => {
                  setInputType(event.target.value);
                }}
              >
                {TYPE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="batch-output-type">Tipo de saída esperada</Label>
              <Select
                id="batch-output-type"
                value={outputType}
                onChange={(event) => {
                  setOutputType(event.target.value);
                }}
              >
                {TYPE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </Select>
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="batch-cpu">Tempo limite (s)</Label>
              <Input
                id="batch-cpu"
                type="number"
                min="0"
                step="0.1"
                value={cpuTime}
                onChange={(event) => {
                  setCpuTime(event.target.value);
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="batch-mem">Limite de memória (bytes)</Label>
              <Input
                id="batch-mem"
                type="number"
                min="0"
                value={memLimit}
                onChange={(event) => {
                  setMemLimit(event.target.value);
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="batch-stack">Limite de pilha (bytes)</Label>
              <Input
                id="batch-stack"
                type="number"
                min="0"
                value={stackLimit}
                onChange={(event) => {
                  setStackLimit(event.target.value);
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="batch-file-size">Limite de arquivo (bytes)</Label>
              <Input
                id="batch-file-size"
                type="number"
                min="0"
                value={fileSizeLimit}
                onChange={(event) => {
                  setFileSizeLimit(event.target.value);
                }}
              />
            </div>
          </div>
          <p className="text-muted-foreground text-xs">
            0 usa o padrão da plataforma.
          </p>

          <div className="flex flex-wrap gap-4">
            <Label htmlFor="batch-show-input" className="font-normal">
              <Checkbox
                id="batch-show-input"
                checked={showInput}
                onCheckedChange={setShowInput}
              />
              Mostrar entrada
            </Label>
            <Label htmlFor="batch-show-output" className="font-normal">
              <Checkbox
                id="batch-show-output"
                checked={showExpectedOutput}
                onCheckedChange={setShowExpectedOutput}
              />
              Mostrar saída esperada
            </Label>
            <Label htmlFor="batch-show-user" className="font-normal">
              <Checkbox
                id="batch-show-user"
                checked={showUserOutput}
                onCheckedChange={setShowUserOutput}
              />
              Mostrar saída do aluno
            </Label>
          </div>

          {created !== null ? (
            <Alert variant="success">
              <AlertDescription>
                {created} caso(s) de teste importado(s).
              </AlertDescription>
            </Alert>
          ) : null}

          {error ? (
            <Alert variant="destructive">
              <AlertTitle>O arquivo não foi importado</AlertTitle>
              <AlertDescription>
                <div>{error}</div>
                {issues.length > 0 ? (
                  <ul className="mt-2 list-disc space-y-1 pl-5 text-sm">
                    {issues.map((issue) => (
                      <li key={`${issue.folder}-${issue.reason}`}>
                        <strong>{issue.folder}</strong>: {issue.reason}
                      </li>
                    ))}
                  </ul>
                ) : null}
              </AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={onClose}
            >
              {created !== null ? "Fechar" : "Cancelar"}
            </Button>
            <Button type="submit" disabled={pending || !archive}>
              {pending ? <Spinner className="size-4" /> : <UploadIcon />}
              Importar casos
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
