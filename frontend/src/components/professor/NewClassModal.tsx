import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { createOffering } from "@/lib/api/offerings";

const formSchema = z.object({
  Name: z.string().min(1, "O nome da turma é obrigatório"),
  EndDate: z.string().min(1, "A data limite é obrigatória"),
  Description: z.string().optional(),
});

/**
 * Converts a date-only value (YYYY-MM-DD) from a date input into an ISO 8601
 * timestamp representing the end of that day in the user's local timezone.
 * Sending the instant (instead of a date-only string) lets the backend store
 * a timezone-aware end date that can be shown in other users' timezones.
 */
function toEndOfDayTimestamp(dateOnly: string): string {
  const [year, month, day] = dateOnly.split("-");
  if (!year || !month || !day) {
    throw new Error(`invalid date value: ${dateOnly}`);
  }
  return new Date(
    Number(year),
    Number(month) - 1,
    Number(day),
    23,
    59,
    59,
  ).toISOString();
}

/**
 * Modal for creating a new class offering. Rendered on top of the current
 * page (no route navigation) and navigates to the new class page on success.
 */
export function NewClassModal({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate();
  const [apiError, setApiError] = useState<string | null>(null);

  const form = useForm({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Name: "",
      EndDate: "",
      Description: "",
    },
  });

  const onSubmit = async (data: z.infer<typeof formSchema>) => {
    try {
      setApiError(null);
      const offering = await createOffering({
        name: data.Name,
        end_date: toEndOfDayTimestamp(data.EndDate),
        description: data.Description,
      });
      // Open the newly created class page.
      void navigate(`/professor/class/${String(offering.id)}`, {
        state: offering,
      });
      onClose();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Erro ao criar a turma";
      setApiError(message);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="sm:max-w-2xl max-h-[calc(100vh-2rem)] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Criar Nova Turma</DialogTitle>
          <DialogDescription>
            Preencha os dados para criar uma nova turma.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(...args) => void form.handleSubmit(onSubmit)(...args)}
          className="space-y-4"
        >
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="Name">Nome da Turma</FieldLabel>
              <Input
                id="Name"
                placeholder="Digite o nome da turma"
                {...form.register("Name")}
                required
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="EndDate">Disponível até</FieldLabel>
              <Input
                id="EndDate"
                type="date"
                {...form.register("EndDate")}
                required
              />
              {form.formState.errors.EndDate && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.EndDate.message}
                </p>
              )}
            </Field>

            <Field>
              <FieldLabel htmlFor="Description">Descrição</FieldLabel>
              <Input
                id="Description"
                placeholder="Digite uma descrição (opcional)"
                {...form.register("Description")}
              />
            </Field>
          </FieldGroup>

          {apiError && (
            <p className="text-sm text-destructive text-center">{apiError}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="destructive" onClick={onClose}>
              Fechar
            </Button>
            <Button type="submit">Criar Turma</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
