import { usePlatformSettings } from "@/hooks/use-platform-settings";

import { Button } from "../ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../ui/dialog";

interface TermsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

/**
 * The platform's terms of use.
 *
 * It is a real Dialog rather than a styled div so that assistive technology is
 * told a dialog opened, focus is trapped inside it while it is open, Escape
 * closes it and focus returns to the trigger afterwards — all of which the
 * dialog primitive provides and a plain overlay does not.
 */
export function TermsModal({ isOpen, onClose }: TermsModalProps) {
  // The address is admin-editable, so it comes from the API; the hook falls
  // back to the build-time value while the request is in flight.
  const { contact_email: contactEmail } = usePlatformSettings();

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="sm:max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-2xl">Termos de Uso</DialogTitle>
          <DialogDescription>
            Leia os termos que regem o uso da plataforma RunCodes.
          </DialogDescription>
        </DialogHeader>

        <div className="prose prose-sm max-w-none">
          <h3 className="font-bold">1. Aceitação dos Termos</h3>
          <p>
            Ao acessar e utilizar o RunCodes, você concorda com estes Termos de
            Uso.
          </p>

          <h3 className="font-bold pt-4">2. Registros de Visitação</h3>
          <p>
            Para fornecer serviços melhores a todos os nossos usuários, quando
            há um acesso ao site RunCodes, são registradas as seguintes
            informações: data e hora dos acessos; páginas visitadas; tipo de
            navegador; endereço IP; ação que o usuário tentou executar (download
            de um documento, por exemplo) e se obteve êxito; detalhes de como
            você usou nosso serviço; e endereço de outro site origem, caso o
            acesso ao site RunCodes ocorra por meio de link. Esses registros são
            usuais na visitação de diversos sites da Internet.
          </p>

          <h3 className="font-bold pt-4">
            3. Do teor dos dados e programas submetidos:
          </h3>
          <p>
            Ao concordar com esta política, você como usuário se compromete a
            não submeter quaisquer programas ou dados maliciosos que atentem
            contra a integridade do sistema RunCodes incluindo (porem está lista
            não é extensiva) phishing, malwares, worms, trojans, etc.
          </p>
          <p>
            Além disso, você como usuário afirma ser autor ou deter autorização
            para envio de qualquer dado ou programa submetido ao sistema
            RunCodes.
          </p>

          <h3 className="font-bold pt-4">4. Sobre correção automática:</h3>
          <p>
            Ao se tornar um usuário do RunCodes você autoriza e concorda com a
            correção automática de qualquer programa ou dado submetido ao nosso
            sistema.
          </p>

          <h3 className="font-bold pt-4">5. Sobre verificação de plágio:</h3>
          <p>
            Ao se tornar um usuário do RunCodes você autoriza e concorda com a
            verificação de plágio, isto é, comparação de seus programas ou
            dados, em relação a demais submissões realizadas por terceiros, bem
            como documentos disponíveis na Internet. Além disso, você autoriza o
            uso de seus programas e dados para verificação de plágio de
            submissões de terceiros.
          </p>

          <h3 className="font-bold pt-4">6. Política de cookies:</h3>
          <p>
            O site RunCodes utiliza cookies. Esses cookies são utilizados para
            distinguir os usuários do site. A utilização desses cookies permite
            a coleta de estatísticas de acesso, ajudando-nos a oferecer uma boa
            experiência aos usuários do site e nos permitindo realizar melhorias
            constantes. São utilizadas, ainda, ferramentas de coleta de
            informações com base em cookies tais como o PiWik e o Google
            Analytics. Assim, essa política de privacidade inclui as políticas
            de privacidade do PiWik e Google Analytics.
          </p>

          <h3 className="font-bold pt-4">7. Segurança das informações:</h3>
          <p>
            Utilizamos criptografia SSL para proteger o RunCodes e nossos
            usuários com o objetivo de evitar acessos, alterações e destruição
            de informações que detemos.
          </p>

          <h3 className="font-bold pt-4">8. Alterações:</h3>
          <p>
            Nossa Política de Privacidade pode ser alterada de tempos em tempos.
            Nós não reduziremos seus direitos nesta Política de Privacidade sem
            seu consentimento explícito. Publicaremos quaisquer alterações da
            política de privacidade nesta página e, se as alterações forem
            significativas, forneceremos um aviso com mais destaque (incluindo,
            para alguns serviços, notificação por e-mail das alterações da
            política de privacidade). Também manteremos as versões anteriores
            desta Política de Privacidade arquivadas para você visualizá-las.
          </p>

          <h3 className="font-bold pt-4">9. Questões legais:</h3>
          <p>
            Para dirimir eventuais conflitos relativos a esta política, as
            partes elegem o fórum da comarcar de São Carlos, São Paulo, Brasil e
            excluem qualquer outro por mais privilegiado que seja.
          </p>

          <h3 className="font-bold pt-4">10. Conduta do Usuário</h3>
          <p>Você concorda em não:</p>
          <ul className="list-disc list-inside">
            <li>Tentar burlar o sistema de correção automática</li>
            <li>Compartilhar soluções que violem a política acadêmica</li>
            <li>Utilizar o serviço para atividades ilegais ou maliciosas</li>
            <li>Interferir no funcionamento normal do sistema</li>
          </ul>

          <h3 className="font-bold pt-4">11. Propriedade Intelectual</h3>
          <p>
            O código fonte submetido permanece de propriedade do usuário, mas o
            RunCodes tem o direito de armazenar e processar as submissões para
            fins de correção e análise.
          </p>

          <h3 className="font-bold pt-4">12. Limitação de Responsabilidade</h3>
          <p>
            O RunCodes não se responsabiliza por perdas ou danos resultantes do
            uso do serviço. O serviço é fornecido "como está" sem garantias de
            qualquer tipo.
          </p>

          <h3 className="font-bold pt-4">13. Contato</h3>
          <p>
            Em caso de dúvidas sobre estes termos, entre em contato através do
            email{" "}
            <a href={`mailto:${contactEmail}`} className="underline">
              {contactEmail}
            </a>
          </p>
        </div>

        <DialogFooter>
          <Button onClick={onClose}>Fechar</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
