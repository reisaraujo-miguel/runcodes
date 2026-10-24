-- Deadline reminder bookkeeping.
--
-- The backend emails a student who has not submitted an exercise that closes in
-- seven days and again when it closes in three days. One row per threshold makes
-- the reminder idempotent: a restart, a second replica or a second run inside the
-- same window cannot send the same warning again.
CREATE TABLE exercise_deadline_notifications (
  exercise_id int NOT NULL REFERENCES exercises (id) ON DELETE CASCADE,
  user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  days_before int NOT NULL CHECK (days_before IN (3, 7)),
  sent_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
  PRIMARY KEY (exercise_id, user_id, days_before)
);

CREATE INDEX idx_exercise_deadline_notifications_user_id
  ON exercise_deadline_notifications (user_id);

-- The backend is the only service that reads this table, so it follows the same
-- row-level-security boundary as the other backend-only tables.
ALTER TABLE "exercise_deadline_notifications" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "runcodes_app_only" ON "exercise_deadline_notifications" TO PUBLIC
  USING (current_user = 'runcodes_app')
  WITH CHECK (current_user = 'runcodes_app');
