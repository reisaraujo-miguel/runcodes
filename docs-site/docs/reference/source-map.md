---
title: Documentation coverage
description: A map from every source document in the repository to the page on this site that covers it.
---

# Documentation coverage

This site consolidates the repository's documentation. The table below maps each
source document to the pages that carry its content, so nothing is lost and
readers can find the original context.

## Source map

| Source document | Covered by |
| --- | --- |
| `README.md` | [Overview](../guide/overview.md), [Architecture](../guide/architecture.md), [Getting started](../guide/getting-started.md), [Deployment](../deployment/index.md), [Observability](../observability/index.md), [Releases](../ci-cd.md) |
| `CONTRIBUTING.md` | [Contributing](../contributing.md) |
| `CONTRIBUTORS.md` | [Contributing § Contributors](../contributing.md#contributors) |
| `backend/README.md` | [Backend](../services/backend.md), [Accounts & settings](../guide/accounts-and-settings.md), [Submissions](../guide/submissions.md), [Users API](../api/auth.md) |
| `backend/.env.example` | [Configuration § Backend](../reference/configuration.md#backend) |
| `frontend/README.md` | [Frontend application](../guide/frontend.md) |
| `frontend/.env.example` | [Configuration § Frontend](../reference/configuration.md#frontend) |
| `judge/README.md` | [Judge](../services/judge.md) |
| `judge/DESIGN.md` | [Judge](../services/judge.md), [Judge API](../api/judge.md) |
| `judge/integration/README.md` | [Judge § Integration test](../services/judge.md#integration-test) |
| `judge/.env.example` | [Configuration § Judge](../reference/configuration.md#judge) |
| `monitor/README.md` | [Runner images & monitor](../services/runners-and-monitor.md) |
| `runners/README.md` | [Runner images & monitor](../services/runners-and-monitor.md), [Languages & images](../reference/languages.md) |
| `database/README.md` | [Database](../services/database.md) |
| `database/legacy-migration/README.md` | [Database § Legacy migration](../services/database.md#legacy-migration) |
| `docs/observability/README.md` | [Observability overview](../observability/index.md), [Local stack](../observability/local.md) |
| `docs/observability/runbook.md` | [Observability runbook](../observability/runbook.md) |
| `deploy/README.md` | [Deployment overview](../deployment/index.md) |
| `deploy/ansible/README.md` | [Ansible setup](../deployment/ansible-setup.md) |
| `deploy/ansible/docs/ARCHITECTURE.md` | [Deployment overview](../deployment/index.md), [Security & scaling](../deployment/security.md), [Architecture](../guide/architecture.md) |
| `deploy/ansible/docs/DEPLOYMENT.md` | [Ansible setup](../deployment/ansible-setup.md), [Ansible operations](../deployment/ansible-operations.md) |
| `deploy/ansible/docs/OBSERVABILITY.md` | [Four-machine observability](../observability/production.md) |
| `deploy/ansible/inventories/group_vars/all/config.yml.example` | [Configuration § Ansible deployment](../reference/configuration.md#ansible-deployment) |
| `deploy/ansible/inventories/group_vars/all/vault.yml.example` | [Ansible setup](../deployment/ansible-setup.md) |
| `.env.example` | [Configuration § Docker Compose](../reference/configuration.md#docker-compose) |
| `.github/workflows/*` | [CI/CD & releases](../ci-cd.md) |
| `trivy.yaml`, `.trivyignore.yaml` | [CI/CD & releases](../ci-cd.md), [Security](../deployment/security.md) |

## What was added

The source docs are strong in depth. This site adds:

- a single navigation structure across the application, services, API,
  deployment and operations;
- an HTTP API reference reconstructed from the route table, handlers and models;
- a configuration reference that consolidates every `.env.example`, the Ansible
  `config.yml` and the observability overlay;
- a ports, storage and retention reference;
- a complete language/extension/image matrix from `judge/internal/language`;
- cross-links between the design contract, the deployment guide and the
  runbooks;
- a coverage map (this page) so readers can trace a statement back to its source.

## Known source-document inconsistencies

Where the repository documents disagreed, this site follows the current code or
the most recent operational document:

| Topic | Resolution |
| --- | --- |
| Browser tracing | `judge/README.md` contains an older note that browser tracing is not implemented; the SPA does emit fetch spans when built with `VITE_OTEL_ENABLED=true`. This site documents the implemented behaviour. |
| Runner count | There are 22 language images: 12 inherit from the base image and 10 copy the harness from it. Some source comments say 23 or 11; the count here follows the directories and Dockerfiles. |
| Observability PR summaries | `docs/observability/README.md` links to `PR-SUMMARY*.md` files that were removed. This site does not reproduce those links. |
| Legacy import path | The legacy guide says `cd database/migration`; the directory is `database/legacy-migration`. This site uses the actual path. |

## Keeping the site current

The docs live in `docs-site/` next to the code. When a change affects behaviour:

1. update the page that describes it;
2. update [Configuration](./configuration.md) if a variable was added or changed;
3. update the [source map](#source-map) if a new source document appears;
4. run `npm run build` in `docs-site/` to catch broken links.

See [Documentation site](../documentation.md).
