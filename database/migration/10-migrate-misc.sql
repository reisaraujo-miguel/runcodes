-- Remaining tables: system logs, mail logs, user messages.
-- (system_disk_reports is intentionally left empty: the legacy monitoring
-- data was unreliable and is discarded.)

-- System logs (legacy `logs`). user_email -> user_id (NULL when the user no
-- longer exists); unparseable IPs become 0.0.0.0 because ip is NOT NULL.

INSERT INTO public.system_logs (id, user_id, log_time, ip, action)
SELECT
  l.id,
  um.new_id,
  _migration.to_tstz(l.datetime),
  COALESCE(_migration.safe_inet(l.ip), '0.0.0.0'),
  l.action
FROM old.logs l
LEFT JOIN _migration.users_map um ON um.old_email = l.user_email;

-- Mail logs. opened/first_opened_time are dropped; sent_date -> sent_at.

INSERT INTO public.mail_logs (id, sent_to, subject, message, hash, sent_at)
SELECT
  m.id,
  COALESCE(m.sent_to, ''),
  COALESCE(m.subject, ''),
  COALESCE(m.message, ''),
  COALESCE(m.hash, ''),
  COALESCE(_migration.to_tstz(m.sent_date), NOW())
FROM old.mail_logs m;

-- User messages (legacy `messages`). The old table had no owner, so user_id
-- is NULL.

INSERT INTO public.user_messages (
  id, user_id, recipes, subject, message, attachments, template, created_at
)
SELECT
  m.id,
  NULL,
  COALESCE(m.recipes, ''),
  COALESCE(m.subject, ''),
  COALESCE(m.message, ''),
  m.attachments,
  m.template,
  NOW()
FROM old.messages m;
