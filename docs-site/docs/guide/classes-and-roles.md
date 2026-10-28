---
title: Classes, roles & access
description: Offerings, enrollment roles, the access model and exercise rules.
---

# Classes, roles & access

A **class** is called an *offering* in the database and API. It groups students,
exercises and staff. Understanding the role model is the key to understanding
what a user can see and do. The task-level guides are the
[Student guide](./student-guide.md), [Professor guide](./professor-guide.md) and
[Administrator guide](../administration/index.md).

## Account roles

`users.role` is a platform-wide role:

| Role | Created how | Purpose |
| --- | --- | --- |
| `student` | signup (default) | enroll in classes, submit solutions |
| `professor` | an admin changes the account role | own classes, author exercises |
| `admin` | seeded account or an admin change | manage users, any class, settings |
| `dev` | legacy accounts only | retained for compatibility |

Only `professor` and `admin` accounts can be assigned to teach a class. A
student cannot be promoted to co-professor through the member endpoints.

## Enrollment roles

`enrollments.role` is per class:

| Role | How assigned | Access |
| --- | --- | --- |
| `student` | joins with the class enrollment code | reads the class, visible exercises, submits |
| `monitor` | owner adds the member | reads class and exercises; submits |
| `professor` | owner adds the member | authors exercises, test cases and files |
| owner (`offerings.owner_id`) | creator of the class | everything, including members and deletion |

An enrollment also has a `banned` flag. A banned member keeps their role but
loses the access that comes with it. The owner cannot be banned or removed.

## Permission matrix

| Action | Owner | Co-professor | Monitor | Student | Admin panel |
| --- | :-: | :-: | :-: | :-: | :-: |
| View class metadata | ✓ | ✓ | ✓ | ✓ | ✓ |
| View exercises | ✓ | ✓ | ✓ | ✓ (open/visible) | — |
| Submit solutions | ✓ | ✓ | ✓ | ✓ | — |
| Submit after deadline | ✓ | ✓ | ✓ | — | — |
| Create/edit exercises | ✓ | ✓ | — | — | — |
| Manage test cases & files | ✓ | ✓ | — | — | — |
| View the gradebook | ✓ | ✓ | — | — | — |
| Export the class, recompile, email | ✓ | ✓ | — | — | — |
| List/add/change members | ✓ | — | — | — | list only |
| Edit/delete class | ✓ | — | — | — | ✓ |
| Rotate the enrollment code | ✓ | — | — | — | — |
| Transfer ownership | — | — | — | — | ✓ |
| Manage file types & settings | — | — | — | — | ✓ |
| Preview as student or professor | — | — | — | — | ✓ |

"Admin panel" means the `/api/v1/admin/*` routes: they act on classes the admin
does not own. The admin's platform role does not automatically give them the
owner's authoring routes; the panel uses its own endpoints. An admin who owns a
class or teaches it as a co-professor gets the teaching routes for that class.

## Grade visibility

- A student sees their own best score and latest status on each exercise, and
  their own submission history. They do not see other students' scores.
- The class owner and co-professors see the full gradebook and its statistics.
- Monitors and students cannot read the gradebook.
- The admin panel does not expose a gradebook. An administrator who needs to read
  one must own the class or be added as a co-professor.

The grade is the best score among the student's submissions. The class grade is
the mean of those best scores over every exercise, counting a missing exercise as
zero. See [How grading works](./grading.md).

## Enrollment

- Students join with a six-character **enrollment code**, case-insensitively.
  An account is limited to five attempts per minute as defense in depth. The
  migration rotates legacy four-character codes; a short code is rejected even
  if it was not rotated. The owner can generate a new code from the class page,
  which invalidates the previous one immediately. Joining is idempotent: entering
  the code twice leaves one enrollment.
- Enrollment is refused when the class is not visible to enroll, has ended, or
  the caller is banned.
- A class owner cannot join their own class with the code; they already have
  access.
- Leaving a class deletes the student's enrollment. The owner cannot leave their
  own class.
- Monitors and co-professors are added by email from the owner's member list.
  Adding an existing member updates the role and clears the ban.

## Deadlines and visibility

Each exercise has an `open_date` and a `deadline`.

- Students see exercises between `open_date` and (optionally) before it, only if
  the exercise is set to show before opening.
- After the deadline, students can no longer submit. Staff can, because the
  deadline is a rule for the students being graded and staff need to try an
  exercise before handing it out.
- The class has its own end date. It closes enrollment; it does not change the
  exercise deadlines. Existing students keep access after it.
- The backend can email a student who has not submitted when an exercise closes
  in 7 days and again in 3 days. Administrators can disable the reminders.
- `removed` hides an exercise from students without deleting its history;
  `ghost` is retained from the legacy model.

The `GET /api/v1/user/exercises` endpoint returns up to 50 exercises currently
open in the caller's classes, which is what the home dashboard uses.

## Exercise authoring

An exercise belongs to one offering and has:

- a title, description, open date and deadline;
- allowed file types (the submission extension must be one of them);
- optional **attached files** (statements, datasets) visible to students;
- optional **compilation files** placed in the workspace before compilation;
- one or more **test cases**, each with:
  - input (`text` or `file`) and expected output (`text` or `file`);
  - visibility flags for input, expected output and user output;
  - CPU time, memory, stack and file-size limits.

The limits are passed to the runner harness as `t_<case_id>`,
`ms_<case_id>`, `fs_<case_id>` and `stack_<case_id>` in `container.config`. The
judge's cgroup caps are a separate, deployment-wide backstop. See
[Judge](../services/judge.md#limits).

## Admin panel

Admins can work across all classes and users:

- search accounts, change name/email/institutional id/role, confirm or unconfirm,
  delete;
- list and edit any class, transfer ownership, delete it;
- list the members of any class;
- maintain the file type catalog;
- edit the platform contact email and disclaimer shown on the login page;
- preview the platform as a student or professor.

The panel does not author exercises, manage membership, read a gradebook or set
another user's password. Those actions stay with the class owner, co-professors
and the user's own recovery flow. See
[User & role administration](../administration/users.md) and
[Class & ownership administration](../administration/classes.md).

An admin cannot change their own role, delete their own account, unconfirm
themselves, or change their own email from the panel. These safeguards prevent an
administrator from accidentally locking themselves out.

## Platform settings

The contact email and disclaimer live in the `platform_settings` key/value
table, not in the frontend build. The login page reads them from
`GET /api/v1/settings/public`; the build-time `VITE_CONTACT_EMAIL` and
`VITE_CONTACT_DISCLAIMER_HTML` values are only fallbacks for the moment before
that request answers.

The disclaimer is HTML, sanitized with DOMPurify in the browser before it is
rendered. An empty disclaimer is valid and hides the paragraph.
