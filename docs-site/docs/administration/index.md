---
title: Administrator guide
description: The admin panel, platform-wide responsibilities, role preview and the first-run checklist.
---

# Administrator guide

An **administrator** account can edit every account, manage any class's metadata
and ownership, edit the public settings and maintain the file type catalog. This
guide covers the admin panel and the decisions an administrator makes. Day-2
infrastructure work, backups and incident response are in
[Platform operations](./operations.md).

The RunCodes interface is in Brazilian Portuguese. This page shows the labels as
they appear on screen and gives the English meaning the first time a label is
used.

## What an administrator can do

The **Administração** (Administration) section of the sidebar has five pages:

| Page | Purpose |
| --- | --- |
| **Painel** (Dashboard) | overview, role preview and shortcuts |
| **Usuários** (Users) | search, edit, confirm and delete accounts |
| **Todas as Turmas** (All classes) | edit any class, transfer ownership, inspect members, delete |
| **Tipos de Arquivo** (File types) | maintain the extension catalog used by exercises |
| **Configurações** (Settings) | contact email and login-page disclaimer |

An administrator also has the teaching area, but only for classes they own or
teach. The admin panel uses its own endpoints and does not grant the owner's
authoring routes over every class. See
[Role boundaries](#role-boundaries) below.

## Roles at a glance

| Role | Scope | Main abilities |
| --- | --- | --- |
| `student` | platform | join classes, submit solutions, read own results |
| `professor` | platform | own classes, author exercises, grade a class |
| `admin` | platform | manage all users and classes, settings, file types |
| `dev` | legacy | retained for compatibility; not used by the UI |

Inside a class, the enrollment role is separate: `student`, `monitor` or
`professor`. The owner is the account in `offerings.owner_id`, not an enrollment
role. See [Classes, roles & access](../guide/classes-and-roles.md) for the full
matrix.

## The dashboard

**Painel** shows four counts derived from the first page of the user and class
listings:

- **Usuários** (Users);
- **Turmas** (Classes);
- **Turmas abertas** (Open classes): enrollment open and end date in the future;
- **Matrículas** (Enrollments): the sum of members in the loaded classes.

The page states its own limit: the counts cover at most 200 records per listing,
not the whole platform. Use **Usuários** and **Todas as Turmas** to search beyond
that page.

The dashboard is also where an administrator can start a role preview.

## Preview the platform as another role

**Visualizar como** (View as) changes the role claim of your own session, not the
account. You can preview **Estudante** (Student) or **Professor**. The admin panel
is hidden while the preview is active, and your own name, email and data stay the
same.

Use the preview to check:

- what a student sees on the dashboard and class pages;
- whether an exercise is visible to students at the right time;
- whether a professor can reach the teaching area;
- whether navigation and empty states make sense for the role.

Stop the preview from the account menu at the bottom of the sidebar:
**Voltar para administrador** (Return to administrator). The preview uses a
normal session lifetime. If the session expires during the preview, sign in
again.

The preview is not impersonation: you do not see another person's classes,
grades or submissions. It only removes your admin role for navigation and
permission checks.

## Role boundaries

The admin role and class ownership are different things:

- The **admin panel** can edit any class's name, description, end date,
  enrollment setting and owner, and can list its members and delete it.
- The admin panel **cannot** create exercises, edit test cases, view a
  gradebook or submit for a class you do not own or teach.
- A professor or administrator who **owns** a class, or was added as a
  co-professor, gets the full teaching routes for that class.
- An administrator can transfer ownership to themselves, work on the class, and
  transfer it back. Do this deliberately: the previous owner loses owner
  privileges immediately.

If you need to inspect grades, ask the owner to add you as a co-professor. That
is less disruptive than a temporary ownership transfer.

## First-run checklist

After a fresh deployment:

1. **Bootstrap the seeded admin.** The `admin@admin.com` account has an unusable
   password until the bootstrap script runs. See
   [Getting started](../guide/getting-started.md#_3-bootstrap-the-admin-account).
2. **Sign in and change the password.** Open **Meu perfil** and use
   **Alterar senha**.
3. **Create a second administrator.** Promote a trusted account so the platform
   does not depend on one person. The panel cannot create accounts; the person
   signs up first.
4. **Set the public contact information.** In **Configurações**, set the support
   email and the optional login disclaimer.
5. **Promote the professors.** Change the role of each teaching account from
   `student` to `professor` in **Usuários**.
6. **Check the file type catalog.** Compare **Tipos de Arquivo** with the
   languages the deployed runner images support.
7. **Verify email delivery.** Sign up a test account and complete the
   verification flow with the production SMTP relay.
8. **Verify the judge.** Check both readiness endpoints and run a real
   submission from a test class.
9. **Configure backups and telemetry.** Follow the deployment guide for the
   database and object-store backup, and enable the observability stack when you
   need alerting.
10. **Document ownership.** Record who receives alerts, who can restore backups
    and who can rotate secrets.

The [production checklist](../deployment/checklist.md) expands these steps for a
real deployment.

## Manage accounts

**Usuários** searches name, email and institutional id and lists the matching
accounts with their role and confirmation status.

An administrator can:

- edit the name, email, institutional id and role;
- confirm or unconfirm an account;
- delete an account.

An administrator cannot:

- create an account (the person signs up);
- set or reset another account's password (the person uses **Esqueceu a
  senha?**);
- change their own role or email, unconfirm themselves, or delete themselves;
- add or remove class members.

Changing a role, email or confirmation signs the account out on its next request.
Deleting a user also deletes the classes they own and their submissions. Read
[User & role administration](./users.md) before a destructive change.

## Manage classes

**Todas as Turmas** searches by class name, owner name or owner email. It lists
the owner, member and exercise counts, enrollment status and end date.

An administrator can:

- edit the class name, description, end date and enrollment setting;
- transfer ownership by email or institutional id;
- list the members of any class;
- delete a class with its exercises, test cases and submissions.

An administrator cannot edit the class's exercises or test cases, add members or
view grades from this page. The owner and co-professors do that. See
[Class & ownership administration](./classes.md).

## Maintain the file type catalog

**Tipos de Arquivo** lists every extension the platform can offer to a professor.
Each entry has:

- a display name;
- an extension without the leading dot (`py`, `omp.cpp`);
- **Compilado** (Compiled) or **Interpretado** (Interpreted);
- **Disponível** (Available) or **Oculto** (Hidden).

Registering a type makes it available in the exercise form immediately, with no
redeploy. Hiding a type removes it from the picker for new exercises. Deleting a
type is refused while an exercise references it; hide it instead.

The catalog is not the execution engine. A judge language mapping and a runner
image still decide how a file runs. Registering `rb` does not add Ruby support.
See [Platform settings & file types](./platform.md#file-types) and
[Languages & images](../reference/languages.md).

## Protect the administrator account

- Use a unique, long password and keep the account email current.
- Keep at least two administrators; a single account is a single point of
  failure.
- Do not share accounts. Most administrative actions are logged; user changes
  record the administrator's account id. There is no audit-log page in the panel.
- Preview a role instead of creating a test student account for routine checks.
- Do not transfer ownership of a class only to read its grades if adding a
  co-professor is possible.
- Sign out on shared machines. Logout clears the cookie, but a copied session
  token stays valid until it expires.
- Rotate the JWT secret if you suspect a session was compromised. This signs out
  every account.

## Where administrative work lives

| Task | Page |
| --- | --- |
| Accounts, roles, confirmation, deletion | [Users & role administration](./users.md) |
| Class metadata, ownership transfer, deletion | [Class & ownership administration](./classes.md) |
| Settings, file types, mail, preview | [Platform settings & file types](./platform.md) |
| Health checks, backups, upgrades, incidents | [Platform operations](./operations.md) |
| Infrastructure deployment | [Deployment overview](../deployment/index.md) |
| Metrics, logs, traces and alerts | [Observability](../observability/index.md) |
| Environment variables | [Configuration](../reference/configuration.md) |

## Related pages

- [Professor guide](../guide/professor-guide.md)
- [Student guide](../guide/student-guide.md)
- [Classes, roles & access](../guide/classes-and-roles.md)
- [Security & scaling](../deployment/security.md)
- [Production checklist](../deployment/checklist.md)
