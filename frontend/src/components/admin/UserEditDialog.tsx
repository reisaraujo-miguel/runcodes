import { useState, type SubmitEvent } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { useSubmit } from "@/hooks/use-submit";
import {
  USER_ROLES,
  adminUpdateUser,
  type AdminUser,
  type UpdateUserPayload,
} from "@/lib/api";
import { platformRoleLabel } from "@/lib/roles";

/**
 * Edits the fields an admin may change on any account: name, email, org id and
 * role, plus its confirmation status. The role of the signed-in account is
 * disabled, since the API refuses to change it.
 */
export function UserEditDialog({
  user,
  isSelf,
  onClose,
  onSaved,
}: {
  user: AdminUser;
  isSelf: boolean;
  onClose: () => void;
  onSaved: (user: AdminUser) => void;
}) {
  const submit = useSubmit("Não foi possível salvar o usuário.");
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email);
  const [orgId, setOrgId] = useState(user.org_id);
  const [role, setRole] = useState(user.role);
  const [confirmed, setConfirmed] = useState(user.confirmed);

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const trimmedName = name.trim();
    if (trimmedName === "") {
      submit.setError("Informe o nome do usuário.");
      return;
    }
    const trimmedEmail = email.trim();
    if (trimmedEmail === "") {
      submit.setError("Informe o email do usuário.");
      return;
    }

    const payload: UpdateUserPayload = {
      name: trimmedName,
      email: trimmedEmail,
      org_id: orgId.trim(),
      confirmed,
    };
    if (!isSelf) payload.role = role;

    void submit.run(async () => {
      const updated = await adminUpdateUser(user.id, payload);
      onSaved(updated);
    });
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Editar usuário</DialogTitle>
          <DialogDescription>
            Altere os dados de {user.name} (id {user.id}).
          </DialogDescription>
        </DialogHeader>

        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="admin-user-name">Nome</Label>
            <Input
              id="admin-user-name"
              value={name}
              disabled={submit.pending}
              onChange={(event) => {
                setName(event.target.value);
              }}
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="admin-user-email">Email</Label>
            <Input
              id="admin-user-email"
              type="email"
              value={email}
              disabled={submit.pending}
              onChange={(event) => {
                setEmail(event.target.value);
              }}
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="admin-user-org">Organização</Label>
            <Input
              id="admin-user-org"
              value={orgId}
              disabled={submit.pending}
              onChange={(event) => {
                setOrgId(event.target.value);
              }}
              placeholder="Opcional"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="admin-user-role">Papel</Label>
            <Select
              id="admin-user-role"
              value={role}
              disabled={isSelf || submit.pending}
              onChange={(event) => {
                setRole(event.target.value);
              }}
            >
              {USER_ROLES.map((value) => (
                <option key={value} value={value}>
                  {platformRoleLabel(value)}
                </option>
              ))}
            </Select>
            {isSelf ? (
              <p className="text-muted-foreground text-xs">
                O papel da sua própria conta não pode ser alterado.
              </p>
            ) : null}
          </div>

          <div className="flex items-center gap-2">
            <Checkbox
              id="admin-user-confirmed"
              checked={confirmed}
              disabled={submit.pending}
              onCheckedChange={(checked) => {
                setConfirmed(checked);
              }}
            />
            <Label htmlFor="admin-user-confirmed">Conta confirmada</Label>
          </div>

          {submit.error ? (
            <Alert variant="destructive">
              <AlertDescription>{submit.error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={submit.pending}
              onClick={onClose}
            >
              Cancelar
            </Button>
            <Button type="submit" disabled={submit.pending}>
              {submit.pending ? <Spinner className="size-4" /> : null}
              Salvar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
