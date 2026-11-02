import { describe, expect, test } from "bun:test";

import type { BulletinComment } from "./api";
import {
  boardWindowNote,
  commentCountLabel,
  commentExcerpt,
  commentThread,
} from "./bulletin";

function comment(
  id: number,
  parentCommentId: number | null,
  authorName: string,
  body = `comment ${String(id)}`,
): BulletinComment {
  return {
    id,
    parent_comment_id: parentCommentId,
    author_id: id,
    author_name: authorName,
    author_role: "student",
    body,
    created_at: "2026-11-02T12:00:00Z",
  };
}

describe("commentThread", () => {
  test("keeps the API order and resolves each reply's target", () => {
    const thread = commentThread([
      comment(1, null, "Ada", "A monitoria cobre k negativo?"),
      comment(2, 1, "Grace"),
      comment(3, 2, "Ada"),
    ]);

    expect(thread.map((entry) => entry.id)).toEqual([1, 2, 3]);
    expect(thread[0]?.replyTo).toBeNull();
    expect(thread[1]?.replyTo).toEqual({
      id: 1,
      authorName: "Ada",
      excerpt: "A monitoria cobre k negativo?",
    });
    expect(thread[2]?.replyTo?.authorName).toBe("Grace");
  });

  test("renders a reply whose parent is missing as a top-level comment", () => {
    const thread = commentThread([comment(5, 404, "Alan")]);

    expect(thread[0]?.replyTo).toBeNull();
    expect(thread[0]?.body).toBe("comment 5");
  });

  test("quotes only the start of a long parent comment", () => {
    const long = "palavra ".repeat(40);
    const thread = commentThread([
      comment(1, null, "Ada", long),
      comment(2, 1, "Grace"),
    ]);

    expect(thread[1]?.replyTo?.excerpt.length).toBe(91);
    expect(thread[1]?.replyTo?.excerpt.endsWith("…")).toBe(true);
  });

  test("handles an empty thread", () => {
    expect(commentThread([])).toEqual([]);
  });
});

describe("commentExcerpt", () => {
  test("collapses newlines and repeated spaces", () => {
    expect(commentExcerpt("  uma\n\nlinha   só ")).toBe("uma linha só");
  });

  test("keeps a short comment intact", () => {
    expect(commentExcerpt("ok")).toBe("ok");
  });

  test("truncates on the character limit", () => {
    expect(commentExcerpt("abcdefghij", 5)).toBe("abcde…");
  });
});

describe("commentCountLabel", () => {
  test("covers the three cases", () => {
    expect(commentCountLabel(0)).toBe("Sem comentários");
    expect(commentCountLabel(1)).toBe("1 comentário");
    expect(commentCountLabel(4)).toBe("4 comentários");
  });
});

describe("boardWindowNote", () => {
  test("is silent when every post is shown", () => {
    expect(boardWindowNote(3, 3)).toBe("");
    expect(boardWindowNote(0, 0)).toBe("");
  });

  test("names the slice when the board is longer than the payload", () => {
    expect(boardWindowNote(45, 20)).toBe(
      "Mostrando os 20 posts mais recentes de 45.",
    );
  });
});
