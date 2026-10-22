---
title: Documentation map
description: The canonical documentation policy and where every topic now lives.
---

# Documentation map

The published site is the **canonical documentation** for RunCodes. Repository
Markdown files are short entry points: they hold the commands needed in that
directory and link here for everything else.

This avoids two copies drifting apart. When behaviour changes, update the page
in [`docs-site/docs/`](https://github.com/reisaraujo-miguel/runcodes/tree/main/docs-site/docs)
and leave the README pointer alone.

## Repository entry points

| File | Purpose |
| --- | --- |
| `README.md` | project landing: what it is, services, quick start, deployment links |
| `CONTRIBUTING.md` | issues, pull requests, checks, database and docs rules |
| `backend/README.md` | how to run and test the backend |
| `frontend/README.md` | how to run and check the SPA |
| `judge/README.md` | how to build, run and integration-test the judge |
| `judge/integration/README.md` | integration harness requirements and commands |
| `monitor/README.md` | monitor usage, report format and test |
| `runners/README.md` | image build, test and the judge contract pointer |
| `database/README.md` | schema change and migration commands |
| `database/legacy-migration/README.md` | importer requirements and commands |
| `deploy/README.md` | deployment entry point |
| `deploy/ansible/README.md` | four-machine quick start and operating commands |
| `docs/README.md` | pointer to this site (the old guides were retired) |
| `docs-site/README.md` | how to edit and deploy the documentation site |

## Where each topic lives

| Topic | Page |
| --- | --- |
| Product overview, services, repository layout | [What is RunCodes?](../guide/overview.md) |
| Component architecture, submission lifecycle, security boundaries | [Architecture](../guide/architecture.md) |
| First local run, secrets, admin bootstrap, verification | [Getting started](../guide/getting-started.md) |
| Student, professor and admin workflows | [Using RunCodes](../guide/using-runcodes.md) |
| Statuses, SSE events, artifacts, failure handling | [Submissions & live judging](../guide/submissions.md) |
| Classes, enrollment roles, deadlines, permissions | [Classes, roles & access](../guide/classes-and-roles.md) |
| Signup, verification, sessions, password policy | [Accounts & settings](../guide/accounts-and-settings.md) |
| SPA routes, stack, editor, browser telemetry | [Frontend application](../guide/frontend.md) |
| Backend internals and readiness | [Backend API](../services/backend.md) |
| Judge queue, contract, limits, isolation | [Judge](../services/judge.md) |
| Runner images, harness, monitor, release contract | [Runner images & monitor](../services/runners-and-monitor.md) |
| Schema, Atlas migrations, legacy import | [Database](../services/database.md) |
| HTTP API (auth, classes, submissions, admin) | [API overview & authentication](../api/index.md) |
| Internal judge API and event schemas | [Judge API](../api/judge.md) |
| Compose vs Ansible, topology | [Deployment overview](../deployment/index.md) |
| Single-host deployment | [Docker Compose](../deployment/compose.md) |
| Four-machine setup | [Ansible deployment: setup](../deployment/ansible-setup.md) |
| Upgrades, backups, migrations, troubleshooting | [Ansible deployment: operations](../deployment/ansible-operations.md) |
| Security model, limits, scaling, capacity | [Security & scaling](../deployment/security.md) |
| Observability stack and signals | [Observability overview](../observability/index.md) |
| Local telemetry stack | [Local observability stack](../observability/local.md) |
| Four-machine telemetry | [Four-machine observability](../observability/production.md) |
| Telemetry procedures and failure modes | [Observability runbook](../observability/runbook.md) |
| Workflows, image releases, test gates | [CI/CD & releases](../ci-cd.md) |
| Environment variables and deployment settings | [Configuration reference](./configuration.md) |
| Ports, buckets, volumes, limits | [Ports & storage](./ports-and-storage.md) |
| Languages, extensions and image names | [Languages & images](./languages.md) |
| Terms and concepts | [Glossary](./glossary.md) |
| How this site is built and deployed | [Documentation site](../documentation.md) |

## Retired files

These files were consolidated into the site and removed from the repository, so
there is one place to update:

| Retired file | Content now in |
| --- | --- |
| `docs/observability/README.md` | [Observability overview](../observability/index.md), [Local stack](../observability/local.md) |
| `docs/observability/runbook.md` | [Observability runbook](../observability/runbook.md) |
| `deploy/ansible/docs/ARCHITECTURE.md` | [Deployment overview](../deployment/index.md), [Security & scaling](../deployment/security.md) |
| `deploy/ansible/docs/DEPLOYMENT.md` | [Ansible setup](../deployment/ansible-setup.md), [Ansible operations](../deployment/ansible-operations.md) |
| `deploy/ansible/docs/OBSERVABILITY.md` | [Four-machine observability](../observability/production.md) |
| `judge/DESIGN.md` | [Judge](../services/judge.md), [Judge API](../api/judge.md) |
| Long service READMEs | The matching service or guide page |

## Documentation rules

1. **Long-form content belongs in `docs-site/docs/`.** Do not add guides to
   service READMEs or a new `docs/` subtree.
2. **READMEs stay operational.** A service README may keep commands, required
   files and a link to this site. It should not repeat the architecture, API or
   operational reasoning.
3. **Link to the published page.** Use
   `https://reisaraujo-miguel.github.io/runcodes/...` from repository files so
   the link works on GitHub. Use relative `.md` links between site pages.
4. **Update the page, then build.** Run `npm ci && npm run build` in
   `docs-site/`; VitePress fails on a dead internal link.
5. **Do not link to retired files.** If a link points at one of the paths in the
   table above, point it at the replacement page instead.
6. **Keep the docs workflow green.** `.github/workflows/docs.yml` builds on pull
   requests and deploys `main` to Pages; see
   [Documentation site](../documentation.md).

## Related pages

- [Documentation site](../documentation.md) — VitePress setup, local commands,
  deployment and page authoring.
- [Contributing](../contributing.md) — development setup and checks.
