#!/usr/bin/env bash
#
# Creates the local Grafana admin password used by the observability overlay.
#
# The file lives in .observability/ (gitignored) and is mounted read-only into
# the Grafana container. It is generated, never committed. For a real deployment
# use your platform's secret manager and set GF_SECURITY_ADMIN_PASSWORD__FILE to
# the mounted path instead; this script is for the local development stack.
#
# The directory is world-traversable and the file world-readable on purpose: the
# Grafana container runs as UID 472 (mapped to a subuid by rootless Podman), so a
# 0600 file owned by the invoking user cannot be read inside the container. The
# parent directory is the user's home, which is typically not world-traversable.
set -euo pipefail

root="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
state_dir="$root/.observability"
password_file="$state_dir/grafana_admin_password"
webhook_file="$state_dir/alertmanager_webhook_url"
gmail_password_file="$state_dir/alertmanager_gmail_app_password"

mkdir -p "$state_dir"
chmod 0755 "$state_dir"

if [[ ! -s "$password_file" ]]; then
  tmp="$(mktemp "$state_dir/.grafana-password.XXXXXX")"
  trap 'rm -f "$tmp"' EXIT
  # 32 hex characters; avoid / and + so the password is safe in curl -u and in
  # a password manager copy/paste. 24 random bytes give 48 hex chars, truncated.
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24 | cut -c1-32 > "$tmp"
  else
    head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32 > "$tmp"
  fi
  mv "$tmp" "$password_file"
  trap - EXIT
  echo "observability: generated Grafana admin password at $password_file"
else
  echo "observability: keeping existing Grafana admin password"
fi

chmod 0644 "$password_file"

# Alertmanager reads its webhook URL from this file. The default is the local
# alert sink so the smoke test works out of the box; replace the contents with a
# real Slack/PagerDuty-compatible webhook in a real deployment, ideally through
# your secret manager. Never commit the real URL.
if [[ ! -s "$webhook_file" ]]; then
  printf '%s\n' "http://alert-sink:8080/alerts" > "$webhook_file"
  echo "observability: generated Alertmanager webhook file at $webhook_file"
else
  echo "observability: keeping existing Alertmanager webhook file"
fi
chmod 0644 "$webhook_file"

# Optional Gmail receiver. Alertmanager reads a Google app password from this
# file when ALERTMANAGER_CONFIG_FILE points at the Gmail example. Leave it empty
# until Gmail is enabled; the default config does not reference it. Run
# `printf '%s\n' '<app-password>' > .observability/alertmanager_gmail_app_password`
# to fill it (remove the spaces Google shows in the 16-character password).
if [[ ! -e "$gmail_password_file" ]]; then
  : > "$gmail_password_file"
  echo "observability: created empty Alertmanager Gmail app password file at $gmail_password_file"
else
  echo "observability: keeping existing Alertmanager Gmail app password file"
fi
chmod 0644 "$gmail_password_file"

echo "observability: admin user is '${GRAFANA_ADMIN_USER:-admin}'"
echo "observability: password file $password_file (do not commit it)"
echo "observability: Alertmanager webhook file $webhook_file (do not commit a real URL)"
echo "observability: Alertmanager Gmail app password file $gmail_password_file (do not commit a real app password)"
