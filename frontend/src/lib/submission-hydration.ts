/**
 * The commit the submission page opens on.
 *
 * `submitted` is a commit queued in this visit; it always wins, because a
 * history list requested before that submission existed may still point at an
 * older run.
 *
 * `latest` is the newest submission the server knows about. It is the normal
 * starting point, because `stored` only exists in the tab that made the
 * submission: a new tab, another browser, or a restarted browser has none, and
 * the page must not claim the student never submitted.
 *
 * `stored` is only a fallback for a history request that failed, where showing
 * the run this tab remembers beats showing nothing. When the history loads and
 * has no submissions, it is ignored: the server is authoritative.
 */
export function initialCommitId(
  submitted: number | null,
  latest: number | null,
  stored: number | null,
  historyFailed: boolean,
): number | null {
  if (submitted !== null) return submitted;
  if (latest !== null) return latest;
  return historyFailed ? stored : null;
}
