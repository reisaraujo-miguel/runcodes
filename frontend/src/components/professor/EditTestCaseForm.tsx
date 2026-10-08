import { useState } from "react";

import {
  draftFromTestCase,
  draftToPatch,
  FILE_INPUT,
  testCaseFileName,
  type TestCaseDraft,
} from "@/components/professor/test-case-draft";
import { TestCaseFields } from "@/components/professor/test-case-fields";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import { updateTestCase, type TestCase } from "@/lib/api";

/**
 * Inline form to edit a test case. The case's input/output types are fixed, so
 * only the values, the visibility flags and the limits are editable. A file side
 * keeps its stored file unless a replacement is picked; extra files are replaced
 * as a whole (the API cannot clear them), which the form states explicitly.
 */
export function EditTestCaseForm({
  exerciseId,
  testCase,
  onSaved,
  onCancel,
}: {
  exerciseId: number;
  testCase: TestCase;
  onSaved: (testCase: TestCase) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState<TestCaseDraft>(() =>
    draftFromTestCase(testCase),
  );
  const { pending, error, run } = useSubmit(
    "Não foi possível salvar o caso de teste.",
  );

  function update(patch: Partial<TestCaseDraft>) {
    setDraft((previous) => ({ ...previous, ...patch }));
  }

  function handleSubmit() {
    const patch = draftToPatch(draft, testCase);
    if (Object.keys(patch).length === 0) {
      onSaved(testCase);
      return;
    }

    void run(async () => {
      onSaved(await updateTestCase(exerciseId, testCase.id, patch));
    });
  }

  const storedFiles = testCase.files;

  const keepFileHint = (
    <p className="text-muted-foreground text-xs">
      O arquivo atual é mantido se você não selecionar outro. Não é possível
      trocar de texto para arquivo (nem o contrário) depois de criado.
    </p>
  );

  return (
    <div className="bg-muted/40 space-y-4 rounded-lg border p-4">
      <h3 className="text-sm font-medium">
        Editar caso #{String(testCase.id)}
      </h3>

      <div className="text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs">
        <span>
          Entrada: {testCase.input_type === FILE_INPUT ? "Arquivo" : "Texto"}
        </span>
        <span>
          Saída esperada:{" "}
          {testCase.expected_output_type === FILE_INPUT ? "Arquivo" : "Texto"}
        </span>
      </div>

      <form
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          handleSubmit();
        }}
      >
        <TestCaseFields
          idPrefix={`edit-case-${String(testCase.id)}`}
          draft={draft}
          onChange={update}
          typeSelectors={false}
          inputFileHint={
            testCase.input_type === FILE_INPUT ? keepFileHint : undefined
          }
          outputFileHint={
            testCase.expected_output_type === FILE_INPUT
              ? keepFileHint
              : undefined
          }
          extraFilesHint={
            <p className="text-muted-foreground text-xs">
              {storedFiles.length > 0
                ? `Este caso tem ${String(storedFiles.length)} arquivo(s) adicional(is): ${storedFiles
                    .map((file, index) => testCaseFileName(file, index))
                    .join(", ")}. `
                : ""}
              Enviar novos arquivos substitui todos os atuais; não é possível
              removê-los individualmente.
            </p>
          }
        />

        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}

        <div className="flex flex-wrap gap-2">
          <Button type="submit" disabled={pending}>
            {pending ? <Spinner className="size-4" /> : null}
            Salvar
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={onCancel}
          >
            Cancelar
          </Button>
        </div>
      </form>
    </div>
  );
}
