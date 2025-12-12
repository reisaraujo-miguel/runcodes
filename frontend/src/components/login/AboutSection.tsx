import { useState } from "react";

import { TermsModal } from "./TermsModal";

export function AboutSection() {
  const [isTermsModalOpen, setTermsModalOpen] = useState(false);

  const openTermsModal = () => {
    setTermsModalOpen(true);
  };
  const closeTermsModal = () => {
    setTermsModalOpen(false);
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
            <a onClick={openTermsModal} className="cursor-pointer">
              termos de uso
            </a>
            .
          </p>
        </div>

        <div className="pt-4">
          <p className="text-sm text-muted-foreground">
            Note que o RunCodes ICMC não é um serviço mantido pelo STI, logo, em
            caso de problemas com a plataforma, entre em contato com{" "}
            <a href="mailto:runcodes@icmc.usp.br">runcodes@icmc.usp.br</a>
          </p>
        </div>
      </div>

      <TermsModal isOpen={isTermsModalOpen} onClose={closeTermsModal} />
    </>
  );
}
