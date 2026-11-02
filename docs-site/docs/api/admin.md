---
title: Administration API
description: Admin endpoints for users, classes, platform settings and language statistics.
---

# Administration API

Every `/admin/*` route here requires an account with the `admin` platform role.
The admin panel acts on classes the admin does not own; it uses its own endpoints
instead of the owner routes. The preview stop route is the exception: it runs
from a preview session, not from the admin group.

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

## File types

The platform-wide catalog of extensions a professor can attach to an exercise.
These routes include hidden entries; the public `/allowed-file-types` route
returns only available ones.

### `GET /admin/file-types`

Returns every entry, ordered by name:

```json
[
  {
    "id": 3,
    "name": "C",
    "extension": "c",
    "is_compilable": true,
    "is_available": true
  }
]
```

### `POST /admin/file-types`

Creates an entry. Returns `201` with the stored object.

```json
{
  "name": "Python 3.13",
  "extension": "py",
  "is_compilable": false,
  "is_available": true
}
```

- `name` is required, trimmed and at most 100 characters.
- `extension` is required, lower-cased, has the leading dot removed and is at
  most 24 characters. It may contain letters, digits, `.`, `+`, `#`, `_` and `-`;
  `..` is rejected.
- `is_compilable` and `is_available` default to `true` when omitted.

`is_compilable` only controls whether the browser editor offers the type. The
judge has its own language mapping. `is_available` controls both the public
catalog and submission validation.

| Status | Meaning |
| --- | --- |
| `201` | created |
| `400` | invalid name or extension |
| `409` | another entry already uses the extension |

### `PUT /admin/file-types/{id}`

Partial update of the same fields. Returns `200` with the updated entry. A
missing id returns `404`; a duplicate extension returns `409`.

### `DELETE /admin/file-types/{id}`

Deletes an unused entry. Returns `204`. An entry referenced by at least one
exercise returns `409`; hide it with `is_available: false` instead.

## Language statistics

### `GET /admin/language-stats`

Returns one row per catalog entry plus the platform denominators the rates use.
The report is cached for up to one minute. The panel and the fairness method are
described in [Language statistics](../administration/language-statistics.md).
For one class, use
[`GET /offerings/{id}/language-stats`](./classes.md#get-offerings-id-language-stats),
which returns the same shape with class-scoped denominators.

```json
{
  "generated_at": "2026-11-02T12:00:00Z",
  "summary": {
    "exercises": 30,
    "professors": 4,
    "catalog_languages": 23,
    "languages_offered": 9,
    "languages_used": 6,
    "classified_submissions": 285,
    "students": 41,
    "unclassified_submissions": 3
  },
  "languages": [
    {
      "file_type_id": 1,
      "name": "C",
      "extension": "c",
      "is_compilable": true,
      "is_available": true,
      "submissions": 120,
      "submission_share": 42.11,
      "students": 34,
      "exercises_chosen": 12,
      "pick_rate": 31.5,
      "pick_rate_samples": 14,
      "exercises_offered": 18,
      "exercise_offer_rate": 60,
      "offerings_offered": 4,
      "professors_offering": 3,
      "professor_offer_rate": 75,
      "results": 480,
      "test_cases": 40,
      "compared_test_cases": 36,
      "compared_exercises": 8,
      "cpu_time_median_seconds": 0.0423,
      "relative_cpu": 0.87,
      "relative_cpu_p25": 0.72,
      "relative_cpu_p75": 1.04,
      "timings": {
        "queue": {
          "median_seconds": 0.4,
          "p25_seconds": 0.2,
          "p75_seconds": 1.1,
          "samples": 40,
          "coverage": 80
        },
        "workspace": {
          "median_seconds": 0.9,
          "p25_seconds": 0.7,
          "p75_seconds": 1.4,
          "samples": 40,
          "coverage": 80
        },
        "image": {
          "median_seconds": 0.04,
          "p25_seconds": 0.03,
          "p75_seconds": 0.06,
          "samples": 40,
          "warm_samples": 38,
          "cold_samples": 2,
          "coverage": 80
        },
        "startup": {
          "median_seconds": 0.3,
          "p25_seconds": 0.25,
          "p75_seconds": 0.4,
          "samples": 40,
          "coverage": 80
        },
        "compile": {
          "median_seconds": 1.2,
          "p25_seconds": 1.0,
          "p75_seconds": 1.5,
          "cpu_time_median_seconds": 1.0,
          "mem_peak_bytes": 12000000,
          "samples": 36,
          "coverage": 72
        },
        "run": {
          "median_seconds": 0.8,
          "p25_seconds": 0.5,
          "p75_seconds": 1.2,
          "samples": 40,
          "coverage": 80
        },
        "grade": {
          "median_seconds": 0.1,
          "p25_seconds": 0.08,
          "p75_seconds": 0.15,
          "samples": 40,
          "coverage": 80
        }
      },
      "memory": {
        "relative": 0.94,
        "p25": 0.82,
        "p75": 1.08,
        "median_bytes": 12582912,
        "compared_test_cases": 34,
        "compared_exercises": 8,
        "known_results": 460,
        "total_results": 480,
        "coverage": 95.83
      }
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `submissions` | student submissions with a matched type, in non-removed exercises |
| `submission_share` | `submissions` as a percentage of `summary.classified_submissions` |
| `students` | distinct students who chose the language |
| `exercises_chosen` | distinct exercises with at least one submission of the language |
| `pick_rate` | mean, across offered exercises with submissions, of the language's share of those submissions; `null` when none |
| `pick_rate_samples` | exercises behind `pick_rate` |
| `exercises_offered` | non-removed exercises that allow the type |
| `exercise_offer_rate` | `exercises_offered` as a percentage of `summary.exercises` |
| `offerings_offered` | distinct classes with an exercise that allows the type |
| `professors_offering` | distinct exercise authors who allow the type |
| `professor_offer_rate` | `professors_offering` as a percentage of `summary.professors` |
| `results` | executed test-case results for the language |
| `test_cases` | distinct test cases with a result for the language |
| `compared_test_cases` | test cases with at least two languages, behind `relative_cpu` |
| `compared_exercises` | distinct exercises behind `relative_cpu` |
| `cpu_time_median_seconds` | median CPU seconds across the compared test cases |
| `relative_cpu` | median, across compared test cases, of the language's median CPU divided by the median of the languages on that case; `null` without a comparison |
| `relative_cpu_p25`, `relative_cpu_p75` | quartiles of the same ratios |
| `timings.<phase>` | median duration, quartiles, sample count and coverage for one measured phase |
| `timings.compile.cpu_time_median_seconds` | CPU time of the compiler command, from the compile monitor |
| `timings.compile.mem_peak_bytes` | compile phase's median peak memory, when the monitor measured it |
| `timings.image.warm_samples`, `cold_samples` | image runs already local vs pulls; the median excludes cold pulls |
| `memory.relative`, `memory.p25`, `memory.p75` | paired peak-memory index and quartiles against the languages on the same test cases |
| `memory.median_bytes` | median peak bytes across the compared test cases |
| `memory.compared_test_cases`, `compared_exercises` | cases and exercises behind the index |
| `memory.known_results`, `total_results`, `coverage` | results with a known memory figure over every classified result |

A `timings` phase is omitted when the report has no measurements for it. Old
runs predate the phase timings and produce no rows. `timings.compile` does not
apply to interpreted languages. `memory` is omitted when no result has a known
memory figure; only values >= 0 enter the index, so `coverage` is part of the
comparison. The monitor samples the container cgroup every 10 ms; the value
includes runtime and page cache and can miss very short peaks.
`relative_cpu` and `memory.relative` are observational comparisons, not
controlled benchmarks; see the
[methodology](../administration/language-statistics.md#what-the-numbers-can-and-cannot-say).

## Role preview

An administrator can replace the role claim of their own session with `student`
or `professor`. The account, name, email and token version stay the same. The
admin panel and every admin-only route become inaccessible until the preview
ends.

### `POST /admin/preview`

```json
{ "role": "student" }
```

Valid roles are `student` and `professor`. Returns `200` with the session info
and sets a new `jwt` cookie. The response includes `previewing_role`.

| Status | Meaning |
| --- | --- |
| `200` | preview started |
| `400` | invalid role or body |
| `404` | the account no longer exists |
| `422` | the account is no longer an admin and cannot preview |

The preview uses the normal session lifetime (30 minutes, sliding). Refresh keeps
the preview claim; a preview does not silently turn back into an admin session.

### `POST /auth/stop-preview`

Restores the administrator's own role and returns the session info. The route is
outside the admin group because the current role is the previewed role. It
requires a session that carries the preview claim; a regular student or
professor session returns `400`.

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
| transfer a class to a student | rejected (`400`) |
| ban or remove a class owner | rejected (`409`) |
| delete a file type used by an exercise | rejected (`409`); hide it instead |
| preview a role from a non-admin account | rejected (`422`) |
| delete a user or class | cascades to dependent rows; irreversible |

## Related pages

- [Accounts & settings](../guide/accounts-and-settings.md)
- [Classes, roles & access](../guide/classes-and-roles.md)
- [Users & accounts API](./auth.md)
