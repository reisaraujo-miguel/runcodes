import { useState } from "react";

import DOMPurify from "dompurify";

import { TermsModal } from "./TermsModal";

const CONTACT_INFO_HTML = import.meta.env.VITE_CONTACT_INFO_HTML as string;

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
            <a
              onClick={openTermsModal}
              className="cursor-pointer text-foreground"
            >
              termos de uso
            </a>
            .
          </p>
        </div>

        <div className="pt-4">
          <p
            className="text-sm text-muted-foreground [&_a]:text-foreground"
            dangerouslySetInnerHTML={{
              __html: DOMPurify.sanitize(CONTACT_INFO_HTML),
            }}
          />
        </div>
      </div>

      <TermsModal isOpen={isTermsModalOpen} onClose={closeTermsModal} />
    </>
  );
}
