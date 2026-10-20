-- Alerts.
--
-- user_email -> user_id (NULL if the legacy user is somehow missing).
--
-- type mapping (legacy ints):
--   0 warning | 1 danger | 2 info | 3 success | anything else -> info
-- recipients mapping (legacy ints):
--   0 students | 1 monitors | 2 professors | anything else -> all
--
-- valid was a date in the old schema; valid_until becomes the end of that day
-- (America/Sao_Paulo) so the alert stays valid for the whole day.
-- read/created_at have no old equivalent: FALSE / migration time.

INSERT INTO public.alerts (
  id, type, offering_id, recipients, user_id, valid_until,
  title, message, created_at, read
)
SELECT
  a.id,
  CASE a.type
    WHEN 0 THEN 'warning'
    WHEN 1 THEN 'danger'
    WHEN 2 THEN 'info'
    WHEN 3 THEN 'success'
    ELSE 'info'
  END::public.alert_type_t,
  a.offering_id,
  CASE a.recipients
    WHEN 0 THEN 'students'
    WHEN 1 THEN 'monitors'
    WHEN 2 THEN 'professors'
    ELSE 'all'
  END::public.alert_recipients_t,
  um.new_id,
  (a.valid::timestamp + INTERVAL '1 day' - INTERVAL '1 second')
    AT TIME ZONE 'America/Sao_Paulo',
  a.title,
  a.message,
  NOW(),
  FALSE
FROM old.alerts a
LEFT JOIN _migration.users_map um ON um.old_email = a.user_email;
