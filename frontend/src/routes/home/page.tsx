import { CollapsibleSection } from "@/components/CollapsibleSection";

import { Footer } from "@/components/Footer";
import { Navbar } from "@/components/Navbar";

export function Home() {
  return (
    <div>
      <Navbar />
      <main>
        <CollapsibleSection label="Próximas Entregas"></CollapsibleSection>
        <CollapsibleSection label="Minhas Disciplinas"></CollapsibleSection>
        <CollapsibleSection label="Nova Matrícula"></CollapsibleSection>
      </main>
      <Footer />
    </div>
  );
}
