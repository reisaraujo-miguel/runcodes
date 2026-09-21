import { useState } from "react";

import DOMPurify from "dompurify";

import { TermsModal } from "./TermsModal";

const CONTACT_DISCLAIMER_HTML =
  import.meta.env.VITE_CONTACT_DISCLAIMER_HTML ?? "";

// Sanitized once at module scope rather than on every render: the value is a
// build-time constant.
const SANITIZED_CONTACT_DISCLAIMER = DOMPurify.sanitize(
  CONTACT_DISCLAIMER_HTML,
);

export function AboutSection() {
  const [isTermsModalOpen, setIsTermsModalOpen] = useState(false);

  const openTermsModal = () => {
    setIsTermsModalOpen(true);
  };
  const closeTermsModal = () => {
    setIsTermsModalOpen(false);
  };

  return (
    <>
      <div className="flex flex-col justify-center space-y-6">
        <div className="space-y-2">
          <h1 className="text-4xl font-bold tracking-tight">
            Bem-vindo ao RunCodes ICMC
          </h1>
          <p className="text-xl text-muted-foreground">
            O RunCodes é um sistema de submissão e correção automática de
            exercícios de programação, com suporte a diversas linguagens como
            C/C++, Python, Java, Haskell, GoLang, dentre outras.
          </p>
        </div>

        <div className="pt-4">
          <p className="text-sm text-muted-foreground">
            Ao navegar no RunCodes você concorda com os{" "}
            <button
              type="button"
              onClick={openTermsModal}
              className="cursor-pointer text-foreground underline underline-offset-4"
            >
              termos de uso
            </button>
            .
          </p>
        </div>

        <div className="pt-4">
          <p
            className="text-sm text-muted-foreground [&_a]:text-foreground"
            // Content is sanitized with DOMPurify before being inserted.
            // eslint-disable-next-line react-dom/no-dangerously-set-innerhtml
            dangerouslySetInnerHTML={{
              __html: SANITIZED_CONTACT_DISCLAIMER,
            }}
          />
        </div>
      </div>

      <TermsModal isOpen={isTermsModalOpen} onClose={closeTermsModal} />
    </>
  );
}
