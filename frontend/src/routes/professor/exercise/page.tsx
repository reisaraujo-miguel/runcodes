import { useEffect, useState, type ChangeEvent } from "react";
import { Link, useParams } from "react-router";

import { NewTestCaseForm } from "@/components/professor/NewTestCaseForm";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import {
  createCompilationFile,
  deleteCompilationFile,
  deleteTestCase,
  getCompilationFiles,
  getExercise,
  getExerciseTestCases,
  type CompilationFile,
  type Exercise,
  type TestCase,
} from "@/lib/api";
import { formatBytes, formatDateTime } from "@/lib/format";

function compilationFileName(file: CompilationFile): string {
  const name = file.filename ?? file.name;
  return name && name !== "" ? name : `Arquivo #${String(file.id)}`;
}

/** Professor page to manage an exercise's test cases and compilation files. */
export function ExercisePage() {
  const { exerciseId } = useParams();
  const numericExerciseId = Number(exerciseId);

  const [exercise, setExercise] = useState<Exercise | null>(null);
  const [testCases, setTestCases] = useState<TestCase[]>([]);
  const [compilationFiles, setCompilationFiles] = useState<CompilationFile[]>(
    [],
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [casesError, setCasesError] = useState<string | null>(null);
  const [filesError, setFilesError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const [newCompilationFile, setNewCompilationFile] = useState<File | null>(
    null,
  );
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function loadExercise() {
      if (!Number.isInteger(numericExerciseId) || numericExerciseId <= 0) {
        setError("Exercício inválido");
        setLoading(false);
        return;
      }

      try {
        const data = await getExercise(numericExerciseId);
        if (!cancelled) setExercise(data);
      } catch {
        if (!cancelled) {
          setError("Exercício não encontrado");
          setLoading(false);
        }
        return;
      }

      try {
        const cases = await getExerciseTestCases(numericExerciseId);
        if (!cancelled) setTestCases(cases);
      } catch {
        if (!cancelled) {
          setCasesError("Não foi possível carregar os casos de teste.");
        }
      }

      try {
        const files = await getCompilationFiles(numericExerciseId);
        if (!cancelled) setCompilationFiles(files);
      } catch {
        if (!cancelled) {
          setFilesError("Não foi possível carregar os arquivos de compilação.");
        }
      }

      if (!cancelled) setLoading(false);
    }

    void loadExercise();

    return () => {
      cancelled = true;
    };
  }, [numericExerciseId]);

  function handleCompilationFileChange(event: ChangeEvent<HTMLInputElement>) {
    setNewCompilationFile(event.target.files?.item(0) ?? null);
  }

  function handleUploadCompilationFile() {
    if (!newCompilationFile) return;
    setUploading(true);
    setActionError(null);
    void (async () => {
      try {
        const uploaded = await createCompilationFile(
          numericExerciseId,
          newCompilationFile,
        );
        setCompilationFiles((previous) => [...previous, uploaded]);
        setNewCompilationFile(null);
      } catch (uploadError) {
        setActionError(
          uploadError instanceof Error
            ? uploadError.message
            : "Erro ao enviar o arquivo de compilação",
        );
      } finally {
        setUploading(false);
      }
    })();
  }

  function handleDeleteCompilationFile(fileId: number) {
    setActionError(null);
    void (async () => {
      try {
        await deleteCompilationFile(numericExerciseId, fileId);
        setCompilationFiles((previous) =>
          previous.filter((file) => file.id !== fileId),
        );
      } catch (deleteError) {
        setActionError(
          deleteError instanceof Error
            ? deleteError.message
            : "Erro ao remover o arquivo de compilação",
        );
      }
    })();
  }

  function handleDeleteTestCase(caseId: number) {
    setActionError(null);
    void (async () => {
      try {
        await deleteTestCase(numericExerciseId, caseId);
        setTestCases((previous) =>
          previous.filter((testCase) => testCase.id !== caseId),
        );
      } catch (deleteError) {
        setActionError(
          deleteError instanceof Error
            ? deleteError.message
            : "Erro ao remover o caso de teste",
        );
      }
    })();
  }

  if (loading) {
    return (
      <div className="flex justify-center p-12">
        <div
          aria-label="Carregando"
          className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
          role="status"
        />
      </div>
    );
  }

  if (!exercise) {
    return (
      <div className="mx-auto max-w-3xl p-6">
        <Card>
          <CardHeader>
            <CardTitle>Exercício não encontrado</CardTitle>
            <CardDescription>
              {error ?? "Não foi possível carregar este exercício."}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-4 p-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">{exercise.title}</CardTitle>
          {exercise.description && (
            <CardDescription>{exercise.description}</CardDescription>
          )}
          <CardAction>
            <Link
              to={`/professor/class/${String(exercise.offering_id)}`}
              className={buttonVariants({ variant: "outline", size: "sm" })}
            >
              Voltar para a turma
            </Link>
          </CardAction>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
          <div>
            <p className="text-muted-foreground">Abertura</p>
            <p>{formatDateTime(exercise.open_date)}</p>
          </div>
          <div>
            <p className="text-muted-foreground">Prazo</p>
            <p>{formatDateTime(exercise.deadline)}</p>
          </div>
        </CardContent>
      </Card>

      {actionError && <p className="text-destructive text-sm">{actionError}</p>}

      <Card>
        <CardHeader>
          <CardTitle>Arquivos de compilação</CardTitle>
          <CardDescription>
            Arquivos auxiliares usados na compilação (headers, bibliotecas).
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-end gap-3">
            <div className="min-w-64 flex-1 space-y-2">
              <Input
                type="file"
                aria-label="Arquivo de compilação"
                onChange={handleCompilationFileChange}
              />
            </div>
            <Button
              type="button"
              disabled={!newCompilationFile || uploading}
              onClick={handleUploadCompilationFile}
            >
              {uploading ? "Enviando…" : "Enviar arquivo"}
            </Button>
          </div>

          {filesError && (
            <p className="text-destructive text-sm">{filesError}</p>
          )}

          {compilationFiles.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              Nenhum arquivo de compilação.
            </p>
          ) : (
            <ul className="space-y-2">
              {compilationFiles.map((file, index) => (
                <li key={file.id}>
                  {index > 0 && <Separator className="mb-2" />}
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-mono text-sm break-all">
                      {compilationFileName(file)}
                    </span>
                    <Button
                      type="button"
                      variant="destructive"
                      size="xs"
                      onClick={() => {
                        handleDeleteCompilationFile(file.id);
                      }}
                    >
                      Remover
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Casos de teste</CardTitle>
          <CardDescription>
            Entradas e saídas esperadas usadas na correção.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <NewTestCaseForm
            exerciseId={exercise.id}
            onCreated={(testCase) => {
              setTestCases((previous) => [...previous, testCase]);
            }}
          />

          {casesError && (
            <p className="text-destructive text-sm">{casesError}</p>
          )}

          {testCases.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              Nenhum caso de teste cadastrado.
            </p>
          ) : (
            <ul className="space-y-3">
              {testCases.map((testCase) => (
                <li
                  key={testCase.id}
                  className="space-y-2 rounded-lg border p-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="font-medium">
                      Caso #{String(testCase.id)}
                    </span>
                    <div className="flex items-center gap-2">
                      <Badge variant="outline">
                        entrada: {testCase.input_type}
                      </Badge>
                      <Badge variant="outline">
                        saída: {testCase.expected_output_type}
                      </Badge>
                      <Button
                        type="button"
                        variant="destructive"
                        size="xs"
                        onClick={() => {
                          handleDeleteTestCase(testCase.id);
                        }}
                      >
                        Remover
                      </Button>
                    </div>
                  </div>

                  <p className="text-muted-foreground text-xs">
                    Tempo: {testCase.cpu_time_limit_seconds} s · Memória:{" "}
                    {formatBytes(testCase.mem_usage_limit_bytes)} · Arquivos:{" "}
                    {String(testCase.files.length)}
                  </p>

                  <div className="text-muted-foreground text-xs">
                    Visibilidade: entrada{" "}
                    {testCase.show_input ? "visível" : "oculta"}, saída esperada{" "}
                    {testCase.show_expected_output ? "visível" : "oculta"},
                    saída do aluno{" "}
                    {testCase.show_user_output ? "visível" : "oculta"}
                  </div>

                  {testCase.input && (
                    <details>
                      <summary className="cursor-pointer text-sm select-none">
                        Ver entrada
                      </summary>
                      <pre className="bg-muted mt-2 max-h-48 overflow-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                        {testCase.input}
                      </pre>
                    </details>
                  )}
                  {testCase.expected_output && (
                    <details>
                      <summary className="cursor-pointer text-sm select-none">
                        Ver saída esperada
                      </summary>
                      <pre className="bg-muted mt-2 max-h-48 overflow-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                        {testCase.expected_output}
                      </pre>
                    </details>
                  )}
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
