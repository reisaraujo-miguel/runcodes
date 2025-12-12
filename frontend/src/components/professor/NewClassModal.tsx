import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { useState } from "react";
import { useForm } from "react-hook-form";
import axios from "axios";


import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";

import { Button } from "../ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../ui/card";
import { Input } from "../ui/input";

interface NewClassModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const formSchema = z.object({
  Name: z.string().min(1, "O nome da turma é obrigatório"),
  EndDate: z.string().optional(),
});

const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT;

export function NewClassModal({ isOpen, onClose }: NewClassModalProps) {
  if (!isOpen) return null;

  const [wasSubmitted, setSubmitted] = useState(false);

  const form = useForm({
    resolver: zodResolver(formSchema),
    defaultValues: {
      Name: "",
      EndDate: "",
    },
  });

  const onSubmit = async (data: z.infer<typeof formSchema>) => {
    try {
      const response = await axios.post(`${API_BASE_URL}/api/offerings/create`, {
        name: data.Name,
        end_date: data.EndDate,
      });
      if (response.data.success) {
        console.log("Turma criada com sucesso!");
      }
    } catch (error) {
      console.error("Erro ao criar a turma:", error);
      return;
    }
    setSubmitted(true);
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
                <Button onClick={onClose}>Fechar</Button>
              </div>
            </div>
          )}
          {!wasSubmitted && (
            <Form {...form}>
              <form
                onSubmit={form.handleSubmit(onSubmit)}
                className="space-y-4"
              >
                <FormField
                  control={form.control}
                  name="Name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Nome da Turma</FormLabel>
                      <FormControl>
                        <Input
                          placeholder="Digite o nome da turma"
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="EndDate"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Disponível até</FormLabel>
                      <FormControl>
                        <Input type="date" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <div className="flex justify-end pt-4">
                  <Button onClick={onClose} variant="destructive">
                    Fechar
                  </Button>
                  <Button className="ml-2" type="submit" variant="default">
                    Criar Turma
                  </Button>
                </div>
              </form>
            </Form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
