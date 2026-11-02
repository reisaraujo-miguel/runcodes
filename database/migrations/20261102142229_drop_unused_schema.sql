-- Modify "exercises" table
ALTER TABLE "exercises" DROP COLUMN "type";
-- Drop "alerts" table
DROP TABLE "alerts";
-- Drop enum type "alert_recipients_t"
DROP TYPE "alert_recipients_t";
-- Drop enum type "exercise_type_t"
DROP TYPE "exercise_type_t";
-- Drop enum type "alert_type_t"
DROP TYPE "alert_type_t";
-- Drop "mail_logs" table
DROP TABLE "mail_logs";
-- Drop "system_disk_reports" table
DROP TABLE "system_disk_reports";
-- Drop "system_logs" table
DROP TABLE "system_logs";
-- Drop "user_messages" table
DROP TABLE "user_messages";
