import { UsersIcon } from "lucide-react";

import { EnrollmentRoleBadge } from "@/components/app/RoleBadge";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAsync } from "@/hooks/use-async";
import { adminListOfferingMembers, type AdminOffering } from "@/lib/api";
import { formatDate } from "@/lib/format";

/** Lists the members of a class, including banned ones, read-only. */
export function OfferingMembersDialog({
  offering,
  onClose,
}: {
  offering: AdminOffering;
  onClose: () => void;
}) {
  const members = useAsync(
    () => adminListOfferingMembers(offering.id),
    `admin-offering-members:${String(offering.id)}`,
    "Não foi possível carregar os membros.",
  );
  const rows = members.data ?? [];

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Membros da turma</DialogTitle>
          <DialogDescription>
            Pessoas matriculadas em {offering.name}.
          </DialogDescription>
        </DialogHeader>

        {members.loading ? (
          <LoadingState />
        ) : members.error ? (
          <ErrorState
            description={members.error}
            onRetry={members.reload}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={UsersIcon}
            title="Nenhum membro"
            description="Esta turma ainda não tem pessoas matriculadas."
            className="py-8"
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">Nome</TableHead>
                <TableHead scope="col">Email</TableHead>
                <TableHead scope="col">Papel</TableHead>
                <TableHead scope="col">Matriculado em</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((member) => (
                <TableRow key={member.user_id}>
                  <TableCell className="font-medium">{member.name}</TableCell>
                  <TableCell>{member.email}</TableCell>
                  <TableCell>
                    <EnrollmentRoleBadge
                      role={member.role}
                      banned={member.banned}
                    />
                  </TableCell>
                  <TableCell>{formatDate(member.created_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Fechar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
