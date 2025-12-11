import { CollapsibleSection } from "@/components/CollapsibleSection";

export function Home() {
  return (
    <div>
      <CollapsibleSection label="Admin Info"></CollapsibleSection>
      <CollapsibleSection label="Estatísticas"></CollapsibleSection>
      <CollapsibleSection label="Próximas Entregas"></CollapsibleSection>
      <CollapsibleSection label="Minhas Disciplinas"></CollapsibleSection>
      <CollapsibleSection label="Nova Matrícula"></CollapsibleSection>
    </div>
  );
}
