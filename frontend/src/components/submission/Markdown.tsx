import DOMPurify from "dompurify";
import { marked } from "marked";
import { useMemo } from "react";

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
    () =>
      DOMPurify.sanitize(marked.parse(children, { async: false }), {
        USE_PROFILES: { html: true },
      }),
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
