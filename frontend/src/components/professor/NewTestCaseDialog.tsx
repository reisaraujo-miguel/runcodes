import { useState } from "react";

import { PlusIcon } from "lucide-react";

import {
  draftToNewTestCase,
  emptyDraft,
  FILE_INPUT,
  type TestCaseDraft,
} from "@/components/professor/test-case-draft";
import { TestCaseFields } from "@/components/professor/test-case-fields";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import { createTestCase, type TestCase } from "@/lib/api";

/**
 * Dialog to create test cases one after another, using text or uploaded files
 * for each side.
 *
 * A creation clears the form and keeps the dialog open, so a professor can fill
 * several cases without reopening it; only the explicit close button ends the
 * session. Every creation is reported in place, and the caller reloads the list.
 */
export function NewTestCaseDialog({
  exerciseId,
  onClose,
  onCreated,
}: {
  exerciseId: number;
  onClose: () => void;
  onCreated: (testCase: TestCase) => void;
}) {
  const [draft, setDraft] = useState<TestCaseDraft>(emptyDraft);
  const [created, setCreated] = useState(0);
  // The native file inputs keep their value when the draft object is replaced,
  // so the fields are remounted to really clear them for the next case.
  const [fieldsKey, setFieldsKey] = useState(0);
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
      const testCase = await createTestCase(
        exerciseId,
        draftToNewTestCase(draft),
      );
      setDraft(emptyDraft());
      setFieldsKey((current) => current + 1);
      setCreated((current) => current + 1);
      onCreated(testCase);
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
          <DialogTitle>Novo caso de teste</DialogTitle>
          <DialogDescription>
            Preencha a entrada e a saída esperada, em texto ou como arquivo. O
            caso é adicionado sem fechar a janela, então você pode incluir
            vários em sequência; use <strong>Fechar</strong> quando terminar.
          </DialogDescription>
        </DialogHeader>

        {created > 0 ? (
          <Alert variant="success">
            <AlertDescription>
              {created === 1
                ? "Caso de teste adicionado."
                : `${String(created)} casos de teste adicionados nesta janela.`}{" "}
              O formulário foi limpo; preencha outro caso ou feche a janela.
            </AlertDescription>
          </Alert>
        ) : null}

        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            handleSubmit();
          }}
        >
          <TestCaseFields
            key={fieldsKey}
            idPrefix="new-case"
            draft={draft}
            onChange={update}
          />

          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Fechar
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? <Spinner /> : <PlusIcon />}
              Adicionar caso de teste
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
