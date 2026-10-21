import { closeBrackets, closeBracketsKeymap } from "@codemirror/autocomplete";
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import { bracketMatching, indentUnit } from "@codemirror/language";
import { EditorState, type Extension } from "@codemirror/state";
import {
  drawSelection,
  dropCursor,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
  placeholder,
} from "@codemirror/view";
import { useEffect, useRef, useState } from "react";

import { useTheme } from "@/hooks/use-theme";
import { editorTheme } from "@/lib/editor/editor-theme";
import { loadLanguageSupport } from "@/lib/editor/language-support";
import type { EditorLanguage } from "@/lib/editor/languages";
import { cn } from "@/lib/utils";

export interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  language: EditorLanguage;
  ariaLabel?: string;
  className?: string;
}

/**
 * A CodeMirror 6 editor. The view is recreated when the language or theme
 * changes (rare), while external value changes are applied without resetting
 * the selection during normal typing. The grammar is fetched on demand so the
 * editor core and the grammars stay in separate chunks.
 */
export function CodeEditor({
  value,
  onChange,
  language,
  ariaLabel = "Editor de código",
  className,
}: CodeEditorProps) {
  const theme = useTheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const valueRef = useRef(value);
  const onChangeRef = useRef(onChange);
  const [loaded, setLoaded] = useState<{
    extension: string;
    support: Extension;
  } | null>(null);

  useEffect(() => {
    let disposed = false;
    void loadLanguageSupport(language.extension).then(
      (support) => {
        if (!disposed) setLoaded({ extension: language.extension, support });
      },
      () => {
        if (!disposed)
          setLoaded({ extension: language.extension, support: [] });
      },
    );
    return () => {
      disposed = true;
    };
  }, [language.extension]);

  const support =
    loaded?.extension === language.extension ? loaded.support : null;

  useEffect(() => {
    valueRef.current = value;
  }, [value]);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    const parent = containerRef.current;
    if (!parent || !support) return;

    const view = new EditorView({
      state: EditorState.create({
        doc: valueRef.current,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          history(),
          drawSelection(),
          dropCursor(),
          bracketMatching(),
          closeBrackets(),
          indentUnit.of("    "),
          EditorState.tabSize.of(4),
          keymap.of([
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...historyKeymap,
            indentWithTab,
          ]),
          placeholder("Escreva a sua solução aqui…"),
          EditorView.contentAttributes.of({
            "aria-label": ariaLabel,
            spellcheck: "false",
            autocapitalize: "off",
            autocomplete: "off",
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current(update.state.doc.toString());
            }
          }),
          support,
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
  }, [ariaLabel, support, theme]);

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
      ref={containerRef}
      className={cn(
        "border-border h-96 overflow-hidden rounded-lg border",
        className,
      )}
    />
  );
}
