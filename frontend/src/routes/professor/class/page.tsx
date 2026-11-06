import { useState } from "react";

import {
  BookOpenIcon,
  CopyIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
  Trash2Icon,
} from "lucide-react";
import {
  Link,
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router";

import { BackLink } from "@/components/app/BackLink";
import { ConfirmButton } from "@/components/app/ConfirmDialog";
import { Field } from "@/components/app/Field";
import { PageHeader } from "@/components/app/PageHeader";
import { Section } from "@/components/app/Section";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { BulletinBoard } from "@/components/classes/BulletinBoard";
import { ClassMembersPanel } from "@/components/professor/ClassMembersPanel";
import { EditClassForm } from "@/components/professor/EditClassForm";
import { NewExerciseForm } from "@/components/professor/NewExerciseForm";
import { OfferingGradesPanel } from "@/components/professor/OfferingGradesPanel";
import { OfferingLanguageStatsPanel } from "@/components/professor/OfferingLanguageStatsPanel";
import { OfferingToolsPanel } from "@/components/professor/OfferingToolsPanel";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
  rotateEnrollmentCode,
} from "@/lib/api";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";

const breadcrumb = <BackLink to="/professor" />;

type ClassTab =
  | "details"
  | "exercises"
  | "bulletin"
  | "grades"
  | "languages"
  | "tools"
  | "participants";

const DEFAULT_TAB: ClassTab = "details";

/** Reads the selected tab from `?tab=`, falling back to the details tab. */
function parseTab(value: string | null): ClassTab {
  switch (value) {
    case "exercises":
    case "bulletin":
    case "grades":
    case "languages":
    case "tools":
    case "participants":
      return value;
    default:
      return DEFAULT_TAB;
  }
}

/** The enrollment code with a copy affordance, so the professor can share it. */
function EnrollmentCode({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);

  if (!code) {
    return (
      <span className="text-sm text-muted-foreground">
        Nenhum código gerado ainda.
      </span>
    );
  }

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
      <code className="rounded-md bg-muted px-2 py-1 font-mono text-base tracking-widest">
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
  const location = useLocation();
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
  const [rotateError, setRotateError] = useState<string | null>(null);

  const info = data.data;

  function selectTab(next: ClassTab) {
    setSearchParams(
      (previous) => {
        const nextParams = new URLSearchParams(previous);
        nextParams.set("tab", next);
        return nextParams;
      },
      // Keep the remembered origin across tab changes; setSearchParams would
      // otherwise replace the entry with an empty state.
      { replace: true, state: location.state as unknown },
    );
  }

  async function handleRotateCode() {
    setRotateError(null);
    try {
      await rotateEnrollmentCode(id);
      data.reload();
    } catch (error) {
      setRotateError(
        errorMessage(error, "Não foi possível gerar um novo código."),
      );
    }
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
    { value: "bulletin", label: "Mural" },
    { value: "grades", label: "Notas" },
    { value: "languages", label: "Linguagens" },
    { value: "tools", label: "Ferramentas" },
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

      <TabBar
        items={tabs}
        value={tab}
        onChange={selectTab}
        label="Seções da turma"
        idPrefix="class-overview"
      />

      <div
        role="tabpanel"
        id="class-overview-panel-details"
        aria-labelledby="class-overview-tab-details"
        hidden={tab !== "details"}
        className="space-y-6"
      >
        <Section
          title="Informações da turma"
          description="Código de matrícula, prazo e resumo desta turma."
          contentClassName="space-y-6"
          action={
            isOwner ? (
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
            ) : undefined
          }
        >
          <dl className="grid gap-x-6 gap-y-5 sm:grid-cols-2">
            <Field label="Código de matrícula">
              <div className="space-y-2">
                <EnrollmentCode code={offering.enrollment_code} />
                {isOwner ? (
                  <ConfirmButton
                    size="sm"
                    variant="outline"
                    title="Gerar novo código de matrícula"
                    description="O código atual deixa de funcionar imediatamente. Todos os alunos que ainda não se matricularam precisarão do novo código."
                    confirmLabel="Gerar novo código"
                    onConfirm={handleRotateCode}
                  >
                    <RefreshCwIcon />
                    Gerar novo código
                  </ConfirmButton>
                ) : null}
                {rotateError ? (
                  <Alert variant="destructive">
                    <AlertDescription>{rotateError}</AlertDescription>
                  </Alert>
                ) : null}
              </div>
            </Field>
            <Field label="Disponível até">
              {formatDateTime(offering.end_date)}
            </Field>
            <Field label="Matrícula">
              <Badge
                variant={offering.visible_to_enroll ? "success" : "secondary"}
              >
                {offering.visible_to_enroll ? "Aberta" : "Encerrada"}
              </Badge>
            </Field>
            <Field label="Resumo">
              {memberCount ?? "—"} participante(s) · {exerciseCount ?? "—"}{" "}
              exercício(s)
            </Field>
          </dl>

          {isOwner ? (
            <div className="rounded-xl border border-dashed border-destructive/30 bg-destructive/5 p-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="space-y-1">
                  <p className="font-medium">Excluir turma</p>
                  <p className="text-sm text-muted-foreground">
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
            <p className="text-sm text-muted-foreground">
              Apenas o professor responsável pela turma pode editá-la ou
              excluí-la.
            </p>
          )}
        </Section>
      </div>

      <div
        role="tabpanel"
        id="class-overview-panel-exercises"
        aria-labelledby="class-overview-tab-exercises"
        hidden={tab !== "exercises"}
        className="space-y-4"
      >
        <Section
          title="Exercícios"
          description="Crie e gerencie os exercícios desta turma."
          action={
            <Button
              size="sm"
              onClick={() => {
                setCreatingExercise(true);
              }}
            >
              <PlusIcon />
              Novo exercício
            </Button>
          }
          contentClassName="space-y-4"
        >
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
                        <p className="line-clamp-1 text-xs text-muted-foreground">
                          {exercise.description}
                        </p>
                      ) : null}
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap text-muted-foreground">
                      {formatDateTime(exercise.open_date)}
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap text-muted-foreground">
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
                              state={{
                                from: `${location.pathname}${location.search}`,
                              }}
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
                              state={{
                                from: `${location.pathname}${location.search}`,
                              }}
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
        </Section>
      </div>

      <div
        role="tabpanel"
        id="class-overview-panel-bulletin"
        aria-labelledby="class-overview-tab-bulletin"
        hidden={tab !== "bulletin"}
      >
        <BulletinBoard offeringId={offering.id} showTitle={false} />
      </div>

      <div
        role="tabpanel"
        id="class-overview-panel-grades"
        aria-labelledby="class-overview-tab-grades"
        hidden={tab !== "grades"}
      >
        <OfferingGradesPanel offeringId={offering.id} />
      </div>

      <div
        role="tabpanel"
        id="class-overview-panel-languages"
        aria-labelledby="class-overview-tab-languages"
        hidden={tab !== "languages"}
      >
        <OfferingLanguageStatsPanel offeringId={offering.id} />
      </div>

      <div
        role="tabpanel"
        id="class-overview-panel-tools"
        aria-labelledby="class-overview-tab-tools"
        hidden={tab !== "tools"}
      >
        <OfferingToolsPanel offeringId={offering.id} />
      </div>

      <div
        role="tabpanel"
        id="class-overview-panel-participants"
        aria-labelledby="class-overview-tab-participants"
        hidden={tab !== "participants"}
      >
        <ClassMembersPanel
          offeringId={offering.id}
          isOwner={isOwner}
          ownerId={ownerId}
          onChanged={data.reload}
        />
      </div>

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
          onCreated={(exercise) => {
            setCreatingExercise(false);
            // Creating an exercise opens its management page, so the professor
            // can add test cases and files without hunting for it in the list.
            void navigate(`/professor/exercise/${String(exercise.id)}`, {
              state: {
                from: `${location.pathname}${location.search}`,
              },
            });
          }}
        />
      ) : null}
    </div>
  );
}
