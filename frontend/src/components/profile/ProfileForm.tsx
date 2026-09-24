import { useEffect, useState, type SubmitEvent } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/hooks/use-auth";
import type { Profile, ProfilePayload } from "@/lib/api/profile";
import { getProfile, updateProfile } from "@/lib/api/profile";
import { formatDateTime } from "@/lib/format";

const ROLE_LABELS: Record<string, string> = {
  student: "Aluno",
  professor: "Professor",
  admin: "Administrador",
  dev: "Desenvolvedor",
};

/** Translates an API role into its Brazilian Portuguese label. */
function translateRole(role: string): string {
  return ROLE_LABELS[role] ?? role;
}

/**
 * Card that loads the caller's own account, edits the mutable fields (name,
 * email, organization id) and shows the read-only ones (role, creation date,
 * confirmation status).
 */
export function ProfileForm() {
  const { reloadSession } = useAuth();

  const [profile, setProfile] = useState<Profile | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [orgId, setOrgId] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadProfile() {
      try {
        const data = await getProfile();
        if (cancelled) return;
        setProfile(data);
        setName(data.name);
        setEmail(data.email);
        setOrgId(data.org_id);
      } catch (loadFailure) {
        if (!cancelled) {
          setLoadError(
            loadFailure instanceof Error
              ? loadFailure.message
              : "Não foi possível carregar os seus dados.",
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadProfile();

    return () => {
      cancelled = true;
    };
  }, []);

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!profile) return;

    const nextName = name.trim();
    const nextEmail = email.trim();
    const nextOrgId = orgId.trim();

    if (!nextName) {
      setError("O nome é obrigatório");
      return;
    }
    if (!nextEmail) {
      setError("O email é obrigatório");
      return;
    }

    // The API does a partial update, so unchanged fields are left out entirely.
    const payload: ProfilePayload = {};
    if (nextName !== profile.name) payload.name = nextName;
    if (nextEmail !== profile.email) payload.email = nextEmail;
    if (nextOrgId !== profile.org_id) payload.org_id = nextOrgId;

    if (Object.keys(payload).length === 0) {
      setError(null);
      setSuccess("Nenhuma alteração para salvar.");
      return;
    }

    setSubmitting(true);
    setError(null);
    setSuccess(null);

    void (async () => {
      try {
        const updated = await updateProfile(payload);
        setProfile(updated);
        setName(updated.name);
        setEmail(updated.email);
        setOrgId(updated.org_id);
        setSuccess("Dados atualizados com sucesso.");
        // The navbar shows the name and email, and the session token still
        // carries the old ones, so the session is renewed here.
        await reloadSession();
      } catch (submitError) {
        setError(
          submitError instanceof Error
            ? submitError.message
            : "Erro ao atualizar os dados",
        );
      } finally {
        setSubmitting(false);
      }
    })();
  }

  if (loading) {
    return (
      <div className="flex justify-center p-12">
        <div
          aria-label="Carregando"
          className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
          role="status"
        />
      </div>
    );
  }

  if (!profile) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Não foi possível carregar o perfil</CardTitle>
          <CardDescription>
            {loadError ?? "Tente novamente mais tarde."}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Dados da conta</CardTitle>
        <CardDescription>
          Atualize o seu nome, email e organização quando necessário.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="profile-name">Nome</Label>
            <Input
              id="profile-name"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
              }}
              autoComplete="name"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="profile-email">Email</Label>
            <Input
              id="profile-email"
              type="email"
              value={email}
              onChange={(event) => {
                setEmail(event.target.value);
              }}
              autoComplete="email"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="profile-org-id">ID da organização</Label>
            <Input
              id="profile-org-id"
              value={orgId}
              onChange={(event) => {
                setOrgId(event.target.value);
              }}
              placeholder="Ex.: 12345678"
            />
          </div>

          {error && <p className="text-destructive text-sm">{error}</p>}
          {success && (
            <p className="text-emerald-600 dark:text-emerald-400 text-sm">
              {success}
            </p>
          )}

          <Button type="submit" disabled={submitting}>
            {submitting ? "Salvando…" : "Salvar alterações"}
          </Button>
        </form>

        <div className="grid gap-4 border-t pt-4 sm:grid-cols-3">
          <div>
            <p className="text-muted-foreground text-sm">Perfil</p>
            <p>{translateRole(profile.role)}</p>
          </div>
          <div>
            <p className="text-muted-foreground text-sm">Conta criada em</p>
            <p>{formatDateTime(profile.created_at)}</p>
          </div>
          <div>
            <p className="text-muted-foreground text-sm">Confirmação</p>
            <Badge variant={profile.confirmed ? "success" : "warning"}>
              {profile.confirmed ? "Confirmada" : "Pendente"}
            </Badge>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
