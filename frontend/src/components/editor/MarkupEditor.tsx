import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import { html } from "@codemirror/lang-html";
import { markdown } from "@codemirror/lang-markdown";
import { Compartment, EditorState } from "@codemirror/state";
import {
  drawSelection,
  dropCursor,
  EditorView,
  highlightActiveLine,
  keymap,
  placeholder as placeholderExtension,
} from "@codemirror/view";
import DOMPurify from "dompurify";
import { CodeIcon, EyeIcon, FileTextIcon, PencilIcon } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Markdown } from "@/components/submission/Markdown";
import { Button } from "@/components/ui/button";
import { useTheme } from "@/hooks/use-theme";
import { editorTheme } from "@/lib/editor/editor-theme";
import { cn } from "@/lib/utils";

/** The markup formats the prose editor can edit. */
export type MarkupFormat = "markdown" | "html";

const supportByFormat = {
  markdown: markdown(),
  html: html(),
} as const;

const presentationByFormat: Record<
  MarkupFormat,
  {
    label: string;
    icon: typeof FileTextIcon;
    ariaLabel: string;
    placeholder: string;
  }
> = {
  markdown: {
    label: "Markdown",
    icon: FileTextIcon,
    ariaLabel: "Instruções do exercício em Markdown",
    placeholder: "Escreva as instruções do exercício…",
  },
  html: {
    label: "HTML",
    icon: CodeIcon,
    ariaLabel: "Conteúdo HTML",
    placeholder: "Escreva o HTML…",
  },
};

type MarkupMode = "edit" | "preview";

export interface MarkupEditorProps {
  format: MarkupFormat;
  value: string;
  onChange: (value: string) => void;
  ariaLabel?: string;
  placeholder?: string;
  /** Height shared by the editor and the preview. */
  heightClassName?: string;
  className?: string;
  /** Renders the content without accepting edits, e.g. while saving. */
  readOnly?: boolean;
  /**
   * False while the surrounding tab is hidden. The draft stays with the parent,
   * but the CodeMirror view is not created inside a zero-size container.
   */
  enabled?: boolean;
}

/**
 * A compact CodeMirror editor for the Markdown and HTML written by professors
 * and admins, with syntax highlighting and a rendered preview. It uses the same
 * theme as the code editor so prose feels like part of the same workspace.
 * HTML previews are sanitized before being rendered.
 */
export function MarkupEditor({
  format,
  value,
  onChange,
  ariaLabel,
  placeholder,
  heightClassName = "h-72",
  className,
  readOnly = false,
  enabled = true,
}: MarkupEditorProps) {
  const theme = useTheme();
  const [mode, setMode] = useState<MarkupMode>("edit");
  const [readOnlyCompartment] = useState(() => new Compartment());
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const valueRef = useRef(value);
  const onChangeRef = useRef(onChange);
  const readOnlyRef = useRef(readOnly);
  const presentation = presentationByFormat[format];
  const resolvedAriaLabel = ariaLabel ?? presentation.ariaLabel;
  const resolvedPlaceholder = placeholder ?? presentation.placeholder;
  const FormatIcon = presentation.icon;

  useEffect(() => {
    valueRef.current = value;
  }, [value]);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    readOnlyRef.current = readOnly;
  }, [readOnly]);

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
          supportByFormat[format],
          EditorView.lineWrapping,
          placeholderExtension(resolvedPlaceholder),
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          readOnlyCompartment.of(readOnlyExtensions(readOnlyRef.current)),
          EditorView.contentAttributes.of({
            "aria-label": resolvedAriaLabel,
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
  }, [
    enabled,
    format,
    mode,
    readOnlyCompartment,
    resolvedAriaLabel,
    resolvedPlaceholder,
    theme,
  ]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: readOnlyCompartment.reconfigure(readOnlyExtensions(readOnly)),
    });
  }, [readOnly, readOnlyCompartment]);

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
          <FormatIcon className="size-3.5" aria-hidden />
          {presentation.label}
        </span>
        <div
          role="group"
          aria-label={`Modo de visualização do conteúdo em ${presentation.label}`}
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
            format === "markdown" ? (
              <Markdown>{value}</Markdown>
            ) : (
              <SanitizedHtml source={value} />
            )
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

/** Keeps the editor locked while the surrounding form is saving. */
function readOnlyExtensions(readOnly: boolean) {
  return [EditorState.readOnly.of(readOnly), EditorView.editable.of(!readOnly)];
}

/** Renders the HTML preview after removing anything unsafe to insert. */
function SanitizedHtml({ source }: { source: string }) {
  const sanitized = useMemo(() => DOMPurify.sanitize(source), [source]);

  return (
    <div
      className="markdown-body"
      // The HTML comes from the admin editor and is sanitized above.
      // eslint-disable-next-line react-dom/no-dangerously-set-innerhtml
      dangerouslySetInnerHTML={{ __html: sanitized }}
    />
  );
}
