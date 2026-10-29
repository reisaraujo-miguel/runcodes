---
title: Production checklist
description: A go-live checklist for a real RunCodes deployment, from DNS and secrets to restore drills and alerting.
---

# Production checklist

Use this checklist before a real class depends on the deployment. It applies to
the single-host [image-based Compose](./image-compose.md) deployment and the
four-machine [Ansible](./ansible-setup.md) deployment. Tick every item that
applies, and record the exceptions.

## 1. Decide the topology and its owner

- [ ] Choose image-based Compose or the four-machine Ansible deployment.
- [ ] Record who operates the host, who holds the secrets and who is on call.
- [ ] Decide the backup schedule and the acceptable data-loss window.
- [ ] Decide the maintenance window for upgrades.
- [ ] Confirm the deployment is **not** expected to be highly available. Every
      supported path runs one replica of each service.

## 2. Prepare hosts, DNS and TLS

- [ ] Provision the hosts with the CPU, memory and disk in
      [Security & scaling](./security.md#capacity-starting-points).
- [ ] Point the public DNS name at the edge host.
- [ ] Allow inbound 80 and 443 on the edge only.
- [ ] On a rootless Podman host, allow the deployment user to bind 80/443
      (`net.ipv4.ip_unprivileged_port_start=80`), or front the stack with a
      proxy on loopback ports.
- [ ] Confirm `https://<public name>/` serves a valid certificate and that
      `http://` redirects to it.
- [ ] Confirm sign-in works over the public origin; the session cookie is
      `Secure` and is not sent over plain HTTP.
- [ ] Restrict SSH to known addresses.
- [ ] Restrict every private port to the deployment peers at the provider
      firewall.
- [ ] Confirm Caddy can obtain and renew a certificate for the public name.
- [ ] Set the host time zone and keep NTP enabled. Exercise deadlines and logs
      depend on the clock.

## 3. Generate and store secrets

- [ ] `RUNCODES_JWT_SECRET`: at least 32 random bytes.
- [ ] `RUNCODES_DB_PASSWORD`: database owner.
- [ ] `RUNCODES_DB_APP_PASSWORD`: backend role.
- [ ] `RUNCODES_DB_JUDGE_PASSWORD`: judge role.
- [ ] `RUNCODES_REDIS_PASSWORD`.
- [ ] `RUNCODES_JUDGE_TOKEN`: shared backend/judge bearer token.
- [ ] `RUNCODES_S3_CREDENTIALS_KEY` and `_SECRET`.
- [ ] Admin bootstrap password.
- [ ] Grafana admin password, Alertmanager credentials and SMTP credentials.
- [ ] Store every secret in a secret manager or encrypted vault, not in Git.
- [ ] Confirm no `REPLACE_WITH` placeholder or empty required value remains.
- [ ] Avoid a literal `$` in Ansible vault values; `podman-compose` interpolates
      it.

Never reuse the example values. A published JWT secret lets anyone forge a
session, and reusing one database password for the app and judge roles crosses
the trust boundary between them.

## 4. Configure the application

- [ ] Set `RUNCODES_PUBLIC_URL` to the exact browser origin (`https://...`, no
      path).
- [ ] Set `RUNCODES_DOMAIN` for Caddy.
- [ ] Set the SMTP relay, port, sender and credentials.
- [ ] Set the admin email and whether the bootstrap should run.
- [ ] Set the database and object-store connection values for each service.
- [ ] Set `RUNCODES_JUDGE_STALE_TIMEOUT` **above** the judge's
      `JUDGE_MAX_RUN_DURATION` plus margin.
- [ ] Set `RUNCODES_JUDGE_MAX_QUEUE_DEPTH` for the expected class size.
- [ ] Set `JUDGE_CONCURRENCY` for the compute host.
- [ ] Decide whether deadline reminders are enabled.
- [ ] Set the deployment environment name for telemetry.

## 5. Build or pull the images

- [ ] Pin the application image tag, not `latest`, for a deployment you need to
      reproduce.
- [ ] Pin `RUNCODES_IMAGE_REGISTRY`, `RUNCODES_RUNNER_IMAGE_PREFIX` and the
      runner tag to the registry and release the deployment should consume.
- [ ] Pre-pull every runner language the institution uses.
- [ ] Confirm the judge and runner images are from the same release. A judge
      with an old runner image can time out on compilation milestones.
- [ ] Keep the previous release tags available for rollback.

## 6. First boot

- [ ] Start the stack and confirm every service is running.
- [ ] Bootstrap the seeded administrator.
- [ ] Sign in and change the admin password.
- [ ] Create a second administrator.
- [ ] Set the contact email and login disclaimer in **Configurações**.
- [ ] Promote the first professor account.
- [ ] Confirm `GET /readyz` on the backend returns `database: ok`.
- [ ] Confirm the judge `/readyz` returns `ok`, not `podman unavailable`.
- [ ] Run a real submission from a test class and confirm **Concluído**.
- [ ] Download the submission source and, if produced, the output archive.
- [ ] Complete a signup and email verification through the real relay.
- [ ] Request a password reset and complete it.
- [ ] Disable or remove the test class and test accounts after the checks.

## 7. Protect the data

- [ ] Back up PostgreSQL and SeaweedFS together on a schedule.
- [ ] Store backups encrypted and off the deployment hosts.
- [ ] Keep the secrets needed to restore (database owner password, S3 keys).
- [ ] Confirm Redis is not treated as durable data.
- [ ] Set a retention for output archives that matches student expectations.
- [ ] Set a retention for logs, traces and metrics that fits the disk.
- [ ] Restore the latest backup into a scratch environment and run a submission.
- [ ] Record the last restore-drill date and the person who performed it.

See [Platform operations § Backups](../administration/operations.md#backups).

## 8. Add observability and alerting

- [ ] Start the observability stack for the deployment.
- [ ] Point Alertmanager at a real receiver, not the test sink.
- [ ] Send a test alert and confirm it arrives.
- [ ] Check that Prometheus targets are up, including the database, Redis,
      node, blackbox and OTLP targets.
- [ ] Open the submission dashboard and confirm traces and logs arrive.
- [ ] Check that the browser trace reaches Tempo (the four-machine deployment
      routes it through `/otel`).
- [ ] Record the dashboards and queries the on-call person should open first.
- [ ] Set retention and disk limits for Prometheus, Loki, Tempo and Pyroscope.

The procedures are in [Observability](../observability/index.md) and the
[runbook](../observability/runbook.md).

## 9. Harden the deployment

- [ ] Only the edge is reachable from untrusted networks.
- [ ] The backend, judge, database, object store, exporters and Grafana are on
      private or loopback addresses.
- [ ] The backend uses `runcodes_app`; the judge uses `runcodes_judge`.
- [ ] The database owner role is used only for migrations and bootstrap.
- [ ] The judge container can reach the rootless Podman socket and nothing else
      needs that socket.
- [ ] Container images and the repository are scanned with Trivy.
- [ ] Admin accounts are personal, unique and at least two.
- [ ] Test and demo accounts are removed or unconfirmed before launch.
- [ ] The login disclaimer and contact address are accurate.
- [ ] The class and account data retention policy is documented.

## 10. Size limits and capacity

- [ ] Disk has room for runner images (roughly 15 GB), workspaces, the database,
      the object store and telemetry.
- [ ] The judge concurrency does not oversubscribe the compute host.
- [ ] The queue depth limit matches the expected submission burst.
- [ ] Per-case limits in exercises are below the deployment's cgroup backstops.
- [ ] The stale timeout is longer than the longest expected run.
- [ ] The artifact retention and object-store growth are understood.
- [ ] A classroom behind one NAT fits the rate limits.

## 11. Hand over the operations

- [ ] The on-call person can reach the deployment hosts and the secret store.
- [ ] The runbook explains how to check readiness, logs and queue depth.
- [ ] The upgrade procedure and rollback boundary are documented.
- [ ] The backup restore procedure is documented and tested.
- [ ] The certificate renewal path is known.
- [ ] The person who can rotate the JWT secret and judge token is known.
- [ ] The support address and escalation path are visible to users.

## 12. Go-live smoke test

Run this sequence in one sitting:

1. Open the public URL in a private window.
2. Sign up, verify the email and sign in.
3. Join a test class with a code.
4. Submit a correct solution and a wrong solution.
5. Confirm the correct run reaches **Concluído** and the wrong one
   **Não concluído**.
6. Download the source of the submission and, if produced, the output archive.
7. As a professor, open **Notas** and download the CSV.
8. Send a class email and confirm delivery.
9. Change the contact email and see it on the login page.
10. Confirm an alert reaches the on-call receiver.
11. Check the backup job's latest run and its timestamp.
12. Delete the test class and the test accounts.

If any step fails, fix it before the first real class. A test that only checks
the homepage does not exercise the judge, the object store or the mail relay.

## After launch

For the first week:

- Check readiness, queue depth and disk every day.
- Watch the submission error rate after each class deadline.
- Confirm backups complete and are readable.
- Check certificate renewal and alert delivery.
- Ask the first professors for friction in the authoring flow.
- Keep a change log of configuration and image tags.

## Related pages

- [Deployment overview](./index.md)
- [Image-based Compose](./image-compose.md)
- [Compose from source](./compose.md)
- [Ansible setup](./ansible-setup.md)
- [Ansible operations](./ansible-operations.md)
- [Security & scaling](./security.md)
- [Platform operations](../administration/operations.md)
- [Observability](../observability/index.md)
- [Configuration](../reference/configuration.md)
