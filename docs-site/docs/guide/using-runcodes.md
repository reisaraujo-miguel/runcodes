---
title: Using RunCodes
description: End-to-end workflows for students, professors and administrators.
---

# Using RunCodes

This is the product guide: what each role does in the web application, in the
order they usually do it.

## Students

### 1. Create and verify an account

Sign up at `/login?mode=signup`. The account starts **unconfirmed**; open the
link in the verification email before trying to sign in (the login page explains
this and can resend the link). In a local Compose stack, the mail is caught by
smtp4dev at <http://localhost:8081>.

### 2. Join a class

A class is called an **offering**. Open **Classes**, enter the enrollment code
from your professor, and join. The class then appears on your dashboard.

### 3. Find an exercise

The home dashboard lists exercises that are **open right now** across your
classes. The exercises page shows all exercises in your classes, with open and
deadline times. An exercise may be visible before its open date depending on the
professor's setting.

### 4. Solve and submit

Open the exercise, then the submission workspace:

- write in the in-browser editor or upload a file;
- only the file types the exercise allows are accepted;
- the editor keeps a local draft and can restore your previous submission;
- the source file limit is 10 MiB.

Submitting creates a commit immediately. The page follows the run live:

| What you see | Meaning |
| --- | --- |
| `queued` | waiting for a judge worker |
| `compiling` | the compiler is running |
| `running` | test cases are executing |
| case verdicts | `correct`, wrong output, or killed |
| `completed` | every case passed |
| `uncompleted` | the run finished with failures |
| `compilation_error` | the compiler rejected the code |
| `timeout` / `server_error` | infrastructure or budget failure |

You can close the page and come back: the persisted state is restored from a
snapshot. The submission history shows your last 50 submissions for the exercise,
with score and status.

### 5. Download what you produced

If a run wrote output files, the result page offers an archive. The original
source of a past submission can also be downloaded and restored into the editor.

### 6. Manage your profile

**Profile** lets you change your name, email and organization id, and turn the
in-browser code editor on or off. Changing your email does not unconfirm the
account, but the new address must be unique.

::: tip Deadline rules
Students cannot submit after the deadline. Professors, co-professors and the
class owner can, because they need to try an exercise before assigning it.
:::

## Professors

A **professor** account can own classes. If your account is a student, ask an
administrator to change your role.

### 1. Create a class

**Professor → Classes → New class**. Set a name, an end date and an optional
description. The server generates a six-character enrollment code. Share that
code with students; add co-professors and monitors from the class page by email.

- **Co-professor** (`professor` enrollment) can author exercises, test cases and
  files, but cannot manage members.
- **Monitor** can read the class and exercises and submit, but cannot author.
- **Students** join with the code and cannot be added by email.

### 2. Create an exercise

Open the class and create an exercise with:

- a title and description;
- an open date and deadline;
- the allowed file types (the judge maps the extension to a language image).

### 3. Add test cases

Each test case has an input, an expected output and limits. Inputs and expected
outputs can be **text** or **files**; extra files can be attached to a case. The
limits are:

- CPU time in seconds;
- memory (address space) in bytes;
- stack size in bytes;
- file size in bytes.

Visibility flags decide whether students see the input, the expected output and
their own output.

### 4. Add supporting files

- **Compilation files** are placed in the workspace before compilation (headers,
  Makefiles, libraries).
- **Attached files** are student-visible material (statements, datasets).

### 5. Test the exercise yourself

Staff can submit to their own exercises, including after the deadline. Do that
before releasing the exercise: it validates the test cases, the allowed file
types and the resource limits against a real run.

### 6. Monitor the class

The class page shows exercises and member counts. Student submissions are graded
independently; the class owner can list members, change roles, ban or remove
members, and transfer nothing (only an admin can transfer ownership).

## Administrators

The **Admin** area is available to accounts with the `admin` role.

| Page | What it does |
| --- | --- |
| Dashboard | platform overview |
| Users | search accounts, change name/email/organization/role, confirm/unconfirm, delete |
| Courses | list every class, edit it, transfer ownership, delete |
| Settings | contact email and login-page disclaimer |

Typical first steps after a fresh deployment:

1. Bootstrap the seeded admin password (see
   [Getting started](./getting-started.md#_3-bootstrap-the-admin-account)).
2. Sign in and promote the professors who will create classes.
3. Update the contact email/disclaimer in **Settings**.
4. Optionally unconfirm or delete test accounts.

An admin cannot change their own role, delete their own account or unconfirm
themselves, to avoid locking themselves out.

## A worked example

1. Admin: bootstrap `admin@admin.com`, promote `grace@example.edu` to
   professor.
2. Grace: create "CS101 — Introduction to Programming", share the code.
3. Grace: create "Hello, world", allow `.c`, add one test case with input
   `Ada` and expected output `Hello, Ada!`.
4. Ada: signs up, verifies, joins with the code, opens the exercise, writes a C
   program and submits.
5. The judge compiles and runs it, the page shows the case verdict and score,
   and Ada can resubmit until the deadline.
6. Grace: reviews the class and the submissions.

## Related pages

- [Submissions & live judging](./submissions.md)
- [Classes, roles & access](./classes-and-roles.md)
- [Accounts & settings](./accounts-and-settings.md)
- [Frontend application](./frontend.md)
