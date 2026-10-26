---
title: Administration API
description: Admin endpoints for users, classes and platform settings.
---

# Administration API

Every route here requires an account with the `admin` platform role. The admin
panel acts on classes the admin does not own; it uses its own endpoints instead
of the owner routes.

## Users

### `GET /admin/users`

Search and paginate accounts.

| Query | Default | Notes |
| --- | --- | --- |
| `query` | empty | case-insensitive match on name, email or institutional id |
| `limit` | 50 | clamped to 200 |
| `offset` | 0 | negative values become 0 |

```json
[
  {
    "id": 7,
    "name": "Ada Lovelace",
    "email": "ada@example.com",
    "org_id": "1234567",
    "role": "professor",
    "confirmed": true,
    "created_at": "2026-01-10T12:00:00Z"
  }
]
```

### `PUT /admin/users/{id}`

Partial update. All fields optional:

```json
{
  "name": "Ada L.",
  "email": "ada@example.org",
  "org_id": "7654321",
  "role": "admin",
  "confirmed": true
}
```

| Status | Meaning |
| --- | --- |
| `200` | updated user |
| `400` | invalid id/body or invalid role |
| `409` | email or institutional id already in use; or the target is the caller and the request changes their own role/email or unconfirms them |

Valid roles are `student`, `professor`, `admin` and `dev`. Promoting a user to
`admin` grants the entire admin panel; demoting an admin is subject to the
self-management rules below.

Changing the email, role or confirmation bumps the account's `token_version`,
so every existing session is signed out. An email change also clears any pending
email change and marks the account unconfirmed unless the same request explicitly
sets `"confirmed": true`.

### `DELETE /admin/users/{id}`

Deletes the account and everything that hangs off it (foreign keys use
`ON DELETE CASCADE`). Returns `204`. An admin cannot delete their own account
(`409`).

## Classes

### `GET /admin/offerings`

Lists every class on the platform, with the same `query`/`limit`/`offset`
pagination as users.

```json
[
  {
    "id": 7,
    "name": "CS101",
    "description": "…",
    "end_date": "2026-12-15T23:59:59-03:00",
    "enrollment_code": "ABC123",
    "visible_to_enroll": true,
    "owner_id": 3,
    "owner_name": "Grace Hopper",
    "owner_email": "grace@example.com",
    "owner_org_id": "1234567",
    "member_count": 42,
    "exercise_count": 8,
    "created_at": "2026-02-01T12:00:00Z"
  }
]
```

### `PUT /admin/offerings/{id}`

Partial update, including **ownership transfer**. Identify the new owner by
email or by their institutional id (for example, a university registration
number):

```json
{
  "name": "CS101",
  "description": "…",
  "end_date": "2027-01-15T23:59:59-03:00",
  "visible_to_enroll": false,
  "owner_email": "grace@example.com"
}
```

Or, when the admin knows the owner's institutional id instead of the email:

```json
{
  "owner_org_id": "1234567"
}
```

`owner_id` remains accepted for compatibility, but the admin panel does not use
it: the internal database id is not needed to transfer a class. Set at most one
of `owner_email`, `owner_org_id` and `owner_id`. The new owner must exist and
must have the `professor` or `admin` role. Transferring a class does not
re-enroll the previous owner; they lose the owner privileges but keep any
enrollment they had. Returns the updated class.

### `DELETE /admin/offerings/{id}`

Deletes any class and its dependent rows. Returns `204`.

### `GET /admin/offerings/{id}/members`

Lists the members of any class, in the same shape as the owner's member route:

```json
[
  {
    "user_id": 12,
    "name": "Alan Turing",
    "email": "alan@example.com",
    "role": "monitor",
    "banned": false,
    "created_at": "2026-03-01T12:00:00Z"
  }
]
```

The admin panel does not add or remove members; it lists them. Ownership transfer
is the supported way to hand a class to another professor.

## Platform settings

### `GET /admin/settings`

Returns the same two values as the public route, for editing:

```json
{
  "contact_email": "contact@example.com",
  "contact_disclaimer_html": "<p>...</p>"
}
```

### `PUT /admin/settings`

```json
{
  "contact_email": "support@example.com",
  "contact_disclaimer_html": "<p>Need help? Email support.</p>"
}
```

- `contact_email` must be a valid address and is required.
- `contact_disclaimer_html` is optional, at most 4000 characters, and may be
  empty to hide the disclaimer.
- The value is stored as submitted; the frontend sanitizes it with DOMPurify
  before rendering.

Returns the stored settings. The public cache entry is invalidated immediately.

## Safeguards

| Action | Rule |
| --- | --- |
| change own role | rejected (`409`) |
| delete own account | rejected (`409`) |
| unconfirm own account | rejected (`409`) |
| assign `professor` enrollment to a student | rejected (`422`) |
| ban or remove a class owner | rejected (`409`) |
| delete a user or class | cascades to dependent rows; irreversible |

## Related pages

- [Accounts & settings](../guide/accounts-and-settings.md)
- [Classes, roles & access](../guide/classes-and-roles.md)
- [Users & accounts API](./auth.md)
