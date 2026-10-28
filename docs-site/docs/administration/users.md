---
title: Users & role administration
description: Search accounts, change roles, handle confirmation and email changes, and delete accounts safely.
---

# Users & role administration

**Usuários** (Users) is the account directory for the whole platform. This page
covers the safest way to change a role, confirm an account, fix an email address
and delete an account without losing class material by accident.

## Search and page through accounts

The page lists 200 accounts at a time and offers **Carregar mais** (Load more)
when more exist. Search matches, case-insensitively:

- the account name;
- the email address;
- the institutional id.

The API behind the page accepts `limit` and `offset` for scripts. The default
page is 50 and the maximum is 200. Accounts are ordered by database id, oldest
first.

## What you can edit

Select **Editar** (Edit) on a row to open the account form.

| Field | Effect |
| --- | --- |
| **Nome** (Name) | display name, at most 100 characters |
| **Email** | login address, unique across the platform |
| **Identificação institucional** (Institutional ID) | optional unique identifier, used for ownership transfer and search |
| **Papel** (Role) | `student`, `professor`, `admin` or `dev` |
| **Conta confirmada** (Confirmed account) | whether the account may sign in |

The form also shows the account id and creation date. The role selector is
disabled for your own account, as is the email and confirmation checkbox.

## Change a role

Platform roles decide what an account can do before it joins a class:

- `student` can join classes and submit. It cannot own a class.
- `professor` can own classes and teach them.
- `admin` can use the admin panel and also own classes.
- `dev` is a legacy value retained for compatibility.

Promote a professor after the person signs up and confirms the account. A role
change bumps the account's token version, so the current session stops working.
The person signs in again and the new role applies.

::: warning Transfer classes before demoting an owner
A class's owner must have the `professor` or `admin` platform role to exercise
owner privileges. If you change an owner to `student`, the class keeps the same
`owner_id` but the account can no longer edit it, manage members or open the
teaching area. The class is effectively unmanaged until you assign a new owner.

Transfer the class first in **Todas as Turmas**, then change the role.
:::

A student cannot be assigned as a co-professor inside a class. The API refuses
that with `422`; promote the account first.

## Confirm an account

New accounts are unconfirmed until the person opens the verification link. An
unconfirmed account cannot sign in, and every protected route rejects an existing
session once the account is unconfirmed.

Use **Conta confirmada** when:

- the verification email cannot be delivered and the person's identity is known;
- an imported or legacy account needs access;
- a support case requires confirming a legitimate address.

Unchecking the box has the opposite effect: it signs the account out and blocks
the next sign-in. The account, its classes and its submissions remain.

The checkbox is disabled for your own account. You cannot unconfirm yourself and
lock the panel.

## Change an email address

Prefer the user's own **Meu perfil** flow when the person can sign in. That flow
requires the current password, sends a verification link to the new address and
keeps the old address active until the link is used.

The admin form is the trusted path when the person cannot access the old address:

- The new address is stored immediately.
- The **Conta confirmada** checkbox decides whether the account stays confirmed.
  Leaving it checked changes the login address without the verification step.
- The old pending email change, if any, is cleared.
- Every existing session is invalidated.

Only change an address you have verified through another channel. An email
address is an account recovery credential.

## Delete an account

Deleting an account is irreversible and removes much more than the profile:

- the account's submissions and their stored source files;
- the classes the account **owns**, with their exercises, test cases, files and
  every student's submissions in those classes;
- enrollments and memberships.

A student who owns no classes takes only their own submissions and enrollments
with them. A professor who owns classes takes the whole class down. Before
deleting a professor, transfer their classes to another professor, or ask the
owner or a co-professor to export each class from its **Ferramentas** tab.

You cannot delete your own account from the panel. Ask another administrator to
delete it, or leave it in place after transferring ownership.

There is no merge operation for duplicate accounts. Decide which account keeps
the history, transfer classes if needed, then delete the other one.

## Support playbooks

### A person cannot sign in

1. Search the email or institutional id.
2. If no account exists, the person needs to sign up. Accounts cannot be created
   from the panel.
3. If the account is unconfirmed, check the confirmation status and the email
   address. Ask the person to resend the verification email from the login page,
   or confirm the account yourself after verifying their identity.
4. If the account is confirmed, check the password. You cannot set it. Ask the
   person to use **Esqueceu a senha?** with an address they can read.
5. If the reset email does not arrive, the SMTP relay is failing or the address
   is wrong. See [Mail and notifications](./platform.md#mail-and-notifications).
6. If the account is banned from a class, the class owner must unban it. Class
   bans do not block the platform login.

### A person changed institutions or lost their email

1. Verify the new address through a channel you trust.
2. Change the email in **Usuários**.
3. Decide whether the account stays confirmed. If you are not sure the new
   address belongs to the person, unconfirmed is the safer state.
4. Ask the person to sign in and change the password.

### A person needs a different role

Change the role and ask them to sign in again. Students cannot be promoted to
co-professor from inside a class. Admins cannot change their own role; another
admin must do it.

### A person is leaving

1. Export or transfer the classes they own.
2. Remove them from classes they teach as co-professors, or let the owner do it.
3. Demote the account to `student` or leave it for reference.
4. Delete the account only after the data is no longer needed. Deletion is
   irreversible.

### An account may be compromised

1. Unconfirm the account or reset its email to a verified address.
2. Ask the person to reset the password with **Esqueceu a senha?**.
3. Check the classes the account owns and any recent admin actions in the server
   logs.
4. If the account is an administrator, promote a replacement first, then remove
   the role.
5. If you suspect a session token was copied, rotate `RUNCODES_JWT_SECRET` on the
   backend. This signs out every account, including yours.

### A person asks for their data to be deleted

Deleting the account removes their submissions and removable personal data. It
also removes classes they own, including other students' work. Transfer those
classes first if the institution needs to keep them. The deletion cannot be
undone.

## Bulk work and scripts

The panel has no bulk edit or bulk delete. For a large cleanup, use the API with
the admin account and page through `GET /api/v1/admin/users`:

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  'https://<host>/api/v1/admin/users?limit=200&offset=0' | jq
```

Treat a script as a destructive operation:

- take a database backup first;
- filter on exact ids, not on a partial email match;
- run the same query with a read-only listing before the mutation;
- remember that deleting a professor deletes their classes;
- check the logs after the run.

## Common tasks

| Task | Where |
| --- | --- |
| Find an account | **Usuários** search |
| Promote a professor | **Usuários → Editar → Papel** |
| Confirm a legitimate account | **Usuários → Editar → Conta confirmada** |
| Change a login email | **Usuários → Editar → Email** |
| Delete a student | **Usuários → Excluir** |
| Transfer a class before demotion | **Todas as Turmas → Editar → Novo responsável** |
| Give an admin access to a class's grades | ask the owner to add them as co-professor |
| Reset a password | the person uses **Esqueceu a senha?** |
| Create an account | the person signs up |

## Related pages

- [Administrator guide](./index.md)
- [Class & ownership administration](./classes.md)
- [Platform settings & file types](./platform.md)
- [Accounts & settings](../guide/accounts-and-settings.md)
- [API: administration](../api/admin.md)
