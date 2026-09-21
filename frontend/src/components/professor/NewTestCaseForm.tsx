import { useState, type ChangeEvent, type SubmitEvent } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { createTestCase, type NewTestCase, type TestCase } from "@/lib/api";

const TEXT = "text";
const FILE = "file";

function parseLimit(value: string): number | undefined {
  if (value.trim() === "") return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
}

interface NewTestCaseFormProps {
  exerciseId: number;
  onCreated: (testCase: TestCase) => void;
}

/** Form to create a test case using inline text or uploaded files. */
export function NewTestCaseForm({
  exerciseId,
  onCreated,
}: NewTestCaseFormProps) {
  const [inputType, setInputType] = useState(TEXT);
  const [outputType, setOutputType] = useState(TEXT);
  const [inputText, setInputText] = useState("");
  const [outputText, setOutputText] = useState("");
  const [inputFile, setInputFile] = useState<File | null>(null);
  const [outputFile, setOutputFile] = useState<File | null>(null);
  const [showInput, setShowInput] = useState(false);
  const [showExpectedOutput, setShowExpectedOutput] = useState(false);
  const [showUserOutput, setShowUserOutput] = useState(true);
  const [cpuTime, setCpuTime] = useState("");
  const [memLimit, setMemLimit] = useState("");
  const [stackLimit, setStackLimit] = useState("");
  const [fileSizeLimit, setFileSizeLimit] = useState("");
  const [extraFiles, setExtraFiles] = useState<File[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function reset() {
    setInputText("");
    setOutputText("");
    setInputFile(null);
    setOutputFile(null);
    setExtraFiles([]);
  }

  function handleExtraFiles(event: ChangeEvent<HTMLInputElement>) {
    setExtraFiles(Array.from(event.target.files ?? []));
  }

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    if (inputType === FILE && !inputFile) {
      setError("Selecione o arquivo de entrada");
      return;
    }
    if (outputType === FILE && !outputFile) {
      setError("Selecione o arquivo de saída esperada");
      return;
    }

    const payload: NewTestCase = {
      input_type: inputType,
      expected_output_type: outputType,
      show_input: showInput,
      show_expected_output: showExpectedOutput,
      show_user_output: showUserOutput,
      ...(inputType === FILE
        ? { input_file: inputFile ?? undefined }
        : { input: inputText }),
      ...(outputType === FILE
        ? { expected_output_file: outputFile ?? undefined }
        : { expected_output: outputText }),
      ...(extraFiles.length > 0 ? { files: extraFiles } : {}),
    };
    const parsedCpuTime = parseLimit(cpuTime);
    const parsedMemLimit = parseLimit(memLimit);
    const parsedStackLimit = parseLimit(stackLimit);
    const parsedFileSizeLimit = parseLimit(fileSizeLimit);
    if (parsedCpuTime !== undefined) {
      payload.cpu_time_limit_seconds = parsedCpuTime;
    }
    if (parsedMemLimit !== undefined) {
      payload.mem_usage_limit_bytes = parsedMemLimit;
    }
    if (parsedStackLimit !== undefined) {
      payload.stack_limit_bytes = parsedStackLimit;
    }
    if (parsedFileSizeLimit !== undefined) {
      payload.file_size_limit_bytes = parsedFileSizeLimit;
    }

    setSubmitting(true);
    setError(null);
    void (async () => {
      try {
        const testCase = await createTestCase(exerciseId, payload);
        onCreated(testCase);
        reset();
      } catch (submitError) {
        setError(
          submitError instanceof Error
            ? submitError.message
            : "Erro ao criar o caso de teste",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  return (
    <Card size="sm" className="bg-muted/40">
      <CardHeader>
        <CardTitle className="text-sm">Novo caso de teste</CardTitle>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="case-input-type">Tipo de entrada</Label>
              <Select
                id="case-input-type"
                value={inputType}
                onChange={(event) => {
                  setInputType(event.target.value);
                }}
              >
                <option value={TEXT}>Texto</option>
                <option value={FILE}>Arquivo</option>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="case-output-type">Tipo de saída esperada</Label>
              <Select
                id="case-output-type"
                value={outputType}
                onChange={(event) => {
                  setOutputType(event.target.value);
                }}
              >
                <option value={TEXT}>Texto</option>
                <option value={FILE}>Arquivo</option>
              </Select>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="case-input">Entrada</Label>
            {inputType === FILE ? (
              <Input
                id="case-input"
                type="file"
                onChange={(event) => {
                  setInputFile(event.target.files?.item(0) ?? null);
                }}
              />
            ) : (
              <Textarea
                id="case-input"
                value={inputText}
                onChange={(event) => {
                  setInputText(event.target.value);
                }}
                placeholder="Entrada padrão do caso de teste"
              />
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="case-output">Saída esperada</Label>
            {outputType === FILE ? (
              <Input
                id="case-output"
                type="file"
                onChange={(event) => {
                  setOutputFile(event.target.files?.item(0) ?? null);
                }}
              />
            ) : (
              <Textarea
                id="case-output"
                value={outputText}
                onChange={(event) => {
                  setOutputText(event.target.value);
                }}
                placeholder="Saída esperada do programa"
              />
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="case-files">
              Arquivos adicionais do caso (opcional)
            </Label>
            <Input
              id="case-files"
              type="file"
              multiple
              onChange={handleExtraFiles}
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="case-cpu">Tempo limite (s)</Label>
              <Input
                id="case-cpu"
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
              <Label htmlFor="case-mem">Limite de memória (bytes)</Label>
              <Input
                id="case-mem"
                type="number"
                min="0"
                value={memLimit}
                onChange={(event) => {
                  setMemLimit(event.target.value);
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="case-stack">Limite de pilha (bytes)</Label>
              <Input
                id="case-stack"
                type="number"
                min="0"
                value={stackLimit}
                onChange={(event) => {
                  setStackLimit(event.target.value);
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="case-filesize">Limite de arquivo (bytes)</Label>
              <Input
                id="case-filesize"
                type="number"
                min="0"
                value={fileSizeLimit}
                onChange={(event) => {
                  setFileSizeLimit(event.target.value);
                }}
              />
            </div>
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

          <Button type="submit" disabled={submitting}>
            {submitting ? "Criando…" : "Adicionar caso de teste"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
