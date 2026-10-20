import { SectionCard } from "@/components/app/SectionCard";
import { LoadingState } from "@/components/app/states";
import type { TestCase } from "@/lib/api/exercises";

/** Read-only preview of the exercise cases, honouring the professor's flags. */
export function TestCasesPreview({
  testCases,
  loading = false,
}: {
  testCases: TestCase[];
  loading?: boolean;
}) {
  return (
    <SectionCard
      title="Casos de teste"
      description="Entradas e saídas que o professor liberou para consulta."
    >
      {loading ? (
        <LoadingState className="py-6" />
      ) : testCases.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          Nenhum caso de teste disponível para consulta.
        </p>
      ) : (
        <div className="space-y-4">
          {testCases.map((testCase) => (
            <div key={testCase.id} className="space-y-2">
              <p className="text-sm font-medium">Caso #{String(testCase.id)}</p>
              {testCase.show_input && testCase.input_type === "text" ? (
                <details>
                  <summary className="text-muted-foreground cursor-pointer text-sm select-none">
                    Ver entrada
                  </summary>
                  <pre className="bg-muted mt-2 max-h-72 overflow-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                    {testCase.input || "(vazio)"}
                  </pre>
                </details>
              ) : (
                <p className="text-muted-foreground text-xs">
                  Entrada não disponível para consulta.
                </p>
              )}
              {testCase.show_expected_output &&
              testCase.expected_output_type === "text" ? (
                <details>
                  <summary className="text-muted-foreground cursor-pointer text-sm select-none">
                    Ver saída esperada
                  </summary>
                  <pre className="bg-muted mt-2 max-h-72 overflow-auto rounded-lg p-3 text-xs whitespace-pre-wrap">
                    {testCase.expected_output || "(vazio)"}
                  </pre>
                </details>
              ) : (
                <p className="text-muted-foreground text-xs">
                  Saída esperada não disponível para consulta.
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </SectionCard>
  );
}
