import { useMemo, useState } from "react";

import DOMPurify from "dompurify";

import { usePlatformSettings } from "@/hooks/use-platform-settings";

import { TermsModal } from "./TermsModal";

/** The marketing panel shown beside the sign-in card. */
export function AboutSection() {
  const [isTermsModalOpen, setIsTermsModalOpen] = useState(false);
  const { contact_disclaimer_html: contactDisclaimerHtml } =
    usePlatformSettings();

  // The disclaimer is admin-editable content that arrives from the API, so it
  // is sanitized whenever it changes rather than once at module scope.
  const sanitizedContactDisclaimer = useMemo(
    () => DOMPurify.sanitize(contactDisclaimerHtml),
    [contactDisclaimerHtml],
  );

  // An admin may clear the disclaimer, in which case the paragraph is omitted
  // instead of rendering empty.
  const hasContactDisclaimer = sanitizedContactDisclaimer.trim() !== "";

  return (
    <div className="flex flex-col justify-center space-y-6">
      <div className="space-y-3">
        <h1 className="text-3xl font-semibold tracking-tight text-balance lg:text-4xl">
          Bem-vinde ao RunCodes
        </h1>
        <p className="text-muted-foreground text-lg text-balance">
          Um sistema de submissão e correção automática de exercícios de
          programação, com suporte a diversas linguagens como C/C++, Python,
          Java, Haskell e Go.
        </p>
      </div>

      <p className="text-muted-foreground text-sm">
        Ao navegar no RunCodes você concorda com os{" "}
        <button
          type="button"
          onClick={() => {
            setIsTermsModalOpen(true);
          }}
          className="text-foreground font-medium underline underline-offset-4"
        >
          termos de uso
        </button>
        .
      </p>

      {hasContactDisclaimer ? (
        <p
          className="text-muted-foreground text-sm [&_a]:text-foreground [&_a]:underline"
          // Content is sanitized with DOMPurify before being inserted.
          // eslint-disable-next-line react-dom/no-dangerously-set-innerhtml
          dangerouslySetInnerHTML={{ __html: sanitizedContactDisclaimer }}
        />
      ) : null}

      <TermsModal
        isOpen={isTermsModalOpen}
        onClose={() => {
          setIsTermsModalOpen(false);
        }}
      />
    </div>
  );
}
