import { useState } from "react";

import { DateTimeField } from "@/components/app/DateTimeField";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { useSubmit } from "@/hooks/use-submit";
import { importOffering, type Offering } from "@/lib/api";
import { dateTimeInputToTimestamp } from "@/lib/format";

/**
 * Restore a class from an archive created with "Exportar turma". The name, the
 * description and the end date may be overridden; moving the end date shifts
 * every exercise by the same amount so the relative schedule survives a new
 * semester.
 */
export function ImportClassModal({
  onClose,
  onImported,
}: {
  onClose: () => void;
  onImported: (offering: Offering) => void;
}) {
  const [archive, setArchive] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [endDate, setEndDate] = useState("");
  const [endTime, setEndTime] = useState("23:59");
  const [endOfDay, setEndOfDay] = useState(true);
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível importar a turma.",
  );

  function handleSubmit() {
    if (!archive) {
      setError("Selecione o arquivo .zip exportado da turma.");
      return;
    }
    if (!archive.name.toLowerCase().endsWith(".zip")) {
      setError("O arquivo precisa ter a extensão .zip.");
      return;
    }

    let endDateIso: string | undefined;
    if (endDate) {
      const parsed = dateTimeInputToTimestamp(endDate, endTime, endOfDay);
      if (!parsed) {
        setError("Data de encerramento inválida.");
        return;
      }
      endDateIso = parsed;
    }

    void run(async () => {
      const offering = await importOffering(archive, {
        name: name.trim() || undefined,
        description: description.trim() || undefined,
        end_date: endDateIso,
      });
      onImported(offering);
    });
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Importar turma</DialogTitle>
          <DialogDescription>
            Crie uma nova turma a partir de um .zip exportado. Os exercícios,
            casos de teste, arquivos de compilação e materiais são restaurados;
            as submissões não fazem parte do backup.
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
            <Label htmlFor="import-class-archive">Arquivo .zip</Label>
            <Input
              id="import-class-archive"
              type="file"
              accept=".zip,application/zip"
              onChange={(event) => {
                setArchive(event.target.files?.item(0) ?? null);
              }}
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="import-class-name">Nome da nova turma</Label>
            <Input
              id="import-class-name"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
              }}
              placeholder="Deixe em branco para manter o nome do arquivo"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="import-class-description">Descrição</Label>
            <Textarea
              id="import-class-description"
              value={description}
              onChange={(event) => {
                setDescription(event.target.value);
              }}
              placeholder="Deixe em branco para manter a descrição original"
            />
          </div>

          <div className="space-y-2">
            <DateTimeField
              id="import-class-end-date"
              label="Novo encerramento (opcional)"
              date={endDate}
              time={endTime}
              onDateChange={setEndDate}
              onTimeChange={setEndTime}
              endOfDay={endOfDay}
              onEndOfDayChange={setEndOfDay}
            />
            <p className="text-muted-foreground text-xs">
              Se informado, os prazos dos exercícios são deslocados pelo mesmo
              intervalo. Sem data, o encerramento original é mantido; se ele já
              passou, a turma é criada com seis meses de prazo.
            </p>
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
            <Button type="submit" disabled={pending || !archive}>
              {pending ? <Spinner className="size-4" /> : null}
              Importar turma
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
