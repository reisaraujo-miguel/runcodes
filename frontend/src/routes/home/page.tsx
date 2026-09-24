import { useEffect, useState } from "react";

import { CollapsibleSection } from "@/components/CollapsibleSection";

import { Footer } from "@/components/Footer";
import { EnrollForm } from "@/components/home/EnrollForm";
import { MyClasses } from "@/components/home/MyClasses";
import { OpenExercises } from "@/components/home/OpenExercises";
import { Navbar } from "@/components/Navbar";
import { getMyOfferings, type UserOffering } from "@/lib/api";

export function Home() {
  const [classes, setClasses] = useState<UserOffering[]>([]);
  const [classesLoading, setClassesLoading] = useState(true);
  const [classesError, setClassesError] = useState<string | null>(null);
  // Bumped after an enrollment: the class list refetches here and the open
  // exercises component reloads through the same key.
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    let cancelled = false;

    async function loadClasses() {
      setClassesLoading(true);
      setClassesError(null);
      try {
        const data = await getMyOfferings();
        if (!cancelled) setClasses(data);
      } catch {
        if (!cancelled) {
          setClassesError("Não foi possível carregar as suas turmas.");
        }
      } finally {
        if (!cancelled) setClassesLoading(false);
      }
    }

    void loadClasses();

    return () => {
      cancelled = true;
    };
  }, [refreshKey]);

  function handleEnrolled() {
    setRefreshKey((previous) => previous + 1);
  }

  function handleUnenrolled(offeringId: number) {
    setClasses((previous) =>
      previous.filter((offering) => offering.offering_id !== offeringId),
    );
    // The exercises of the class the user just left must disappear with it.
    setRefreshKey((previous) => previous + 1);
  }

  return (
    <div>
      <Navbar />
      <main className="mx-auto max-w-3xl">
        <CollapsibleSection label="Próximas Entregas">
          <OpenExercises refreshKey={refreshKey} />
        </CollapsibleSection>
        <CollapsibleSection label="Minhas Disciplinas">
          <MyClasses
            classes={classes}
            loading={classesLoading}
            error={classesError}
            onUnenrolled={handleUnenrolled}
          />
        </CollapsibleSection>
        <CollapsibleSection label="Nova Matrícula">
          <EnrollForm onEnrolled={handleEnrolled} />
        </CollapsibleSection>
      </main>
      <Footer />
    </div>
  );
}
