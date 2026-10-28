---
title: Workflows at a glance
description: A one-page map of the student, professor and administrator workflows, with links to the detailed guides.
---

# Workflows at a glance

RunCodes has three browser workflows: students solve exercises, professors run
classes and administrators run the platform. This page maps them in one place.
Each role has a detailed guide:

| Role | Detailed guide | What the role does |
| --- | --- | --- |
| Student | [Student guide](./student-guide.md) | join a class, submit code, read verdicts, download work |
| Professor | [Professor guide](./professor-guide.md) | create a class, author exercises, grade, reuse material |
| Administrator | [Administrator guide](../administration/index.md) | manage accounts, classes, settings, file types and operations |

The interface is in Brazilian Portuguese. The detailed guides use the on-screen
labels with an English translation on first use.

## Student workflow

1. Sign up and open the verification link.
2. Join a class with the enrollment code.
3. Open an exercise from the dashboard, **Exercícios** or the class page.
4. Write in the editor or upload a file.
5. Submit and follow the per-case verdicts.
6. Resubmit until the deadline; the best score is kept.
7. Download your source from the history or your generated output files from the
   results panel.

Read the [Student guide](./student-guide.md) for the full flow, including drafts,
deadlines, visibility rules and troubleshooting.

## Professor workflow

1. Ask an administrator to promote the account to `professor`.
2. Create a class and share its six-character enrollment code.
3. Add co-professors and monitors by email.
4. Create exercises with dates, instructions, allowed file types and materials.
5. Add test cases, limits and visibility flags.
6. Test the exercise with a real submission.
7. Publish it and monitor the class gradebook.
8. Export the class at the end of the term.

Read the [Professor guide](./professor-guide.md) for authoring, batch test cases,
class tools, gradebook details and recompilation.

## Administrator workflow

1. Bootstrap the seeded admin and change its password.
2. Create a second administrator and promote professors.
3. Set the contact email and login disclaimer.
4. Review the file type catalog against the runner images.
5. Verify mail delivery and judge readiness.
6. Configure backups, telemetry and alerting.
7. Manage users, classes and ownership changes during the term.
8. Upgrade outside submission windows and keep the restore procedure tested.

Read the [Administrator guide](../administration/index.md) for the panel, and
[Platform operations](../administration/operations.md) for day-2 work.

## Rules that cross roles

- **A class is an offering.** The UI says **turma**; the API and database say
  offering.
- **Enrollment is a code, not an invitation.** Students join with the class code;
  monitors and co-professors are added by email.
- **Deadlines apply to students.** Owners and co-professors can submit after the
  deadline to test an exercise.
- **The best score counts.** The per-exercise grade is the best score among the
  student's submissions; the class grade averages those best scores and counts a
  missing exercise as zero.
- **Removed exercises are excluded from the gradebook.** Deleting a class or an
  account cascades to submissions and is irreversible.
- **The judge owns execution.** The backend owns persistence. A judge outage
  stops new runs but not sign-in, classes or browsing.

The grading rules are in [How grading works](./grading.md). The role model is in
[Classes, roles & access](./classes-and-roles.md).

## A worked example

1. An administrator bootstraps `admin@admin.com` and promotes
   `grace@example.edu` to professor.
2. Grace creates "CS101 — Introduction to Programming" and shares the code.
3. Grace creates "Hello, world", allows `.c` and adds one test case with input
   `Ada` and expected output `Hello, Ada!`.
4. Ada signs up, verifies her email, joins with the code and opens the exercise.
5. Ada writes a C program and submits it. The judge compiles and runs it, and
   the page shows the case verdict and score.
6. Ada resubmits until the deadline. Grace opens **Notas** and downloads the
   gradebook CSV.

## Related pages

- [Student guide](./student-guide.md)
- [Professor guide](./professor-guide.md)
- [Administrator guide](../administration/index.md)
- [Submissions & live judging](./submissions.md)
- [Classes, roles & access](./classes-and-roles.md)
- [Accounts & settings](./accounts-and-settings.md)
