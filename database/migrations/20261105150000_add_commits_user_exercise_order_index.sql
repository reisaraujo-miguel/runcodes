-- Submission numbers are per user and exercise, ordered by created_at then id.
-- This index serves both the history's window function and the count that reads
-- one commit's position, without scanning the student's whole commit history.
CREATE INDEX idx_commits_user_exercise_order
  ON commits (user_id, exercise_id, created_at, id);
