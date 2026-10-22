import DOMPurify from "dompurify";

/**
 * The sanitizer policy for author-written rich text: exercise statements,
 * markdown previews and the admin's contact disclaimer.
 *
 * DOMPurify's HTML profile is intentionally broad. It allows `<style>`, form
 * controls and the `action`/`style` attributes, which lets staff-authored
 * content restyle the app or embed a credential-collecting form inside the
 * trusted origin. Those tags are not needed by exercise statements, so they are
 * forbidden here instead of being allowed by default.
 */
const RICH_TEXT_CONFIG = {
  USE_PROFILES: { html: true },
  FORBID_TAGS: ["style", "form", "input", "button", "textarea", "select"],
  FORBID_ATTR: ["style", "formaction"],
} satisfies Parameters<typeof DOMPurify.sanitize>[1];

/** Sanitizes author-written HTML for insertion into the page. */
export function sanitizeRichText(html: string): string {
  return DOMPurify.sanitize(html, RICH_TEXT_CONFIG);
}
