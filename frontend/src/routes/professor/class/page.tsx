import { useState } from "react";

import {
  BookOpenIcon,
  CopyIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "lucide-react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { PageHeader } from "@/components/app/PageHeader";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { ClassMembersCard } from "@/components/professor/ClassMembersCard";
import { EditClassForm } from "@/components/professor/EditClassForm";
import { NewExerciseForm } from "@/components/professor/NewExerciseForm";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { TabBar, type TabBarItem } from "@/components/ui/tab-bar";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAsync } from "@/hooks/use-async";
import {
  deleteOffering,
  getOffering,
  getOfferingExercises,
  listOfferings,
} from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";

const breadcrumb = (
  <Link
    to="/professor"
    className="text-muted-foreground text-sm hover:underline"
  >
    ← Gerenciar turmas
  </Link>
);

type ClassTab = "details" | "exercises" | "participants";

const DEFAULT_TAB: ClassTab = "details";

/** Reads the selected tab from `?tab=`, falling back to the details tab. */
function parseTab(value: string | null): ClassTab {
  return value === "exercises" || value === "participants"
    ? value
    : DEFAULT_TAB;
}

/** The enrollment code with a copy affordance, so the professor can share it. */
function EnrollmentCode({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => {
        setCopied(false);
      }, 2000);
    } catch {
      // The clipboard is unavailable (insecure context); the code stays visible.
    }
  }

  return (
    <span className="flex flex-wrap items-center gap-2">
      <code className="bg-muted rounded-md px-2 py-1 font-mono text-base tracking-widest">
        {code}
      </code>
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() => {
          void handleCopy();
        }}
      >
        <CopyIcon />
        {copied ? "Copiado" : "Copiar"}
      </Button>
    </span>
  );
}

/**
 * Page for a single class: its details, exercises and members, split into tabs
 * so the management sections are not stacked on top of each other. A
 * co-professor reaches the page too, but every owner-only action (editing,
 * deleting, member management) is hidden for them; the API enforces the same
 * rule.
 */
export function ClassPage() {
  const { offeringId } = useParams();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const id = Number(offeringId);
  const valid = Number.isInteger(id) && id > 0;
  const key = offeringId ?? "";
  const tab = parseTab(searchParams.get("tab"));

  const data = useAsync(
    async () => {
      if (!valid) throw new Error("Turma inválida.");
      const [offering, managed] = await Promise.all([
        getOffering(id),
        listOfferings(),
      ]);
      const mine = managed.find((item) => item.id === id) ?? null;
      return {
        offering,
        isOwner: mine?.is_owner ?? false,
        ownerId: mine?.owner_id ?? null,
        memberCount: mine?.member_count ?? null,
        exerciseCount: mine?.exercise_count ?? null,
      };
    },
    `offering-${key}`,
    "Não foi possível carregar esta turma.",
  );

  const exercises = useAsync(
    () => (valid ? getOfferingExercises(id) : Promise.resolve([])),
    `offering-exercises-${key}`,
    "Não foi possível carregar os exercícios.",
  );

  const [editing, setEditing] = useState(false);
  const [creatingExercise, setCreatingExercise] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const info = data.data;

  function selectTab(next: ClassTab) {
    setSearchParams(
      (previous) => {
        const nextParams = new URLSearchParams(previous);
        nextParams.set("tab", next);
        return nextParams;
      },
      { replace: true },
    );
  }

  async function handleDeleteClass() {
    setDeleteError(null);
    try {
      await deleteOffering(id);
      void navigate("/professor");
    } catch (error) {
      setDeleteError(errorMessage(error, "Erro ao excluir a turma."));
    }
  }

  if (data.loading && info === null) return <LoadingState />;

  if (!info) {
    return (
      <div className="space-y-6">
        <PageHeader title="Turma" breadcrumb={breadcrumb} />
        <ErrorState
          description={data.error ?? "Não foi possível carregar esta turma."}
          onRetry={data.reload}
        />
      </div>
    );
  }

  const { offering, isOwner, ownerId, memberCount, exerciseCount } = info;
  const exerciseList = exercises.data ?? [];

  const tabs: TabBarItem<ClassTab>[] = [
    { value: "details", label: "Detalhes" },
    {
      value: "exercises",
      label: "Exercícios",
      badge: exerciseCount ?? exerciseList.length,
    },
    {
      value: "participants",
      label: "Participantes",
      badge: memberCount ?? undefined,
    },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        title={offering.name}
        description={offering.description || undefined}
        breadcrumb={breadcrumb}
        actions={
          isOwner ? (
            <Badge variant="default">Responsável</Badge>
          ) : (
            <Badge variant="info">Professor convidado</Badge>
          )
        }
      />

      <Card className="gap-0 py-0">
        <TabBar
          items={tabs}
          value={tab}
          onChange={selectTab}
          label="Seções da turma"
          idPrefix="class-overview"
          className="px-2"
        />

        <div className="p-4 sm:p-5">
          <div
            role="tabpanel"
            id="class-overview-panel-details"
            aria-labelledby="class-overview-tab-details"
            hidden={tab !== "details"}
            className="space-y-6"
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 className="text-base font-medium">Informações da turma</h2>
                <p className="text-muted-foreground text-sm">
                  Código de matrícula, prazo e resumo desta turma.
                </p>
              </div>
              {isOwner ? (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setEditing(true);
                  }}
                >
                  <PencilIcon />
                  Editar turma
                </Button>
              ) : null}
            </div>

            <dl className="grid gap-4 sm:grid-cols-2">
              <div>
                <dt className="text-muted-foreground text-sm">
                  Código de matrícula
                </dt>
                <dd className="mt-1">
                  <EnrollmentCode code={offering.enrollment_code} />
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground text-sm">
                  Disponível até
                </dt>
                <dd className="mt-1">{formatDateTime(offering.end_date)}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground text-sm">Matrícula</dt>
                <dd className="mt-1">
                  <Badge
                    variant={
                      offering.visible_to_enroll ? "success" : "secondary"
                    }
                  >
                    {offering.visible_to_enroll ? "Aberta" : "Encerrada"}
                  </Badge>
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground text-sm">Resumo</dt>
                <dd className="mt-1 text-sm">
                  {memberCount ?? "—"} participante(s) · {exerciseCount ?? "—"}{" "}
                  exercício(s)
                </dd>
              </div>
            </dl>

            {isOwner ? (
              <div className="border-destructive/30 bg-destructive/5 rounded-xl border border-dashed p-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="space-y-1">
                    <p className="font-medium">Excluir turma</p>
                    <p className="text-muted-foreground text-sm">
                      Excluir a turma apaga também os exercícios, os casos de
                      teste e as submissões dos alunos. Esta ação não pode ser
                      desfeita.
                    </p>
                  </div>
                  <ConfirmButton
                    destructive
                    variant="destructive"
                    title="Excluir turma"
                    description={`Excluir “${offering.name}” e todo o seu conteúdo?`}
                    confirmLabel="Excluir turma"
                    onConfirm={handleDeleteClass}
                  >
                    <Trash2Icon />
                    Excluir turma
                  </ConfirmButton>
                </div>
                {deleteError ? (
                  <Alert variant="destructive" className="mt-3">
                    <AlertDescription>{deleteError}</AlertDescription>
                  </Alert>
                ) : null}
              </div>
            ) : (
              <p className="text-muted-foreground text-sm">
                Apenas o professor responsável pela turma pode editá-la ou
                excluí-la.
              </p>
            )}
          </div>

          <div
            role="tabpanel"
            id="class-overview-panel-exercises"
            aria-labelledby="class-overview-tab-exercises"
            hidden={tab !== "exercises"}
            className="space-y-4"
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h2 className="text-base font-medium">Exercícios</h2>
                <p className="text-muted-foreground text-sm">
                  Crie e gerencie os exercícios desta turma.
                </p>
              </div>
              <Button
                size="sm"
                onClick={() => {
                  setCreatingExercise(true);
                }}
              >
                <PlusIcon />
                Novo exercício
              </Button>
            </div>

            {exercises.loading && exercises.data === null ? (
              <LoadingState className="py-8" />
            ) : exercises.error ? (
              <ErrorState
                description={exercises.error}
                onRetry={exercises.reload}
              />
            ) : exerciseList.length === 0 ? (
              <EmptyState
                icon={BookOpenIcon}
                title="Nenhum exercício ainda"
                description="Crie o primeiro exercício e defina o período de entrega."
                action={
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setCreatingExercise(true);
                    }}
                  >
                    <PlusIcon />
                    Novo exercício
                  </Button>
                }
              />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Exercício</TableHead>
                    <TableHead>Abertura</TableHead>
                    <TableHead>Prazo</TableHead>
                    <TableHead>Situação</TableHead>
                    <TableHead className="text-right">Ações</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {exerciseList.map((exercise) => (
                    <TableRow key={exercise.id}>
                      <TableCell>
                        <p
                          className={
                            exercise.removed
                              ? "font-medium line-through"
                              : "font-medium"
                          }
                        >
                          {exercise.title}
                        </p>
                        {exercise.description ? (
                          <p className="text-muted-foreground line-clamp-1 text-xs">
                            {exercise.description}
                          </p>
                        ) : null}
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap text-xs">
                        {formatDateTime(exercise.open_date)}
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap text-xs">
                        {formatDateTime(exercise.deadline)}
                      </TableCell>
                      <TableCell>
                        {exercise.removed ? (
                          <Badge variant="destructive">Removido</Badge>
                        ) : exercise.show_before_open_date ? (
                          <Badge variant="outline">
                            Visível antes da abertura
                          </Badge>
                        ) : (
                          <Badge variant="secondary">Publicado</Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap items-center justify-end gap-2">
                          <Button
                            size="sm"
                            variant="outline"
                            render={
                              <Link
                                to={`/exercises/${String(exercise.id)}/submit`}
                              />
                            }
                          >
                            Enviar solução
                          </Button>
                          <Button
                            size="sm"
                            variant="secondary"
                            render={
                              <Link
                                to={`/professor/exercise/${String(exercise.id)}`}
                              />
                            }
                          >
                            Gerenciar
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>

          <div
            role="tabpanel"
            id="class-overview-panel-participants"
            aria-labelledby="class-overview-tab-participants"
            hidden={tab !== "participants"}
          >
            <ClassMembersCard
              offeringId={offering.id}
              isOwner={isOwner}
              ownerId={ownerId}
              embedded
              onChanged={data.reload}
            />
          </div>
        </div>
      </Card>

      {editing ? (
        <EditClassForm
          offering={offering}
          onClose={() => {
            setEditing(false);
          }}
          onSaved={() => {
            setEditing(false);
            data.reload();
          }}
        />
      ) : null}

      {creatingExercise ? (
        <NewExerciseForm
          offeringId={offering.id}
          onClose={() => {
            setCreatingExercise(false);
          }}
          onCreated={() => {
            setCreatingExercise(false);
            exercises.reload();
            data.reload();
          }}
        />
      ) : null}
    </div>
  );
}
