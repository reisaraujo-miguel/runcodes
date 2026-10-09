import { useState } from "react";

import {
  draftToNewTestCase,
  emptyDraft,
  FILE_INPUT,
  type TestCaseDraft,
} from "@/components/professor/test-case-draft";
import { TestCaseFields } from "@/components/professor/test-case-fields";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import { createTestCase, type TestCase } from "@/lib/api";

/**
 * Inline form to create a test case, using text or uploaded files for each side.
 */
export function NewTestCaseForm({
  exerciseId,
  onCreated,
}: {
  exerciseId: number;
  onCreated: (testCase: TestCase) => void;
}) {
  const [draft, setDraft] = useState<TestCaseDraft>(emptyDraft);
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível criar o caso de teste.",
  );

  function update(patch: Partial<TestCaseDraft>) {
    setDraft((previous) => ({ ...previous, ...patch }));
  }

  function handleSubmit() {
    if (draft.inputType === FILE_INPUT && !draft.inputFile) {
      setError("Selecione o arquivo de entrada.");
      return;
    }
    if (draft.outputType === FILE_INPUT && !draft.outputFile) {
      setError("Selecione o arquivo de saída esperada.");
      return;
    }

    void run(async () => {
      const created = await createTestCase(
        exerciseId,
        draftToNewTestCase(draft),
      );
      onCreated(created);
      setDraft(emptyDraft());
    });
  }

  return (
    <div className="bg-muted/40 space-y-4 rounded-lg border p-4">
      <h3 className="text-sm font-medium">Novo caso de teste</h3>

      <form
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          handleSubmit();
        }}
      >
        <TestCaseFields idPrefix="new-case" draft={draft} onChange={update} />

        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}

        <Button type="submit" disabled={pending}>
          {pending ? <Spinner className="size-4" /> : null}
          Adicionar caso de teste
        </Button>
      </form>
    </div>
  );
}
