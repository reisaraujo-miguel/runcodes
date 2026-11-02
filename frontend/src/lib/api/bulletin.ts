import { apiGet, apiPost } from "./client";
import type { EnrollmentRole } from "./enrollments";

/**
 * One comment on a bulletin-board post.
 *
 * `parent_comment_id` answers another comment of the same post; null means the
 * comment is on the post itself. The author's `role` is their role in the class
 * (the owner is shown as a professor even without an enrollment row).
 */
export interface BulletinComment {
  id: number;
  parent_comment_id: number | null;
  author_id: number;
  author_name: string;
  author_role: EnrollmentRole;
  body: string;
  created_at: string;
}

/** One post with the discussion under it. */
export interface BulletinPost {
  id: number;
  author_id: number;
  author_name: string;
  author_role: EnrollmentRole;
  title: string;
  body: string;
  created_at: string;
  comments: BulletinComment[];
}

/** The class page's board: the newest posts and what the caller may do. */
export interface BulletinBoard {
  offering_id: number;
  /** True for the class staff (owner, professors, monitors). */
  can_post: boolean;
  /** Total posts on the board; the payload carries only the newest ones. */
  post_count: number;
  posts: BulletinPost[];
}

/** Load a class's bulletin board. */
export function getOfferingBulletin(
  offeringId: number,
): Promise<BulletinBoard> {
  return apiGet<BulletinBoard>(
    `/api/v1/offerings/${String(offeringId)}/bulletin`,
  );
}

/**
 * Publish an announcement. Only the class staff may call it; the server refuses
 * anybody else. Every enrolled student is emailed the post in the background.
 */
export function createBulletinPost(
  offeringId: number,
  payload: { title: string; body: string },
): Promise<BulletinPost> {
  return apiPost<BulletinPost>(
    `/api/v1/offerings/${String(offeringId)}/bulletin/posts`,
    payload,
  );
}

/**
 * Comment on a post, optionally answering another comment of the same post.
 * The reply is what decides who is notified: a reply to a student reaches only
 * that student, anything else reaches the class staff.
 */
export function createBulletinComment(
  offeringId: number,
  postId: number,
  payload: { body: string; parent_comment_id: number | null },
): Promise<BulletinComment> {
  return apiPost<BulletinComment>(
    `/api/v1/offerings/${String(offeringId)}/bulletin/posts/${String(postId)}/comments`,
    payload,
  );
}
