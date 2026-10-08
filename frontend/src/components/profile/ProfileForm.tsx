import { useState, type SubmitEvent } from "react";

import { PlatformRoleBadge } from "@/components/app/RoleBadge";
import { SectionCard } from "@/components/app/SectionCard";
import { ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
import { useAuth } from "@/hooks/use-auth";
import { useSubmit } from "@/hooks/use-submit";
import {
  getProfile,
  updateProfile,
  type Profile,
  type ProfilePayload,
} from "@/lib/api/profile";
import { formatDateTime } from "@/lib/format";

/** The editable fields, seeded once from the loaded profile. */
function ProfileFields({
  profile,
  onSaved,
}: {
  profile: Profile;
  onSaved: () => void;
}) {
  const { reloadSession } = useAuth();
  const { pending, error, success, setError, run } = useSubmit(
    "Não foi possível atualizar os dados.",
  );

  const [name, setName] = useState(profile.name);
  const [email, setEmail] = useState(profile.email);
  const [orgId, setOrgId] = useState(profile.org_id);

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextName = name.trim();
    const nextEmail = email.trim();
    const nextOrgId = orgId.trim();

    if (!nextName || !nextEmail) {
      setError("O nome e o email são obrigatórios.");
      return;
    }

    // The API does a partial update, so unchanged fields are left out entirely.
    const payload: ProfilePayload = {};
    if (nextName !== profile.name) payload.name = nextName;
    if (nextEmail !== profile.email) payload.email = nextEmail;
    if (nextOrgId !== profile.org_id) payload.org_id = nextOrgId;

    if (Object.keys(payload).length === 0) {
      void run(() => Promise.resolve(), "Nenhuma alteração para salvar.");
      return;
    }

    void run(async () => {
      await updateProfile(payload);
      // The sidebar shows the name and email, and the token still carries the
      // old ones, so the session is renewed after a change.
      await reloadSession();
      onSaved();
    }, "Dados atualizados com sucesso.");
  }

  return (
    <div className="space-y-6">
      <form
        className="space-y-4"
        onSubmit={(event) => {
          handleSubmit(event);
        }}
      >
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

        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
        {success ? (
          <Alert variant="success">
            <AlertDescription>{success}</AlertDescription>
          </Alert>
        ) : null}

        <Button type="submit" disabled={pending}>
          {pending ? <Spinner className="size-4" /> : null}
          Salvar alterações
        </Button>
      </form>

      <dl className="grid gap-4 border-t pt-4 sm:grid-cols-3">
        <div>
          <dt className="text-muted-foreground text-sm">Perfil</dt>
          <dd className="mt-1">
            <PlatformRoleBadge role={profile.role} />
          </dd>
        </div>
        <div>
          <dt className="text-muted-foreground text-sm">Conta criada em</dt>
          <dd className="mt-1 text-sm">{formatDateTime(profile.created_at)}</dd>
        </div>
        <div>
          <dt className="text-muted-foreground text-sm">Confirmação</dt>
          <dd className="mt-1">
            <Badge variant={profile.confirmed ? "success" : "warning"}>
              {profile.confirmed ? "Confirmada" : "Pendente"}
            </Badge>
          </dd>
        </div>
      </dl>
    </div>
  );
}

/**
 * Loads the caller's own account, edits the mutable fields (name, email,
 * organization id) and shows the read-only ones.
 */
export function ProfileForm() {
  const profile = useAsync(() => getProfile(), "profile");

  if (!profile.data) {
    return profile.loading ? (
      <LoadingState />
    ) : (
      <ErrorState
        description={profile.error ?? "Não foi possível carregar o perfil."}
        onRetry={profile.reload}
      />
    );
  }

  return (
    <SectionCard
      title="Dados da conta"
      description="Atualize o seu nome, email e organização."
    >
      <ProfileFields
        key={profile.data.id}
        profile={profile.data}
        onSaved={profile.reload}
      />
    </SectionCard>
  );
}
