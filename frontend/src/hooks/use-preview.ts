import { useState } from "react";

import { useNavigate } from "react-router";

import { useAuth } from "@/hooks/use-auth";
import { adminPreview, stopPreview, type PreviewRole } from "@/lib/api";
import { errorMessage } from "@/lib/errors";

/**
 * Start and stop the admin role preview. The API keeps the session on the
 * administrator's own account and only changes the role claim, so the auth
 * state has to be reloaded after each transition before navigating.
 */
export function usePreview() {
  const navigate = useNavigate();
  const { refreshAuth } = useAuth();
  const [pending, setPending] = useState<PreviewRole | "stop" | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function start(role: PreviewRole): Promise<void> {
    setPending(role);
    setError(null);
    try {
      await adminPreview(role);
      await refreshAuth();
      void navigate(role === "professor" ? "/professor" : "/");
    } catch (caught) {
      setError(
        errorMessage(caught, "Não foi possível iniciar a pré-visualização."),
      );
    } finally {
      setPending(null);
    }
  }

  async function stop(): Promise<void> {
    setPending("stop");
    setError(null);
    try {
      await stopPreview();
      await refreshAuth();
      void navigate("/admin");
    } catch (caught) {
      setError(
        errorMessage(caught, "Não foi possível encerrar a pré-visualização."),
      );
    } finally {
      setPending(null);
    }
  }

  return { pending, error, start, stop };
}
