---
title: Classes & exercises API
description: Offerings, enrollment, members, exercises, test cases, compilation files and attached files.
---

# Classes & exercises API

Paths are relative to `/api/v1`. "Staff" means the class owner, an enrolled
co-professor, or the admin panel. See
[Classes, roles & access](../guide/classes-and-roles.md) for the authorization
rules.

## Student and self-service routes

### `GET /user/offerings`

Classes the caller belongs to or owns. Returns an array of:

```json
{
  "offering_id": 7,
  "name": "CS101 - Introduction to Programming (2026/1)",
  "description": "…",
  "end_date": "2026-12-15T23:59:59-03:00",
  "role": "student",
  "owner_name": "Grace Hopper",
  "is_owner": false
}
```

### `GET /user/offerings/{id}`

One class with the caller's role and per-exercise progress. Reachable by every
participant and by the owner. Unlike the staff route, it never includes the
enrollment code.

```json
{
  "id": 7,
  "name": "CS101 - Introduction to Programming (2026/1)",
  "description": "…",
  "end_date": "2026-12-15T23:59:59-03:00",
  "role": "student",
  "owner_name": "Grace Hopper",
  "is_owner": false,
  "member_count": 42,
  "exercises": [
    {
      "id": 15,
      "offering_id": 7,
      "title": "Hello, world",
      "description": "…",
      "deadline": "2026-03-20T23:59:59-03:00",
      "open_date": "2026-03-01T00:00:00-03:00",
      "submission_count": 3,
      "last_submission_at": "2026-03-10T14:22:01Z",
      "last_status": "completed",
      "best_score": 100
    }
  ]
}
```

### `GET /user/exercises`

Exercises open **right now** in the caller's classes. Returns an array of:

```json
{
  "id": 15,
  "offering_id": 7,
  "offering_name": "CS101",
  "title": "Hello, world",
  "description": "…",
  "deadline": "2026-03-20T23:59:59-03:00",
  "open_date": "2026-03-01T00:00:00-03:00"
}
```

### `POST /offerings/enroll`

```json
{ "enrollment_code": "ABC123" }
```

Returns `201` with the enrollment object. The code is case-insensitive and
joining is idempotent.

| Status | Meaning |
| --- | --- |
| `201` | enrolled (or already enrolled) |
| `404` | unknown code |
| `409` | class not open to enrollment, ended, caller banned, or caller owns the class |

### `DELETE /offerings/{id}/enrollment`

Leaves a class. Returns `204`. A class owner cannot leave their own class
(`409`).

## Staff class routes

These require a `professor` or `admin` account role; ownership is checked by the
service.

### `GET /offerings`

Classes the caller owns or teaches. Returns `OwnedOffering` objects:

```json
{
  "id": 7,
  "name": "CS101",
  "description": "…",
  "end_date": "2026-12-15T23:59:59-03:00",
  "enrollment_code": "ABC123",
  "visible_to_enroll": true,
  "owner_id": 3,
  "is_owner": true,
  "member_count": 42,
  "exercise_count": 8,
  "enrollment_open": true
}
```

### `POST /offerings/create`

```json
{
  "name": "CS101 - Introduction to Programming",
  "description": "…",
  "end_date": "2026-12-15T23:59:59-03:00"
}
```

- `name` is required and at most 100 characters.
- `end_date` is RFC 3339 and cannot be in the past.
- The server generates the enrollment code.
- Returns `201` with the offering.

### `GET /offerings/{id}`

Returns the class including its enrollment code. Owner, co-professor and admin
panel only.

### `PUT /offerings/{id}`

Partial update:

```json
{
  "name": "CS101 (new name)",
  "end_date": "2027-01-15T23:59:59-03:00",
  "description": "updated",
  "visible_to_enroll": false
}
```

Owner only. Returns the updated offering.

### `DELETE /offerings/{id}`

Deletes the class and its dependent rows. Owner only. Returns `204`.

### Members

| Method | Path | Body | Result |
| --- | --- | --- | --- |
| GET | `/offerings/{id}/members` | — | array of members |
| POST | `/offerings/{id}/members` | `{"email":"...","role":"monitor"}` | `201` member |
| PUT | `/offerings/{id}/members/{userId}` | `{"role":"professor","banned":false}` | `200` member |
| DELETE | `/offerings/{id}/members/{userId}` | — | `204` |

```json
{
  "user_id": 12,
  "name": "Alan Turing",
  "email": "alan@example.com",
  "role": "monitor",
  "banned": false,
  "created_at": "2026-03-01T12:00:00Z"
}
```

Rules:

- only the **owner** manages members (the admin panel uses its own routes);
- assignable roles are `monitor` and `professor`; students join with a code;
- assigning `professor` requires the account's platform role to be
  `professor` or `admin` (`422` otherwise);
- the owner cannot be banned or removed;
- banning keeps the role but removes access;
- adding an existing member updates the role and clears the ban.

## Exercises

### `POST /offerings/{offeringId}/exercises`

JSON body:

```json
{
  "title": "Hello, world",
  "description": "Print a greeting.",
  "open_date": "2026-03-01T00:00:00-03:00",
  "deadline": "2026-03-20T23:59:59-03:00",
  "show_before_open_date": false,
  "allowed_file_type_ids": [1, 2]
}
```

- title required, at most 200 characters;
- `open_date` and `deadline` required; deadline cannot precede open date;
- allowed file types come from `GET /allowed-file-types`.

Returns `201` with the exercise.

### `GET /offerings/{offeringId}/exercises`

Lists the class's exercises. Students see only what they may see; staff see all.

### `GET /exercises/{id}`

Returns one exercise:

```json
{
  "id": 15,
  "offering_id": 7,
  "title": "Hello, world",
  "description": "…",
  "deadline": "2026-03-20T23:59:59-03:00",
  "open_date": "2026-03-01T00:00:00-03:00",
  "show_before_open_date": false,
  "removed": false,
  "allowed_file_type_ids": [1, 2],
  "allowed_file_types": [{ "id": 1, "name": "C", "extension": "c", "is_compilable": true, "is_available": true }],
  "created_at": "…",
  "updated_at": "…"
}
```

### `PUT /exercises/{id}`

Partial update of the same fields. Staff only.

### `DELETE /exercises/{id}`

Deletes the exercise and its test cases and files. Staff only. Returns `204`.

### `GET /allowed-file-types`

The extension catalogue. Any signed-in user.

## Test cases

Test cases are created and updated as **multipart form data** (32 MiB cap) so
that file inputs and outputs can be uploaded together.

| Field | Type | Meaning |
| --- | --- | --- |
| `input_type` | `text` / `file` | how the input is stored |
| `input` | string | inline input when `input_type=text` |
| `input_file` | file | input when `input_type=file` |
| `expected_output_type` | `text` / `file` | how the expected output is stored |
| `expected_output` | string | inline expected output |
| `expected_output_file` | file | expected output when type is `file` |
| `show_input` | bool | expose input to students |
| `show_expected_output` | bool | expose expected output |
| `show_user_output` | bool | expose the student's output |
| `cpu_time_limit_seconds` | int | wall/CPU budget for the case |
| `mem_usage_limit_bytes` | int64 | address-space limit |
| `stack_limit_bytes` | int64 | stack limit |
| `file_size_limit_bytes` | int64 | file-size limit |
| `files` | multiple files | extra files placed beside the case |

| Method | Path | Result |
| --- | --- | --- |
| GET | `/exercises/{id}/test-cases` | array of test cases |
| POST | `/exercises/{id}/test-cases` | `201` test case |
| PUT | `/exercises/{id}/test-cases/{caseId}` | `200` updated test case |
| DELETE | `/exercises/{id}/test-cases/{caseId}` | `204` |

```json
{
  "id": 91,
  "exercise_id": 15,
  "input": "42\n",
  "input_type": "text",
  "show_input": true,
  "expected_output": "42\n",
  "expected_output_type": "text",
  "show_expected_output": true,
  "show_user_output": true,
  "cpu_time_limit_seconds": 2,
  "mem_usage_limit_bytes": 268435456,
  "stack_limit_bytes": 8388608,
  "file_size_limit_bytes": 5242880,
  "files": []
}
```

Example with curl:

```bash
curl -b cookies.txt -X POST \
  http://localhost:8080/api/v1/exercises/15/test-cases \
  -F input_type=text \
  -F input='42
' \
  -F expected_output_type=text \
  -F expected_output='42
' \
  -F show_input=true \
  -F show_expected_output=true \
  -F show_user_output=true \
  -F cpu_time_limit_seconds=2 \
  -F mem_usage_limit_bytes=268435456 \
  -F stack_limit_bytes=8388608 \
  -F file_size_limit_bytes=5242880
```

Upload limits are deliberate: a test case can carry at most 32 MiB of multipart
data.

## Compilation and attached files

**Compilation files** are placed in the workspace before compilation (for
example a Makefile or a headers package). **Attached files** are visible to
students as exercise material. Both use `multipart/form-data` with a `file`
field.

| Method | Path | Result |
| --- | --- | --- |
| GET | `/exercises/{id}/compilation-files` | array of file metadata |
| POST | `/exercises/{id}/compilation-files` | `201` metadata |
| GET | `/exercises/{id}/compilation-files/{fileId}` | file download |
| DELETE | `/exercises/{id}/compilation-files/{fileId}` | `204` |
| GET | `/exercises/{id}/attached-files` | array of file metadata |
| POST | `/exercises/{id}/attached-files` | `201` metadata |
| GET | `/exercises/{id}/attached-files/{fileId}` | file download |
| DELETE | `/exercises/{id}/attached-files/{fileId}` | `204` |

```bash
# Upload a compilation file
curl -b cookies.txt -X POST \
  http://localhost:8080/api/v1/exercises/15/compilation-files \
  -F file=@Makefile

# Download it
curl -b cookies.txt -OJ \
  http://localhost:8080/api/v1/exercises/15/compilation-files/3
```

File metadata:

```json
{
  "id": 3,
  "exercise_id": 15,
  "path": "compilationfiles/15/Makefile",
  "filename": "Makefile"
}
```

## Error behaviour

| Status | Meaning |
| --- | --- |
| `400` | malformed JSON/multipart, invalid ids, validation failure |
| `403` | not the owner, not enrolled, or not staff of the class |
| `404` | class, exercise, test case or file not found; also used when a resource is invisible to the caller |
| `409` | class ended, already a member, owner cannot be removed, etc. |
| `413` | upload exceeds the 32 MiB authoring cap |
| `422` | a well-formed request the account cannot satisfy (for example assigning co-professor to a student) |
| `500` | server error |
