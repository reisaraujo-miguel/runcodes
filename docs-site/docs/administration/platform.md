---
title: Platform settings & file types
description: Edit the public contact information and disclaimer, maintain the file type catalog, and verify mail delivery.
---

# Platform settings & file types

Two admin pages shape the platform for everyone: **Configurações** (Settings)
controls the public contact information, and **Tipos de Arquivo** (File types)
controls the extensions a professor can assign to an exercise. This page explains
both, plus the mail and public-URL settings that live in the environment rather
than in the panel.

## Public settings

**Configurações** edits two values stored in the `platform_settings` table:

| Setting | Required | Limit | Where it appears |
| --- | :-: | --- | --- |
| **Email de contato** (Contact email) | yes | valid address | login page, terms, error-reporting instructions |
| **Aviso de contato (HTML)** (Contact notice) | no | 4000 characters | below the login card |

The disclaimer accepts HTML. The browser sanitizes it with DOMPurify before
rendering, but the stored value is what you submitted. Treat it as trusted admin
content and do not paste script tags: they are removed, and the visible result may
differ from what you expected.

An empty disclaimer hides the paragraph. The contact email is required; the form
refuses an empty or invalid address.

The public route `GET /api/v1/settings/public` serves both values before login.
The cache entry is invalidated on save, so a change appears on the next page load.
The build-time `VITE_CONTACT_EMAIL` and `VITE_CONTACT_DISCLAIMER_HTML` values are
only fallbacks for the moment before that request answers.

The page does not change the site name, logo, theme or language. Those are
build-time assets in the frontend image.

## File types

**Tipos de Arquivo** is the platform catalog of extensions. A professor selects
from its available entries when creating an exercise. The page shows hidden
entries too.

### What each field means

| Field | Meaning |
| --- | --- |
| **Nome** (Name) | display name, such as `Python 3.13` |
| **Extensão** (Extension) | lower-case extension without the dot, such as `py` or `omp.cpp` |
| **Compilado** / **Interpretado** | whether the browser editor offers the type as source code |
| **Disponível** / **Oculto** | whether the type appears in the catalog and is accepted by submission validation |

The extension is normalised to lower case and stored without the leading dot. It
may contain letters, digits, dots, `+`, `#`, `_` and `-`; compound extensions are
supported. The maximum length is 24 characters. Two entries cannot share an
extension.

### The compile label is about the editor

**Compilado** marks a type the in-browser editor can offer. **Interpretado**
removes it from the editor language list; students can still upload it. This flag
does not choose the judge's compile step. The judge maps the submitted extension
to a language with its own compile flag and runner image.

A wrong value here does not change how code executes, but it can hide a language
from the editor. See [Languages & images](../reference/languages.md) for the
judge's mapping.

### Available and hidden

A hidden type is removed from the professor-facing picker. Submission validation
also reads only available entries, so hiding a type stops it from matching an
exercise that still lists it. Keep a type available while an exercise uses it.

::: danger Hiding the only allowed type widens acceptance
If an exercise lists exactly one type and you hide it, submission validation finds
no allowed extension and accepts any file. That is rarely what you want. Update
the exercise to use an available type before hiding the old one.
:::

Hiding is reversible. Use it instead of deleting a type that is still referenced.

### Delete a type

Deleting is refused while any exercise references the type. The page reports
this and asks you to hide it instead. A type no exercise uses can be deleted.

### Changes are immediate

Creating, editing or hiding an entry drops the public catalog cache. Professors
see the change on their next page load, without a redeploy.

### The catalog is not a language

Registering an extension does not make the judge able to run it. Execution needs:

1. an entry in the judge's language mapping;
2. a runner image for that language;
3. the extension registered in this catalog.

If the judge does not recognise the extension, it falls back to the generic
runner path, which may work for a small script but is not a supported language.
Adding a language is a release change, not an admin action. The steps are in
[Languages & images](../reference/languages.md#adding-a-language).

## Mail and notifications

Mail settings are environment variables on the backend, not admin settings. The
platform sends:

- account verification links;
- password recovery links;
- deadline reminders;
- class emails written by professors.

### Verify delivery

The most reliable check is a real flow:

1. Sign up a fresh address and open the verification mail.
2. Request a password reset for that account.
3. Ask a professor to send a class email to a two-person test class.

If no mail arrives, check the backend log for SMTP errors and confirm
`RUNCODES_SMTP_HOST`, `RUNCODES_SMTP_PORT`, `RUNCODES_SMTP_FROM` and any relay
credentials. The [configuration reference](../reference/configuration.md#mail)
lists every variable.

In a local Compose stack, mail is caught by smtp4dev at
<http://localhost:8081> instead of being delivered. That is expected.

### Link origin

`RUNCODES_PUBLIC_URL` is the browser-facing origin used in verification and
recovery links. It must be the URL people type in the browser, not an internal
service name. A wrong value produces links that point at `localhost` or a host
that does not resolve.

Changing it requires a backend restart. After a domain or port change, update it
and test a verification link.

### Deadline reminders

The backend sends reminders to students who have not submitted when an exercise
closes in 7 days and again in 3 days. The notifier runs inside the API process.
It can be disabled, and its interval changed, with:

```bash
RUNCODES_DEADLINE_REMINDERS=true
RUNCODES_DEADLINE_REMINDER_INTERVAL=30m
```

The interval is a Go duration and must be at least 1 minute. The notifier stores
one row per exercise, student and window, so a restart or a second API replica
does not send the same reminder twice.

### Delivery limits

The credential and mail routes are rate limited separately from the rest of the
API:

- signup and login: 20 requests per minute per client IP;
- verification, resend, forgot and reset: 10 requests per minute per client IP.

A classroom behind one NAT address can reach these limits. They are deliberate,
but they can make a whole lab appear unable to sign up at once. Raise them only
after measuring the traffic; the limits are compiled into the backend.

## Public URL and domain

| Variable | Where it runs | Meaning |
| --- | --- | --- |
| `RUNCODES_PUBLIC_URL` | backend | origin in mailed links |
| `RUNCODES_DOMAIN` | Caddy | site address; a hostname enables HTTPS |
| `FRONTEND_ORIGIN` | backend | allowed CORS origin for the Vite dev server |

In the Compose and Ansible deployments the SPA and API are same-origin through
Caddy, so `FRONTEND_ORIGIN` is only relevant to local development. If you change
the public hostname, update Caddy, DNS, TLS and `RUNCODES_PUBLIC_URL` together.

## What the panel does not configure

| Setting | Where it lives |
| --- | --- |
| SMTP relay | backend environment |
| Public URL and domain | backend and Caddy environment |
| JWT secret, judge token, database and S3 credentials | deployment secrets |
| Backup schedule | host tooling |
| Retention of traces, logs and metrics | observability configuration |
| Runner images and language mapping | release artifacts and judge code |
| Branding, logo and site language | frontend build |

Changing any of these requires a deployment change, not an admin-panel action.

## Common tasks

| Task | Steps |
| --- | --- |
| Update the support address | **Configurações → Email de contato → Salvar** |
| Hide the login disclaimer | **Configurações**, clear the disclaimer, save |
| Offer a new extension | **Tipos de Arquivo → Novo tipo**, set name and extension |
| Hide a language from new exercises | **Tipos de Arquivo**, select **Ocultar** |
| Fix a wrong display name | **Tipos de Arquivo → Editar** |
| Remove an unused extension | **Tipos de Arquivo → Excluir** (only when unused) |
| Check verification mail | sign up a test account and inspect the backend log |
| Change the link domain | update `RUNCODES_PUBLIC_URL`, restart the backend, test a link |
| Disable deadline reminders | set `RUNCODES_DEADLINE_REMINDERS=false`, restart the backend |

## Related pages

- [Administrator guide](./index.md)
- [Platform operations](./operations.md)
- [Configuration reference](../reference/configuration.md)
- [Languages & images](../reference/languages.md)
- [API: administration](../api/admin.md)
- [Documentation site](../documentation.md)
