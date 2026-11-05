import { useRef, useState } from "react";

import {
  ChevronRightIcon,
  MessagesSquareIcon,
  PlusIcon,
  ReplyIcon,
  SendIcon,
  XIcon,
} from "lucide-react";

import { EnrollmentRoleBadge } from "@/components/app/RoleBadge";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
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
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { useAsync } from "@/hooks/use-async";
import { useSubmit } from "@/hooks/use-submit";
import {
  createBulletinComment,
  createBulletinPost,
  getOfferingBulletin,
  type BulletinComment,
  type BulletinPost,
} from "@/lib/api";
import {
  boardWindowNote,
  commentCountLabel,
  commentExcerpt,
  commentThread,
} from "@/lib/bulletin";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";
import { fieldLabelClass } from "@/lib/label";
import { cn } from "@/lib/utils";

/** What the comment box is answering: the post, or one of its comments. */
interface ReplyTarget {
  postId: number;
  commentId: number | null;
  authorName: string;
  /** The start of the answered comment, quoted above the box. */
  excerpt: string;
}

/**
 * A class's bulletin board ("Mural"): announcements from the professors and
 * monitors, and the discussion under each one.
 *
 * Any participant reads and comments; only the class staff sees the "Novo post"
 * button. The server enforces both rules, so the flag the API returns is what
 * this component trusts. Publishing a post emails every enrolled student, and a
 * comment emails the class staff — or only the student it answers.
 */
export function BulletinBoard({
  offeringId,
  showTitle = true,
}: {
  offeringId: number;
  /**
   * Whether the board renders its own heading. A tab whose label already says
   * "Mural" passes false and keeps only the intro and the composer.
   */
  showTitle?: boolean;
}) {
  const board = useAsync(
    () => getOfferingBulletin(offeringId),
    `bulletin-${String(offeringId)}`,
    "Não foi possível carregar o mural da turma.",
  );

  const [posting, setPosting] = useState(false);
  const [expanded, setExpanded] = useState<Record<number, boolean>>({});
  const [replyTo, setReplyTo] = useState<ReplyTarget | null>(null);
  const [draft, setDraft] = useState<Record<number, string>>({});
  const [busyPostId, setBusyPostId] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const data = board.data;
  const posts = data?.posts ?? [];

  async function submitComment(post: BulletinPost) {
    const body = (draft[post.id] ?? "").trim();
    if (body === "") return;

    const target = replyTo?.postId === post.id ? replyTo : null;

    setBusyPostId(post.id);
    setActionError(null);
    try {
      await createBulletinComment(offeringId, post.id, {
        body,
        parent_comment_id: target?.commentId ?? null,
      });
      setDraft((current) => ({ ...current, [post.id]: "" }));
      setReplyTo(null);
      board.reload();
    } catch (error) {
      setActionError(
        errorMessage(error, "Não foi possível publicar o comentário."),
      );
    } finally {
      setBusyPostId(null);
    }
  }

  return (
    <section
      className="space-y-4"
      aria-labelledby={showTitle ? "class-bulletin-heading" : undefined}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-1">
          {showTitle ? (
            <h2 id="class-bulletin-heading" className="text-lg font-semibold">
              Mural da turma
            </h2>
          ) : null}
          <p className="text-sm text-muted-foreground">
            Avisos dos professores e monitores, e as dúvidas da turma em cada
            post. Um novo post é enviado por email a todos os alunos
            matriculados.
          </p>
        </div>
        {data?.can_post ? (
          <Button
            size="sm"
            onClick={() => {
              setPosting(true);
            }}
          >
            <PlusIcon />
            Novo post
          </Button>
        ) : null}
      </div>

      {actionError ? (
        <Alert variant="destructive">
          <AlertDescription>{actionError}</AlertDescription>
        </Alert>
      ) : null}

      {board.loading && !data ? (
        <LoadingState label="Carregando o mural…" />
      ) : board.error ? (
        <ErrorState description={board.error} onRetry={board.reload} />
      ) : posts.length === 0 ? (
        <EmptyState
          icon={MessagesSquareIcon}
          title="Nenhum post no mural"
          description={
            data?.can_post
              ? "Publique um aviso para a turma: ele também vai por email para todos os alunos."
              : "Quando um professor ou monitor publicar um aviso, ele aparece aqui."
          }
        />
      ) : (
        <div className="divide-y divide-border border-t">
          {posts.map((post) => (
            <PostCard
              key={post.id}
              post={post}
              expanded={expanded[post.id] ?? false}
              onToggle={() => {
                setExpanded((current) => ({
                  ...current,
                  [post.id]: !(current[post.id] ?? false),
                }));
              }}
              replyTo={replyTo}
              draft={draft[post.id] ?? ""}
              busy={busyPostId === post.id}
              onDraftChange={(value) => {
                setDraft((current) => ({ ...current, [post.id]: value }));
              }}
              onReply={(comment) => {
                setReplyTo({
                  postId: post.id,
                  commentId: comment.id,
                  authorName: comment.author_name,
                  excerpt: commentExcerpt(comment.body),
                });
              }}
              onCancelReply={() => {
                setReplyTo(null);
              }}
              onSubmit={() => {
                void submitComment(post);
              }}
            />
          ))}
          {boardWindowNote(data?.post_count ?? 0, posts.length) ? (
            <p className="text-xs text-muted-foreground">
              {boardWindowNote(data?.post_count ?? 0, posts.length)}
            </p>
          ) : null}
        </div>
      )}

      {posting ? (
        <PostDialog
          offeringId={offeringId}
          onClose={() => {
            setPosting(false);
          }}
          onCreated={() => {
            setPosting(false);
            board.reload();
          }}
        />
      ) : null}
    </section>
  );
}

/**
 * One post. Collapsed it is a summary: the title, two lines of the text and the
 * comment count. Expanded it adds the full text, the thread and the comment box,
 * so a long board stays scannable.
 */
function PostCard({
  post,
  expanded,
  onToggle,
  replyTo,
  draft,
  busy,
  onDraftChange,
  onReply,
  onCancelReply,
  onSubmit,
}: {
  post: BulletinPost;
  expanded: boolean;
  onToggle: () => void;
  replyTo: ReplyTarget | null;
  draft: string;
  busy: boolean;
  onDraftChange: (value: string) => void;
  onReply: (comment: BulletinComment) => void;
  onCancelReply: () => void;
  onSubmit: () => void;
}) {
  const thread = commentThread(post.comments);
  const replyingTo = replyTo?.postId === post.id ? replyTo : null;
  const panelId = `bulletin-post-${String(post.id)}`;
  const commentFieldId = `bulletin-comment-${String(post.id)}`;
  const commentBox = useRef<HTMLTextAreaElement>(null);

  function handleReply(comment: BulletinComment) {
    onReply(comment);
    commentBox.current?.focus();
  }

  return (
    <article className="py-4 first:pt-2 last:pb-0">
      <div className="relative">
        <h3 className="text-base leading-snug font-semibold">
          <button
            type="button"
            aria-expanded={expanded}
            aria-controls={expanded ? panelId : undefined}
            onClick={onToggle}
            className="flex cursor-pointer items-start gap-2 rounded-sm text-left before:absolute before:inset-0 before:content-[''] focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
          >
            <ChevronRightIcon
              className={cn(
                "mt-0.5 size-4 shrink-0 text-muted-foreground transition-transform",
                expanded && "rotate-90",
              )}
              aria-hidden
            />
            <span>{post.title}</span>
          </button>
        </h3>

        <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 pl-6 text-xs text-muted-foreground">
          <span className="font-medium text-foreground/80">
            {post.author_name}
          </span>
          <EnrollmentRoleBadge role={post.author_role} />
          <span aria-hidden>·</span>
          <span>{formatDateTime(post.created_at)}</span>
          <span aria-hidden>·</span>
          <span>{commentCountLabel(post.comments.length)}</span>
        </p>

        {expanded ? null : (
          <p className="mt-1 line-clamp-2 pl-6 text-sm whitespace-pre-line text-muted-foreground">
            {post.body}
          </p>
        )}
      </div>

      {expanded ? (
        <div id={panelId} className="mt-3 space-y-4 pl-6">
          <p className="text-base leading-relaxed whitespace-pre-line">
            {post.body}
          </p>

          {thread.length > 0 ? (
            <div className="flex items-center gap-3 pt-1">
              <h4 className={cn(fieldLabelClass, "shrink-0")}>Comentários</h4>
              <span className="h-px flex-1 bg-border" aria-hidden />
            </div>
          ) : null}

          {thread.length > 0 ? (
            <ol className="divide-y divide-border pl-4">
              {thread.map((comment) => (
                <li key={comment.id} className="py-3">
                  <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <span className="text-sm font-medium">
                      {comment.author_name}
                    </span>
                    <EnrollmentRoleBadge role={comment.author_role} />
                    <span className="text-xs text-muted-foreground">
                      {formatDateTime(comment.created_at)}
                    </span>
                  </div>

                  {comment.replyTo ? (
                    <div className="mt-2 border-l-2 border-border pl-2.5">
                      <p className="text-xs text-muted-foreground">
                        Em resposta a{" "}
                        <span className="font-medium text-foreground/80">
                          {comment.replyTo.authorName}
                        </span>
                      </p>
                      <p className="line-clamp-2 text-xs text-muted-foreground">
                        {`“${comment.replyTo.excerpt}”`}
                      </p>
                    </div>
                  ) : null}

                  <p className="mt-1.5 text-sm whitespace-pre-line">
                    {comment.body}
                  </p>

                  <Button
                    size="sm"
                    variant="ghost"
                    className="mt-1 -ml-2 h-7 px-2 text-xs text-muted-foreground"
                    onClick={() => {
                      handleReply(comment);
                    }}
                  >
                    <ReplyIcon />
                    Responder
                  </Button>
                </li>
              ))}
            </ol>
          ) : null}

          <form
            className="space-y-2 rounded-lg bg-muted/30 p-3"
            onSubmit={(event) => {
              event.preventDefault();
              onSubmit();
            }}
          >
            {replyingTo ? (
              <div className="flex items-start justify-between gap-2 rounded-lg bg-muted/60 px-3 py-2 text-xs">
                <div className="min-w-0">
                  <p className="font-medium">
                    Respondendo a {replyingTo.authorName}
                  </p>
                  {replyingTo.excerpt ? (
                    <p className="line-clamp-2 text-muted-foreground">
                      {replyingTo.excerpt}
                    </p>
                  ) : null}
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-6 shrink-0 px-1.5"
                  onClick={onCancelReply}
                  aria-label="Cancelar resposta"
                >
                  <XIcon />
                </Button>
              </div>
            ) : null}

            <Label htmlFor={commentFieldId} className="sr-only">
              Comentar no post {post.title}
            </Label>
            <Textarea
              id={commentFieldId}
              ref={commentBox}
              value={draft}
              rows={3}
              maxLength={4000}
              placeholder={
                replyingTo
                  ? `Responder a ${replyingTo.authorName}…`
                  : "Escreva um comentário para a turma…"
              }
              onChange={(event) => {
                onDraftChange(event.target.value);
              }}
            />
            <div className="flex justify-end">
              <Button
                type="submit"
                size="sm"
                disabled={busy || draft.trim() === ""}
              >
                {busy ? <Spinner /> : <SendIcon />}
                {replyingTo ? "Responder" : "Comentar"}
              </Button>
            </div>
          </form>
        </div>
      ) : null}
    </article>
  );
}

/** The staff-only composer for a new announcement. */
function PostDialog({
  offeringId,
  onClose,
  onCreated,
}: {
  offeringId: number;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const { pending, error, setError, run } = useSubmit(
    "Não foi possível publicar o post.",
  );

  function handleSubmit() {
    if (!title.trim()) {
      setError("Informe o título do post.");
      return;
    }
    if (!body.trim()) {
      setError("Escreva o texto do post.");
      return;
    }

    void run(async () => {
      await createBulletinPost(offeringId, {
        title: title.trim(),
        body: body.trim(),
      });
      onCreated();
    });
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Novo post no mural</DialogTitle>
          <DialogDescription>
            O post aparece na página da turma e é enviado por email a todos os
            alunos matriculados, com o texto completo.
          </DialogDescription>
        </DialogHeader>

        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            handleSubmit();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor="bulletin-post-title">Título</Label>
            <Input
              id="bulletin-post-title"
              value={title}
              maxLength={160}
              onChange={(event) => {
                setTitle(event.target.value);
              }}
              placeholder="Ex.: Alteração no prazo do exercício 3"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="bulletin-post-body">Mensagem</Label>
            <Textarea
              id="bulletin-post-body"
              value={body}
              rows={8}
              maxLength={8000}
              onChange={(event) => {
                setBody(event.target.value);
              }}
              placeholder="Escreva o aviso para a turma…"
              required
            />
            <p className="text-xs text-muted-foreground">
              A mensagem é enviada como texto simples com uma versão em HTML. Os
              alunos podem comentar o post para tirar dúvidas.
            </p>
          </div>

          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? <Spinner /> : <SendIcon />}
              Publicar e enviar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
