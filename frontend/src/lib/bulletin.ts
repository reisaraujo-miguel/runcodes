import type { BulletinComment } from "@/lib/api";

/** How much of the answered comment a reply quote shows. */
const REPLY_EXCERPT_LENGTH = 90;

/**
 * Collapses whitespace and trims a comment to a short quote.
 *
 * A quote has to stay one or two lines whether the original was a sentence or a
 * paragraph, so the excerpt is measured in characters, not words.
 */
export function commentExcerpt(
  body: string,
  max: number = REPLY_EXCERPT_LENGTH,
): string {
  const collapsed = body.replace(/\s+/g, " ").trim();
  if (collapsed.length <= max) return collapsed;
  return `${collapsed.slice(0, max).trimEnd()}…`;
}

/** The comment a reply answers, with enough of its text to recognise it. */
export interface BulletinReplyTarget {
  id: number;
  authorName: string;
  excerpt: string;
}

/**
 * A comment ready to render: the comment itself plus the comment it answers.
 *
 * The board quotes the answered comment under a reply, which needs its author
 * and text. Resolving that here keeps the component free of lookups per row and
 * puts the one edge case — a parent that is not in the payload — in a function a
 * test can pin.
 */
export interface BulletinThreadComment extends BulletinComment {
  /** The comment this one answers, or null for a top-level one. */
  replyTo: BulletinReplyTarget | null;
}

/**
 * Flattens a post's comments into display order, resolving each reply's target.
 *
 * Comments stay in the order the API returned them (oldest first, which is how
 * a discussion reads). A reply whose parent is missing is rendered as a
 * top-level comment rather than dropped: the text is still part of the thread.
 */
export function commentThread(
  comments: readonly BulletinComment[],
): BulletinThreadComment[] {
  const byId = new Map(comments.map((comment) => [comment.id, comment]));

  return comments.map((comment) => {
    const parent =
      comment.parent_comment_id === null
        ? null
        : (byId.get(comment.parent_comment_id) ?? null);

    return {
      ...comment,
      replyTo: parent
        ? {
            id: parent.id,
            authorName: parent.author_name,
            excerpt: commentExcerpt(parent.body),
          }
        : null,
    };
  });
}

/** The comment count as a caption, including the zero case. */
export function commentCountLabel(count: number): string {
  if (count === 0) return "Sem comentários";
  return count === 1 ? "1 comentário" : `${String(count)} comentários`;
}

/**
 * Which slice of the board is on screen, for the note under the list. Empty when
 * the payload carries every post.
 */
export function boardWindowNote(postCount: number, shown: number): string {
  if (shown >= postCount) return "";

  return `Mostrando os ${String(shown)} posts mais recentes de ${String(postCount)}.`;
}
