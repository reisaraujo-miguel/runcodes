import { useCallback, useState } from "react";

import { errorMessage } from "@/lib/errors";

/**
 * Standard submit state for a form: runs an async action while tracking a
 * pending flag and surfacing either the returned error or a success message.
 */
export function useSubmit(fallbackError = "Não foi possível salvar.") {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const run = useCallback(
    async (action: () => Promise<void>, successMessage?: string) => {
      setPending(true);
      setError(null);
      setSuccess(null);
      try {
        await action();
        if (successMessage) setSuccess(successMessage);
        return true;
      } catch (err) {
        setError(errorMessage(err, fallbackError));
        return false;
      } finally {
        setPending(false);
      }
    },
    [fallbackError],
  );

  return { pending, error, success, setError, setSuccess, run };
}
