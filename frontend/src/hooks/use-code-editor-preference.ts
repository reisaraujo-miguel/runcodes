import { useCallback, useState } from "react";

import { useAsync } from "@/hooks/use-async";
import { getProfile, updateProfile } from "@/lib/api/profile";
import { errorMessage } from "@/lib/errors";

export interface CodeEditorPreference {
  /** Whether the editor should be shown; defaults to true before loading. */
  enabled: boolean;
  loading: boolean;
  saving: boolean;
  error: string | null;
  /** Persists the preference. Returns false when the update failed. */
  setEnabled: (next: boolean) => Promise<boolean>;
}

/**
 * Loads and persists the caller's in-browser editor preference. The preference
 * lives on the user profile, so it is shared by the profile and submission
 * pages; both write through the same partial profile update.
 *
 * The optimistic value is kept in local state and falls back to the profile
 * whenever the user has not overridden it in this session.
 */
export function useCodeEditorPreference(): CodeEditorPreference {
  const profile = useAsync(() => getProfile(), "profile");
  const [override, setOverride] = useState<boolean | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const enabled = override ?? profile.data?.code_editor_enabled ?? true;
  const reloadProfile = profile.reload;

  const setEnabled = useCallback(
    async (next: boolean) => {
      setOverride(next);
      setSaving(true);
      setError(null);
      try {
        await updateProfile({ code_editor_enabled: next });
        return true;
      } catch (err) {
        // Drop the optimistic value and re-read the profile so the toggle shows
        // what the server actually has, not what this session guessed.
        setOverride(null);
        reloadProfile();
        setError(
          errorMessage(err, "Não foi possível salvar a preferência do editor."),
        );
        return false;
      } finally {
        setSaving(false);
      }
    },
    [reloadProfile],
  );

  return {
    enabled,
    loading: profile.loading,
    saving,
    error,
    setEnabled,
  };
}
