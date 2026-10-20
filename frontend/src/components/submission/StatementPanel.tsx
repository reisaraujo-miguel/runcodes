import {
  ExternalLinkIcon,
  FileTextIcon,
  PanelLeftCloseIcon,
  PanelLeftOpenIcon,
  PaperclipIcon,
} from "lucide-react";
import { useState } from "react";

import { LoadingState } from "@/components/app/states";
import { Markdown } from "@/components/submission/Markdown";
import { Button } from "@/components/ui/button";
import { TabBar, type TabBarItem } from "@/components/ui/tab-bar";
import type { AttachedFile } from "@/lib/api";
import { attachedFileDownloadUrl } from "@/lib/api";
import { extensionOf } from "@/lib/file-types";
import { cn } from "@/lib/utils";

type StatementTab = "statement" | "materials" | "pdf";

/**
 * The left half of the submission workspace: the exercise statement, its
 * attached materials and the PDF statement. Everything lives in one panel with
 * tabs, instead of a stack of cards that pushes the editor down the page.
 */
export function StatementPanel({
  exerciseId,
  description,
  materials,
  materialsLoading,
  materialsError,
  collapsed = false,
  onToggleCollapsed,
  className,
}: {
  exerciseId: number;
  description: string;
  materials: AttachedFile[];
  materialsLoading: boolean;
  materialsError: string | null;
  collapsed?: boolean;
  onToggleCollapsed?: () => void;
  className?: string;
}) {
  const [activeTab, setActiveTab] = useState<StatementTab>("statement");
  const primaryPdf = materials.find(
    (file) => extensionOf(file.filename) === ".pdf",
  );
  // Materials load after the first render; a tab that is no longer available
  // must fall back rather than render an empty panel.
  const tab: StatementTab =
    activeTab === "pdf" && !primaryPdf ? "statement" : activeTab;

  const tabs: TabBarItem<StatementTab>[] = [
    { value: "statement", label: "Enunciado" },
    {
      value: "materials",
      label: "Materiais",
      badge: materials.length > 0 ? materials.length : undefined,
    },
    ...(primaryPdf ? [{ value: "pdf" as const, label: "PDF" }] : []),
  ];

  return (
    <section
      className={cn(
        "bg-card flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl border",
        className,
      )}
    >
      <TabBar
        items={tabs}
        value={tab}
        onChange={setActiveTab}
        label="Conteúdo do exercício"
        idPrefix="statement"
        className={collapsed ? "xl:hidden" : undefined}
        trailing={
          onToggleCollapsed ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="hidden xl:inline-flex"
              aria-label="Recolher enunciado"
              title="Recolher enunciado"
              onClick={onToggleCollapsed}
            >
              <PanelLeftCloseIcon className="size-4.5" />
            </Button>
          ) : undefined
        }
      />

      {tab === "pdf" && primaryPdf ? (
        <div
          role="tabpanel"
          id="statement-panel-pdf"
          aria-labelledby="statement-tab-pdf"
          className={cn(
            "flex min-h-0 flex-1 flex-col overflow-hidden",
            collapsed && "xl:hidden",
          )}
        >
          <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2">
            <FileTextIcon
              className="text-muted-foreground size-3.5 shrink-0"
              aria-hidden
            />
            <p className="min-w-0 flex-1 truncate font-mono text-xs">
              {primaryPdf.filename}
            </p>
            <a
              href={attachedFileDownloadUrl(exerciseId, primaryPdf.id)}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary inline-flex shrink-0 items-center gap-1 text-xs font-medium hover:underline"
            >
              Abrir
              <ExternalLinkIcon className="size-3" aria-hidden />
            </a>
          </div>
          <iframe
            title={`Enunciado em PDF: ${primaryPdf.filename}`}
            src={attachedFileDownloadUrl(exerciseId, primaryPdf.id)}
            className="h-[52vh] w-full border-0 xl:h-auto xl:flex-1"
          />
        </div>
      ) : (
        <div
          role="tabpanel"
          id={`statement-panel-${tab}`}
          aria-labelledby={`statement-tab-${tab}`}
          className={cn(
            "max-h-[52vh] min-h-0 overflow-y-auto px-4 py-3 xl:max-h-none xl:flex-1",
            collapsed && "xl:hidden",
          )}
        >
          {tab === "statement" ? (
            description.trim() ? (
              <Markdown>{description}</Markdown>
            ) : (
              <p className="text-muted-foreground text-sm">
                Este exercício não possui descrição em texto.
              </p>
            )
          ) : materialsLoading ? (
            <LoadingState label="Carregando materiais…" className="py-8" />
          ) : materialsError ? (
            <p className="text-destructive text-xs">
              Não foi possível carregar os materiais: {materialsError}
            </p>
          ) : materials.length === 0 ? (
            <div className="text-muted-foreground flex items-start gap-2 text-sm">
              <PaperclipIcon className="mt-0.5 size-4 shrink-0" aria-hidden />
              <p>Nenhum material anexado a este exercício.</p>
            </div>
          ) : (
            <ul className="divide-border divide-y">
              {materials.map((file) => {
                const isPdf = extensionOf(file.filename) === ".pdf";
                return (
                  <li key={file.id}>
                    <a
                      href={attachedFileDownloadUrl(exerciseId, file.id)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="hover:bg-muted/60 group -mx-2 flex items-center gap-2.5 rounded-md px-2 py-2"
                    >
                      <span className="bg-muted text-muted-foreground flex size-7 shrink-0 items-center justify-center rounded-md">
                        {isPdf ? (
                          <FileTextIcon className="size-3.5" aria-hidden />
                        ) : (
                          <PaperclipIcon className="size-3.5" aria-hidden />
                        )}
                      </span>
                      <span className="min-w-0 flex-1 truncate font-mono text-xs group-hover:underline">
                        {file.filename}
                      </span>
                      <ExternalLinkIcon
                        className="text-muted-foreground size-3.5 shrink-0"
                        aria-hidden
                      />
                    </a>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}

      {collapsed && onToggleCollapsed ? (
        <button
          type="button"
          onClick={onToggleCollapsed}
          title="Expandir enunciado"
          aria-label="Expandir enunciado"
          className="text-muted-foreground hover:bg-muted/40 hover:text-foreground focus-visible:ring-ring/50 hidden h-full min-h-0 w-full flex-1 flex-col items-center gap-4 py-3 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset xl:flex"
        >
          <PanelLeftOpenIcon className="size-5" aria-hidden />
          <span className="text-[11px] font-medium [writing-mode:vertical-rl]">
            Enunciado
          </span>
        </button>
      ) : null}
    </section>
  );
}
