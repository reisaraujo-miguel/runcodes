---
title: Users & accounts API
description: Endpoints for signup, login, verification, recovery, sessions, profile and passwords.
---

# Users & accounts API

All paths are relative to `/api/v1`. See [API overview](./index.md) for
authentication and error conventions.

## Public routes

### `POST /user/signup`

Creates an unconfirmed account and sends a verification email.

```json
{
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "password": "S3cure!pass",
  "password_confirmation": "S3cure!pass"
}
```

| Status | Meaning |
| --- | --- |
| `201` | account created, verification mail attempted |
| `400` | invalid name/email/password, or passwords do not match |
| `409` | email already in use |
| `500` | database error |

The account exists even if mail delivery fails; this is intentional so a retry
does not report the email as taken. The client should offer "resend
verification".

### `POST /user/login`

```json
{ "email": "ada@example.com", "password": "S3cure!pass" }
```

| Status | Meaning |
| --- | --- |
| `200` | sets the `jwt` cookie; empty response body |
| `400` | malformed request or invalid email format |
| `401` | invalid credentials |
| `403` | account exists but email is not verified |
| `500` | internal error |

### `POST /user/verify-email`

```json
{ "token": "<token from the link>" }
```

Consumes the token and marks the account confirmed. Returns `204`. The same
endpoint completes a pending email change: the token issued to the new address
swaps `pending_email` into `email`, clears the pending value, confirms the
account and bumps `token_version` (signing out existing sessions). An unknown,
consumed or expired token returns `400` with one message, so the link's state is
not disclosed; a pending address taken by another account returns `409`. Rate
limited to 10 requests/minute per IP.

### `POST /user/resend-verification`

```json
{ "email": "ada@example.com" }
```

Always returns `202 Accepted`, whether or not the address belongs to an account.
A resend invalidates the previous link. Rate limited to 10 requests/minute per IP.

### `POST /user/forgot-password`

```json
{ "email": "ada@example.com" }
```

Sends a recovery link to the address if it belongs to an account. Requesting a
new link invalidates the previous one.

| Status | Meaning |
| --- | --- |
| `202` | request accepted; the same answer for every address |
| `400` | malformed request or invalid email format |
| `429` | rate limited |

### `POST /user/reset-password`

```json
{
  "token": "<token from the recovery link>",
  "password": "N3w!pass",
  "password_confirmation": "N3w!pass"
}
```

Consumes the token and stores the new password. The token expires after 1 hour
and is single use.

| Status | Meaning |
| --- | --- |
| `204` | password changed |
| `400` | invalid, consumed or expired token; passwords do not match; the new password fails the policy |
| `429` | rate limited |

An unknown, consumed or expired token gets one message, so the link's state is
not disclosed. Resetting a password does not confirm an email address: an
unconfirmed account still has to use the verification flow before it can log in.

### `GET /settings/public`

Returns the contact settings shown on the login page. No authentication.

```json
{
  "contact_email": "contact@example.com",
  "contact_disclaimer_html": "<p>...</p>"
}
```

## Session routes

### `GET /auth`

Returns the current user and session expiry (Unix seconds). Requires a valid
session.

```json
{
  "id": 42,
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "role": "professor",
  "expires_at": 1790000000
}
```

| Status | Meaning |
| --- | --- |
| `200` | current session |
| `401` | missing, invalid or expired session |
| `403` | account is no longer confirmed |

### `POST /auth/refresh`

Extends the session by 30 minutes and sets a fresh cookie. The user is re-read
from the database, so a role change or deleted account takes effect here rather
than trusting stale claims. Returns the same shape as `GET /auth`.

### `POST /user/logout`

Clears the `jwt` cookie and returns `204`. Available even to an unconfirmed
account, so the browser can always drop a dead session. Logout itself does not
bump the account's session version, so a copied token remains valid until it
expires; password changes, email changes and admin role/confirmation changes do
invalidate every issued token.

## Profile routes

### `GET /user/profile`

```json
{
  "id": 42,
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "org_id": "1234567",
  "role": "professor",
  "confirmed": true,
  "code_editor_enabled": true,
  "created_at": "2026-01-10T12:00:00Z",
  "pending_email": ""
}
```

### `PUT /user/profile`

Partial update. All fields are optional:

```json
{
  "name": "Ada L.",
  "email": "ada@example.org",
  "org_id": "7654321",
  "code_editor_enabled": false,
  "current_password": "S3cure!pass"
}
```

`current_password` is required only when `email` changes. An email change is
two-step: the new address is stored as `pending_email`, a verification link is
sent to it, and the advertised `email` only changes after that link is used. The
current address stays active until then. `pending_email` in the response is the
address awaiting confirmation.

| Status | Meaning |
| --- | --- |
| `200` | updated profile |
| `400` | invalid input or `current_password` missing for an email change |
| `401` | `current_password` is incorrect |
| `409` | email or organization id already in use |

Role and confirmation are not editable here; an admin changes them through
[Administration](./admin.md).

### `PUT /user/password`

```json
{
  "current_password": "S3cure!pass",
  "new_password": "N3w!pass",
  "new_password_confirmation": "N3w!pass"
}
```

| Status | Meaning |
| --- | --- |
| `204` | password changed |
| `400` | passwords do not match or the new password fails the policy |
| `401` | current password is incorrect |

The password policy is: at least 8 characters, with an uppercase letter, a
lowercase letter, a digit and a special character. The session cookie is left in
place; the client should re-authenticate as a matter of practice.

## Complete curl example

```bash
BASE=http://localhost:8080/api/v1

# Sign up
curl -s -X POST "$BASE/user/signup" -H 'Content-Type: application/json' \
  -d '{"name":"Ada","email":"ada@example.com","password":"S3cure!pass","password_confirmation":"S3cure!pass"}'

# Verify (token comes from smtp4dev at http://localhost:8081)
curl -s -X POST "$BASE/user/verify-email" -H 'Content-Type: application/json' \
  -d '{"token":"<token>"}'

# Request a password recovery link (also caught by smtp4dev)
curl -s -X POST "$BASE/user/forgot-password" -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com"}'

# Reset the password with the token from that link
curl -s -X POST "$BASE/user/reset-password" -H 'Content-Type: application/json' \
  -d '{"token":"<token>","password":"N3w!pass","password_confirmation":"N3w!pass"}'

# Log in with the new password and keep the cookie
curl -s -c cookies.txt -X POST "$BASE/user/login" -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"N3w!pass"}'

# Read the profile with the cookie
curl -s -b cookies.txt "$BASE/user/profile"

# Or extract the JWT for a bearer-style call
TOKEN=$(awk '$6 == "jwt" { print $7 }' cookies.txt)
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/auth"
```
