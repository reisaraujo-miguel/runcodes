import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { EditorState } from "@codemirror/state";
import {
  drawSelection,
  dropCursor,
  EditorView,
  highlightActiveLine,
  keymap,
  placeholder as placeholderExtension,
} from "@codemirror/view";
import { EyeIcon, FileTextIcon, PencilIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { Markdown } from "@/components/submission/Markdown";
import { Button } from "@/components/ui/button";
import { useTheme } from "@/hooks/use-theme";
import { editorTheme } from "@/lib/editor/editor-theme";
import { cn } from "@/lib/utils";

const markdownSupport = markdown();

type MarkdownMode = "edit" | "preview";

export interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  ariaLabel?: string;
  placeholder?: string;
  /** Height shared by the editor and the preview. */
  heightClassName?: string;
  className?: string;
  /**
   * False while the surrounding tab is hidden. The draft stays with the parent,
   * but the CodeMirror view is not created inside a zero-size container.
   */
  enabled?: boolean;
}

/**
 * A compact Markdown editor with syntax highlighting and a rendered preview.
 * It uses the same CodeMirror theme as the code editor so instructions feel
 * like part of the same workspace.
 */
export function MarkdownEditor({
  value,
  onChange,
  ariaLabel = "Instruções do exercício em Markdown",
  placeholder = "Escreva as instruções do exercício…",
  heightClassName = "h-72",
  className,
  enabled = true,
}: MarkdownEditorProps) {
  const theme = useTheme();
  const [mode, setMode] = useState<MarkdownMode>("edit");
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const valueRef = useRef(value);
  const onChangeRef = useRef(onChange);

  useEffect(() => {
    valueRef.current = value;
  }, [value]);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    const parent = containerRef.current;
    if (!parent || mode !== "edit" || !enabled) return;

    const view = new EditorView({
      state: EditorState.create({
        doc: valueRef.current,
        extensions: [
          history(),
          drawSelection(),
          dropCursor(),
          highlightActiveLine(),
          markdownSupport,
          EditorView.lineWrapping,
          placeholderExtension(placeholder),
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          EditorView.contentAttributes.of({
            "aria-label": ariaLabel,
            spellcheck: "true",
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current(update.state.doc.toString());
            }
          }),
          ...editorTheme(theme === "dark"),
        ],
      }),
      parent,
    });

    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, [ariaLabel, enabled, mode, placeholder, theme]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current === value) return;
    view.dispatch({
      changes: { from: 0, to: current.length, insert: value },
    });
  }, [value]);

  return (
    <div
      className={cn(
        "border-border overflow-hidden rounded-xl border",
        className,
      )}
    >
      <div className="bg-muted/30 flex items-center justify-between gap-2 border-b px-3 py-1.5">
        <span className="text-muted-foreground inline-flex items-center gap-1.5 text-xs font-medium">
          <FileTextIcon className="size-3.5" aria-hidden />
          Markdown
        </span>
        <div
          role="group"
          aria-label="Modo de visualização das instruções"
          className="flex items-center gap-1"
        >
          <Button
            type="button"
            size="xs"
            variant={mode === "edit" ? "secondary" : "ghost"}
            aria-pressed={mode === "edit"}
            onClick={() => {
              setMode("edit");
            }}
          >
            <PencilIcon />
            Editar
          </Button>
          <Button
            type="button"
            size="xs"
            variant={mode === "preview" ? "secondary" : "ghost"}
            aria-pressed={mode === "preview"}
            onClick={() => {
              setMode("preview");
            }}
          >
            <EyeIcon />
            Prévia
          </Button>
        </div>
      </div>

      {mode === "edit" ? (
        <div ref={containerRef} className={cn("w-full", heightClassName)} />
      ) : (
        <div className={cn("overflow-y-auto px-4 py-3", heightClassName)}>
          {value.trim() ? (
            <Markdown>{value}</Markdown>
          ) : (
            <p className="text-muted-foreground text-sm">
              Nada para visualizar ainda.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
