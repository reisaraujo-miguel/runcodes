import { useState, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { updateTestCase, type TestCase, type TestCasePatch } from "@/lib/api";

const FILE = "file";

/** Parses a limit field; an empty value means "keep the stored limit". */
function parseLimit(value: string): number | undefined {
  if (value.trim() === "") return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
}

/** Renders a stored limit as the value of a number input (0 = default). */
function limitToInput(value: number): string {
  return Number.isFinite(value) ? String(value) : "";
}

interface EditTestCaseFormProps {
  exerciseId: number;
  testCase: TestCase;
  onSaved: (testCase: TestCase) => void;
  onCancel: () => void;
}

/** Inline form to edit an existing test case of an exercise. */
export function EditTestCaseForm({
  exerciseId,
  testCase,
  onSaved,
  onCancel,
}: EditTestCaseFormProps) {
  const [inputText, setInputText] = useState(testCase.input);
  const [outputText, setOutputText] = useState(testCase.expected_output);
  const [inputFile, setInputFile] = useState<File | null>(null);
  const [outputFile, setOutputFile] = useState<File | null>(null);
  const [showInput, setShowInput] = useState(testCase.show_input);
  const [showExpectedOutput, setShowExpectedOutput] = useState(
    testCase.show_expected_output,
  );
  const [showUserOutput, setShowUserOutput] = useState(
    testCase.show_user_output,
  );
  const [cpuTime, setCpuTime] = useState(() =>
    limitToInput(testCase.cpu_time_limit_seconds),
  );
  const [memLimit, setMemLimit] = useState(() =>
    limitToInput(testCase.mem_usage_limit_bytes),
  );
  const [stackLimit, setStackLimit] = useState(() =>
    limitToInput(testCase.stack_limit_bytes),
  );
  const [fileSizeLimit, setFileSizeLimit] = useState(() =>
    limitToInput(testCase.file_size_limit_bytes),
  );
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const isFileInput = testCase.input_type === FILE;
  const isFileOutput = testCase.expected_output_type === FILE;

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    // The patch carries exactly the values that differ from the stored ones, so
    // the untouched fields (and any uploaded file) keep their current value.
    const patch: TestCasePatch = {};

    if (isFileInput) {
      if (inputFile) patch.input_file = inputFile;
    } else if (inputText !== testCase.input) {
      patch.input = inputText;
    }

    if (isFileOutput) {
      if (outputFile) patch.expected_output_file = outputFile;
    } else if (outputText !== testCase.expected_output) {
      patch.expected_output = outputText;
    }

    if (showInput !== testCase.show_input) patch.show_input = showInput;
    if (showExpectedOutput !== testCase.show_expected_output) {
      patch.show_expected_output = showExpectedOutput;
    }
    if (showUserOutput !== testCase.show_user_output) {
      patch.show_user_output = showUserOutput;
    }

    const parsedCpuTime = parseLimit(cpuTime);
    if (
      parsedCpuTime !== undefined &&
      parsedCpuTime !== testCase.cpu_time_limit_seconds
    ) {
      patch.cpu_time_limit_seconds = parsedCpuTime;
    }
    const parsedMemLimit = parseLimit(memLimit);
    if (
      parsedMemLimit !== undefined &&
      parsedMemLimit !== testCase.mem_usage_limit_bytes
    ) {
      patch.mem_usage_limit_bytes = parsedMemLimit;
    }
    const parsedStackLimit = parseLimit(stackLimit);
    if (
      parsedStackLimit !== undefined &&
      parsedStackLimit !== testCase.stack_limit_bytes
    ) {
      patch.stack_limit_bytes = parsedStackLimit;
    }
    const parsedFileSizeLimit = parseLimit(fileSizeLimit);
    if (
      parsedFileSizeLimit !== undefined &&
      parsedFileSizeLimit !== testCase.file_size_limit_bytes
    ) {
      patch.file_size_limit_bytes = parsedFileSizeLimit;
    }

    if (Object.keys(patch).length === 0) {
      onSaved(testCase);
      return;
    }

    setSubmitting(true);
    setError(null);
    void (async () => {
      try {
        const updated = await updateTestCase(exerciseId, testCase.id, patch);
        onSaved(updated);
      } catch (submitError) {
        setError(
          submitError instanceof Error
            ? submitError.message
            : "Erro ao salvar o caso de teste",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  return (
    <Card size="sm" className="bg-muted/40">
      <CardHeader>
        <CardTitle className="text-sm">
          Editar caso #{String(testCase.id)}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="edit-case-input">Entrada</Label>
            {isFileInput ? (
              <div className="space-y-2">
                <p className="text-muted-foreground text-sm">
                  A entrada deste caso é um arquivo enviado. Selecione um
                  arquivo para substituí-lo, ou deixe em branco para manter o
                  arquivo atual.
                </p>
                <Input
                  id="edit-case-input"
                  type="file"
                  onChange={(event) => {
                    setInputFile(event.target.files?.item(0) ?? null);
                  }}
                />
              </div>
            ) : (
              <Textarea
                id="edit-case-input"
                value={inputText}
                onChange={(event) => {
                  setInputText(event.target.value);
                }}
                placeholder="Entrada padrão do caso de teste"
              />
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="edit-case-output">Saída esperada</Label>
            {isFileOutput ? (
              <div className="space-y-2">
                <p className="text-muted-foreground text-sm">
                  A saída esperada deste caso é um arquivo enviado. Selecione um
                  arquivo para substituí-lo, ou deixe em branco para manter o
                  arquivo atual.
                </p>
                <Input
                  id="edit-case-output"
                  type="file"
                  onChange={(event) => {
                    setOutputFile(event.target.files?.item(0) ?? null);
                  }}
                />
              </div>
            ) : (
              <Textarea
                id="edit-case-output"
                value={outputText}
                onChange={(event) => {
                  setOutputText(event.target.value);
                }}
                placeholder="Saída esperada do programa"
              />
            )}
          </div>

          <div className="space-y-2">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="edit-case-cpu">Tempo limite (s)</Label>
                <Input
                  id="edit-case-cpu"
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
                <Label htmlFor="edit-case-mem">Limite de memória (bytes)</Label>
                <Input
                  id="edit-case-mem"
                  type="number"
                  min="0"
                  value={memLimit}
                  onChange={(event) => {
                    setMemLimit(event.target.value);
                  }}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-case-stack">Limite de pilha (bytes)</Label>
                <Input
                  id="edit-case-stack"
                  type="number"
                  min="0"
                  value={stackLimit}
                  onChange={(event) => {
                    setStackLimit(event.target.value);
                  }}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="edit-case-filesize">
                  Limite de arquivo (bytes)
                </Label>
                <Input
                  id="edit-case-filesize"
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
          </div>

          <div className="flex flex-wrap gap-4">
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="accent-primary size-4"
                checked={showInput}
                onChange={(event) => {
                  setShowInput(event.target.checked);
                }}
              />
              Mostrar entrada
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="accent-primary size-4"
                checked={showExpectedOutput}
                onChange={(event) => {
                  setShowExpectedOutput(event.target.checked);
                }}
              />
              Mostrar saída esperada
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="accent-primary size-4"
                checked={showUserOutput}
                onChange={(event) => {
                  setShowUserOutput(event.target.checked);
                }}
              />
              Mostrar saída do aluno
            </label>
          </div>

          {error && <p className="text-destructive text-sm">{error}</p>}

          <div className="flex gap-2">
            <Button type="submit" disabled={submitting}>
              {submitting ? "Salvando…" : "Salvar"}
            </Button>
            <Button type="button" variant="outline" onClick={onCancel}>
              Cancelar
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
