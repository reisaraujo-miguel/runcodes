---
title: Classes & exercises API
description: Offerings, enrollment, members, exercises, test cases, compilation files and attached files.
---

# Classes & exercises API

Paths are relative to `/api/v1`. "Staff" means the class owner or an enrolled
co-professor. The admin panel uses its own `/admin/*` routes and does not grant
these authoring routes to an unrelated administrator. See
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

Exercises open **right now** in the caller's classes, ordered by deadline. The
list is capped at 50 entries. Returns an array of:

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
joining is idempotent. Enrollment codes are six characters. The migration
rotates surviving legacy four-character codes; a short code is rejected even if
it was not migrated. A class owner can generate a new code from the class page
or with the rotation endpoint below.

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
- The server generates a six-character enrollment code.
- Returns `201` with the offering.

### `GET /offerings/{id}`

Returns the class including its enrollment code. Owner and co-professors only.
The admin panel uses its own route and does not grant this one to an unrelated
administrator.

### `POST /offerings/{id}/enrollment-code/rotate`

Owner only. Generates a new six-character enrollment code and returns the
offering. The previous code stops working immediately, so students who have not
enrolled yet need the new one. Rotating a class with no code creates one.

| Status | Meaning |
| --- | --- |
| `200` | code rotated; body is the updated offering |
| `403` | caller is not the class owner |
| `404` | class not found |

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

### `POST /offerings/import`

Creates a new class from an archive produced by `GET /offerings/{id}/export`.
Any professor or administrator can import; the caller becomes the owner of the
new class.

The body is `multipart/form-data`:

| Field | Type | Meaning |
| --- | --- | --- |
| `archive` | file | the `.zip` exported by the platform, at most 256 MiB |
| `name` | string | optional new class name; the manifest name is the fallback |
| `description` | string | optional new description |
| `end_date` | RFC 3339 | optional new end date; exercise dates shift by the difference |

The archive contains the class configuration, exercises, test cases, compilation
files and materials. It does not contain submissions, grades or members. The new
class gets a new enrollment code.

If `end_date` is omitted and the manifest's end date is already in the past, the
new class gets a six-month period. Every allowed extension in the archive must
exist in the platform catalog, or the import fails with `422` before writing
anything.

Returns `201` with the new offering.

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

## Grades and statistics

Owner and co-professors only. Students and monitors receive `404`, like every
other author-only view. The admin panel has no gradebook route; an administrator
who needs one must own the class or be a co-professor.

### `GET /offerings/{id}/grades`

Returns the grade table and the statistics derived from it:

```json
{
  "offering_id": 7,
  "offering_name": "CS101",
  "exercises": [
    { "id": 15, "title": "Hello, world", "deadline": "2026-03-20T23:59:59-03:00" }
  ],
  "students": [
    {
      "user_id": 12,
      "name": "Ada Lovelace",
      "email": "ada@example.com",
      "scores": [
        {
          "exercise_id": 15,
          "score": 100,
          "submission_count": 3,
          "last_submission_at": "2026-03-18T14:02:11Z"
        }
      ],
      "average": 100,
      "submission_count": 3,
      "last_submission_at": "2026-03-18T14:02:11Z"
    }
  ],
  "summary": {
    "student_count": 1,
    "students_with_submission": 1,
    "students_above_50": 1,
    "global_average": 100,
    "total_submissions": 3
  },
  "exercise_statistics": [
    {
      "exercise_id": 15,
      "title": "Hello, world",
      "deadline": "2026-03-20T23:59:59-03:00",
      "student_count": 1,
      "submitted_students": 1,
      "students_above_50": 1,
      "average": 100,
      "average_submitted": 100,
      "approval_rate": 100,
      "best_score": 100,
      "lowest_score": 100
    }
  ]
}
```

The exercise grade is the best score among the student's submissions.
`score` is `null` when the student has no submission; the class `average` still
counts it as zero. Removed exercises are excluded. Only enrolled, non-banned
students with the `student` role appear.

### `GET /offerings/{id}/grades.csv`

Streams the same table as RFC 4180 CSV with a UTF-8 byte-order mark, so Excel
opens accented names correctly. The file name is
`notas-turma-<offering-id>.csv`.

Columns: `Aluno`, `Email`, one column per exercise, `Média`, `Submissões`,
`Última submissão`. A missing exercise score is an empty cell. Names and titles
that start with a spreadsheet formula character are prefixed with an apostrophe.

## Class tools

Owner and co-professors only. These routes act on the whole class.

### `GET /offerings/{id}/export`

Streams a zip with the class configuration, exercises, test cases, compilation
files and materials. Submissions, members and grades are excluded. The archive
can be imported with `POST /offerings/import`.

### `GET /offerings/{id}/submissions/export`

Streams a zip with the latest submission of every student on every exercise,
grouped under `alunos/<user-id>_<name>/<exercise-id>_<title>/`, plus `notas.csv`
and `LEIA-ME.txt`.

`notas.csv` columns: `Turma`, `Aluno`, `Email`, `Exercício`,
`Nota da última submissão`, `Situação`, `Enviada em`, `Submissão`. The score is
the latest submission's score, not the student's best score. A missing source
object becomes an `.indisponivel.txt` note in its folder instead of failing the
download.

The response is streamed; the frontend saves it as a file. A client that closes
the connection early gets a truncated zip, so keep the connection open.

### `POST /offerings/{id}/submissions/recompile`

Queues a fresh run of the latest submission of every student on every exercise.
Returns `202`:

```json
{ "queued": 42, "failed": 0 }
```

A new commit is created for each source; the previous commit and its score stay
in the history. At most 5000 submissions are queued per request. The judge must
be ready: if it is not, the request returns `503` and nothing is queued.
Wake-up calls run in the background, so the response returns before the runs
finish.

### `POST /offerings/{id}/notify`

Sends one email to every non-banned member of the class, including students,
monitors, co-professors and the owner.

```json
{
  "subject": "Schedule change for exercise 3",
  "message": "The deadline moved to Friday."
}
```

- `subject` is required, at most 200 characters and cannot contain a line break.
- `message` is required and at most 20,000 characters.
- The message is sent as plain text with an HTML version. Attachments are not
  supported.
- Delivery runs in the background. Returns `202` with the number of recipients
  queued:

  ```json
  { "recipients": 41 }
  ```

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

Soft-deletes the exercise: it sets `removed = true` and returns `204`. Students
stop seeing it and it no longer accepts submissions. Its test cases, files and
historical submissions are preserved. There is no endpoint to undo the removal.

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
| `cpu_time_limit_seconds` | int | wall-clock timeout for the case; reported `cpu_time` is measured CPU time |
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

### `POST /exercises/{id}/test-cases/batch`

Creates many cases from one zip. The body is `multipart/form-data`:

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `archive` | file | — | the zip, at most 64 MiB |
| `input_type` | `text` / `file` | `text` | applied to every case |
| `expected_output_type` | `text` / `file` | `text` | applied to every case |
| `show_input` | bool | `true` | applied to every case |
| `show_expected_output` | bool | `true` | applied to every case |
| `show_user_output` | bool | `true` | applied to every case |
| `cpu_time_limit_seconds` | int | `0` | 0 = platform default |
| `mem_usage_limit_bytes` | int64 | `0` | 0 = platform default |
| `stack_limit_bytes` | int64 | `0` | 0 = platform default |
| `file_size_limit_bytes` | int64 | `0` | 0 = platform default |

The archive layout is one folder per case:

```text
caso1/
  entrada.in
  saida.out
  ajuda.txt
caso2/
  entrada.in
  saida.out
  files/notas.in
```

Each folder must contain exactly one `.in` and one `.out` file. Other files are
extra case files. An extra file whose name ends in `.in` or `.out` belongs in a
`files/` subfolder. The whole archive is validated before anything is written.

Limits: 500 cases, 4000 zip entries, 256 MiB expanded and 32 MiB per file.

Returns `201`:

```json
{ "created": 12 }
```

The response also contains the created cases in `test_cases`.

A structural failure returns `422` with every bad folder:

```json
{
  "error_msg": "invalid test case: 2 folder(s) do not follow the expected format",
  "invalid_folders": [
    {
      "folder": "caso2",
      "reason": "falta um arquivo com extensão .out (saída esperada do caso)"
    }
  ]
}
```

### `GET /exercises/{id}/test-cases/export`

Streams a zip in the layout the batch importer accepts, with
`caso_001/input.in`, `caso_001/output.out` and the extra files. Use it to move
cases between exercises. The export carries the input and output content, not the
per-case limits or visibility configuration; the import form supplies those.

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
| `413` | upload exceeds the endpoint's cap (32 MiB test case, 64 MiB test-case batch, 256 MiB class import) |
| `422` | a well-formed request the account cannot satisfy (for example assigning co-professor to a student) |
| `500` | server error |
