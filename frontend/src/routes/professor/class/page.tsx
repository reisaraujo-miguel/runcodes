import { useEffect, useState } from "react";
import { useLocation, useParams } from "react-router";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

import { getOffering, type Offering } from "@/lib/api/offerings";

/**
 * Page for a class offering. After creating a class, the offering is passed
 * through navigation state to avoid a loading flash; the page always
 * (re)fetches from the API so it also works on refresh/direct visits.
 */
export function ClassPage() {
  const location = useLocation();
  const { offeringId } = useParams();
  const initialOffering = (location.state as Offering | null) ?? null;
  const [offering, setOffering] = useState<Offering | null>(initialOffering);
  const [loading, setLoading] = useState(initialOffering === null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const id = Number(offeringId);

    async function loadOffering() {
      if (!Number.isInteger(id) || id <= 0) {
        setError("Turma inválida");
        setLoading(false);
        return;
      }
      try {
        const data = await getOffering(id);
        if (!cancelled) setOffering(data);
      } catch {
        if (!cancelled) setError("Turma não encontrada");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadOffering();

    return () => {
      cancelled = true;
    };
  }, [offeringId]);

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

  if (!offering) {
    return (
      <div className="mx-auto max-w-3xl p-6">
        <Card>
          <CardHeader>
            <CardTitle>Turma não encontrada</CardTitle>
            <CardDescription>
              {error ?? "Não foi possível carregar os dados desta turma."}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  const parsedEndDate = offering.end_date ? new Date(offering.end_date) : null;
  const endDate =
    parsedEndDate && !Number.isNaN(parsedEndDate.getTime())
      ? parsedEndDate.toLocaleString(undefined, {
          dateStyle: "long",
          timeStyle: "short",
        })
      : null;

  return (
    <div className="mx-auto max-w-3xl p-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">{offering.name}</CardTitle>
          {offering.description && (
            <CardDescription>{offering.description}</CardDescription>
          )}
        </CardHeader>
        <CardContent className="space-y-6">
          <div>
            <p className="text-sm text-muted-foreground">Código de matrícula</p>
            <p className="font-mono text-lg tracking-widest">
              {offering.enrollment_code}
            </p>
          </div>
          {endDate && (
            <div>
              <p className="text-sm text-muted-foreground">Disponível até</p>
              <p>{endDate}</p>
            </div>
          )}
          <p className="text-sm text-muted-foreground">
            Os exercícios desta turma aparecerão aqui em breve.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
