import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags } from "@lezer/highlight";

/**
 * Syntax colors come from the app's design tokens, so every editor follows the
 * light/dark theme without a second palette to maintain. The Markdown and HTML
 * tags are appended to the code tags because the prose editor shares the same
 * CodeMirror surface.
 */
const highlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: "var(--primary)", fontWeight: "600" },
  {
    tag: [
      tags.controlKeyword,
      tags.operatorKeyword,
      tags.moduleKeyword,
      tags.definitionKeyword,
    ],
    color: "var(--primary)",
  },
  {
    tag: [tags.function(tags.variableName), tags.labelName],
    color: "var(--info)",
  },
  {
    tag: [tags.typeName, tags.className, tags.namespace],
    color: "var(--warning)",
  },
  {
    tag: [tags.number, tags.integer, tags.float, tags.constant(tags.name)],
    color: "var(--warning)",
  },
  { tag: [tags.string, tags.special(tags.string)], color: "var(--success)" },
  {
    tag: [tags.comment, tags.lineComment, tags.blockComment],
    color: "var(--muted-foreground)",
    fontStyle: "italic",
  },
  { tag: [tags.operator, tags.punctuation], color: "var(--muted-foreground)" },
  { tag: [tags.bool, tags.atom], color: "var(--destructive)" },
  { tag: tags.invalid, color: "var(--destructive)" },

  // Markdown
  { tag: tags.heading, color: "var(--primary)", fontWeight: "600" },
  { tag: tags.strong, fontWeight: "700" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strikethrough, textDecoration: "line-through" },
  {
    tag: [tags.link, tags.url],
    color: "var(--info)",
    textDecoration: "underline",
  },
  { tag: tags.monospace, color: "var(--success)" },
  {
    tag: tags.quote,
    color: "var(--muted-foreground)",
    fontStyle: "italic",
  },
  {
    tag: [tags.meta, tags.contentSeparator],
    color: "var(--muted-foreground)",
  },

  // HTML
  {
    tag: [tags.tagName, tags.angleBracket],
    color: "var(--primary)",
  },
  { tag: tags.attributeName, color: "var(--warning)" },
  { tag: tags.attributeValue, color: "var(--success)" },
  {
    tag: [tags.character, tags.processingInstruction, tags.documentMeta],
    color: "var(--muted-foreground)",
  },
]);

function themeSpec(dark: boolean) {
  return EditorView.theme(
    {
      "&": {
        height: "100%",
        color: "var(--foreground)",
        backgroundColor: "var(--card)",
        fontSize: "13px",
      },
      "&.cm-focused": { outline: "none" },
      ".cm-scroller": {
        fontFamily: "var(--font-mono)",
        lineHeight: "1.6",
      },
      ".cm-content": {
        padding: "12px 0",
        caretColor: "var(--primary)",
      },
      ".cm-gutters": {
        backgroundColor: "var(--muted)",
        color: "var(--muted-foreground)",
        border: "none",
        borderRight: "1px solid var(--border)",
      },
      ".cm-activeLine": {
        backgroundColor: "color-mix(in oklab, var(--primary) 6%, transparent)",
      },
      ".cm-activeLineGutter": {
        backgroundColor: "color-mix(in oklab, var(--primary) 10%, transparent)",
        color: "var(--foreground)",
      },
      ".cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection":
        {
          backgroundColor:
            "color-mix(in oklab, var(--primary) 24%, transparent)",
        },
      ".cm-cursor, .cm-dropCursor": {
        borderLeftColor: "var(--primary)",
      },
      ".cm-panels": {
        backgroundColor: "var(--card)",
        color: "var(--foreground)",
      },
      ".cm-tooltip": {
        backgroundColor: "var(--popover)",
        border: "1px solid var(--border)",
        color: "var(--popover-foreground)",
      },
      ".cm-placeholder": { color: "var(--muted-foreground)" },
      ".cm-foldPlaceholder": {
        backgroundColor: "var(--muted)",
        border: "none",
        color: "var(--muted-foreground)",
      },
    },
    { dark },
  );
}

/** Shared CodeMirror theme and syntax colors for code and Markdown editors. */
export function editorTheme(dark: boolean) {
  return [themeSpec(dark), syntaxHighlighting(highlightStyle)];
}
