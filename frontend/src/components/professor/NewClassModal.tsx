import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { useState } from "react";
import { useForm } from "react-hook-form";

import { NavLink } from "react-router";

import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

const formSchema = z.object({
  Name: z.string().min(1, "O nome da turma é obrigatório"),
  EndDate: z.string().optional(),
  Description: z.string().optional(),
});

const apiErrorSchema = z.object({
  error_msg: z.string(),
});

const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT;

export function NewClassModal() {
  const [wasSubmitted, setSubmitted] = useState(false);

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
      const response = await fetch(`${API_BASE_URL}/api/v1/offerings/create`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          //"Authorization": `Bearer ${getAuthToken()}`,  //TODO: Implement token retrieval
        },
        body: JSON.stringify({
          name: data.Name,
          end_date: data.EndDate,
          description: data.Description,
        }),
      });
      if (response.ok) {
        console.log("Turma criada com sucesso!");
        setSubmitted(true);
      } else {
        const rawData = await response.json();
        const parsed = apiErrorSchema.safeParse(rawData);
        const message: string = parsed.success
          ? parsed.data.error_msg
          : "Erro desconhecido";
        console.error("Erro ao criar a turma:", message);
      }
    } catch (error) {
      console.error("Erro ao criar a turma:", error);
      return;
    }
  };

  return (
    <div className="fixed inset-0 bg-black/75 flex items-center justify-center z-50 p-4">
      <Card className="w-full max-w-2xl max-h-[80vh] overflow-y-auto">
        <CardHeader className="top-0 border-b">
          <CardTitle className="text-2xl">Criar Nova Turma</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 pt-4">
          {wasSubmitted && (
            <div>
              <div
                className="p-4 mb-4 text-sm text-green-700 bg-green-100 rounded-lg"
                role="alert"
              >
                Turma criada com sucesso!
              </div>
              <div className="flex justify-end pt-4">
                <NavLink
                  to="/professor"
                  className={buttonVariants({ variant: "default" })}
                >
                  Fechar
                </NavLink>
              </div>
            </div>
          )}
          {!wasSubmitted && (
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
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
                  />
                </Field>

                <Field>
                  <FieldLabel htmlFor="Description">Descrição</FieldLabel>
                  <Input
                    id="Description"
                    placeholder="Digite uma descrição (opcional)"
                    {...form.register("Description")}
                  />
                </Field>

                <Field>
                  <div className="flex justify-end pt-4">
                    <NavLink
                      to="/professor"
                      className={buttonVariants({ variant: "destructive" })}
                    >
                      Fechar
                    </NavLink>
                    <Button className="ml-2" type="submit" variant="default">
                      Criar Turma
                    </Button>
                  </div>
                </Field>
              </FieldGroup>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
