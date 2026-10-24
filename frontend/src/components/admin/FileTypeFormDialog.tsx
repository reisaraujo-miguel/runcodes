import { useState } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
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
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import {
  adminCreateFileType,
  adminUpdateFileType,
  type AdminFileType,
} from "@/lib/api";

/**
 * Create or edit one entry of the allowed file type catalog. The backend
 * normalises the extension (no leading dot, lower case), so the form shows the
 * plain value and lets the professor-facing picker use it immediately.
 */
export function FileTypeFormDialog({
  fileType,
  onClose,
  onSaved,
}: {
  /** Absent when registering a new file type. */
  fileType?: AdminFileType;
  onClose: () => void;
  onSaved: (fileType: AdminFileType) => void;
}) {
  const editing = fileType !== undefined;
  const [name, setName] = useState(fileType?.name ?? "");
  const [extension, setExtension] = useState(fileType?.extension ?? "");
  const [isCompilable, setIsCompilable] = useState(
    fileType?.is_compilable ?? true,
  );
  const [isAvailable, setIsAvailable] = useState(
    fileType?.is_available ?? true,
  );
  const { pending, error, setError, run } = useSubmit(
    editing
      ? "Não foi possível atualizar o tipo de arquivo."
      : "Não foi possível cadastrar o tipo de arquivo.",
  );

  function handleSubmit() {
    const trimmedName = name.trim();
    const trimmedExtension = extension.trim().replace(/^\./, "");
    if (!trimmedName) {
      setError("Informe o nome do tipo de arquivo.");
      return;
    }
    if (!trimmedExtension) {
      setError("Informe a extensão (por exemplo, py ou omp.cpp).");
      return;
    }

    void run(async () => {
      const saved = fileType
        ? await adminUpdateFileType(fileType.id, {
            name: trimmedName,
            extension: trimmedExtension,
            is_compilable: isCompilable,
            is_available: isAvailable,
          })
        : await adminCreateFileType({
            name: trimmedName,
            extension: trimmedExtension,
            is_compilable: isCompilable,
            is_available: isAvailable,
          });
      onSaved(saved);
    });
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {editing ? "Editar tipo de arquivo" : "Novo tipo de arquivo"}
          </DialogTitle>
          <DialogDescription>
            O tipo fica disponível para os professores imediatamente, sem
            redeploy.
          </DialogDescription>
        </DialogHeader>

        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            handleSubmit();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="file-type-name">Nome</Label>
            <Input
              id="file-type-name"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
              }}
              placeholder="Ex.: Python 3.13"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="file-type-extension">Extensão</Label>
            <Input
              id="file-type-extension"
              value={extension}
              onChange={(event) => {
                setExtension(event.target.value);
              }}
              placeholder="py"
              required
            />
            <p className="text-muted-foreground text-xs">
              Sem o ponto. Extensões compostas são aceitas, como{" "}
              <code>omp.cpp</code>.
            </p>
          </div>

          <div className="flex flex-col gap-3">
            <Label htmlFor="file-type-compilable" className="font-normal">
              <Checkbox
                id="file-type-compilable"
                checked={isCompilable}
                onCheckedChange={(checked) => {
                  setIsCompilable(checked);
                }}
              />
              O código é compilado antes de executar
            </Label>
            <Label htmlFor="file-type-available" className="font-normal">
              <Checkbox
                id="file-type-available"
                checked={isAvailable}
                onCheckedChange={(checked) => {
                  setIsAvailable(checked);
                }}
              />
              Disponível para novos exercícios
            </Label>
          </div>

          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={onClose}
            >
              Cancelar
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? <Spinner className="size-4" /> : null}
              {editing ? "Salvar" : "Cadastrar"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
