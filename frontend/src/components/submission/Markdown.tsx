import { marked } from "marked";
import { useMemo } from "react";

import { sanitizeRichText } from "@/lib/rich-text";
import { cn } from "@/lib/utils";

marked.setOptions({ gfm: true, breaks: true });

/** Renders markdown as sanitized HTML, styled by `.markdown-body`. */
export function Markdown({
  children,
  className,
}: {
  children: string;
  className?: string;
}) {
  const html = useMemo(
    () => sanitizeRichText(marked.parse(children, { async: false })),
    [children],
  );

  return (
    <div
      className={cn("markdown-body", className)}
      /* The HTML is produced by marked and sanitized by DOMPurify above. */
      /* eslint-disable-next-line react-dom/no-dangerously-set-innerhtml */
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
