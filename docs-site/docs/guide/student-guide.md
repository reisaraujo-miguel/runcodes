---
title: Student guide
description: Create an account, join a class, submit solutions, read verdicts and manage your profile.
---

# Student guide

This guide follows a student from the first signup to the last submission. It
covers the whole browser workflow: verifying an account, joining a class,
solving an exercise, reading the judge's verdicts and downloading your work.

The RunCodes interface is in Brazilian Portuguese. This page shows the labels as
they appear on screen and gives the English meaning the first time a label is
used.

## What you need

- A RunCodes account with a confirmed email address.
- The **enrollment code** (código de matrícula) from your professor.
- A supported browser with JavaScript enabled.
- A solution file, or your code written in the in-browser editor.

## Create your account

### Sign up

1. Open the RunCodes address your institution gave you.
2. Select **Cadastre-se** (Sign up) on the login card.
3. Fill in your name, email, password and password confirmation.
4. Select **Criar conta** (Create account).

The password must have at least 8 characters, including an uppercase letter, a
lowercase letter, a digit and a special character.

Signing up does not sign you in. The account starts **unconfirmed**, and the
platform sends a verification link to the address you entered.

### Verify your email

1. Open the verification email.
2. Follow the link. The `/verify-email` page confirms the account.
3. Return to the login page and sign in.

The link expires after 24 hours. If it does not arrive, check the spam folder
and select **Reenviar email de verificação** (Resend verification email). A new
link invalidates the previous one.

If your organization runs a local development stack, verification mail is caught
by smtp4dev instead of being delivered. See
[Getting started](./getting-started.md#_4-create-and-verify-an-account).

### Sign in

1. Enter your email and password.
2. Select **Entrar** (Sign in).

An unconfirmed account is refused with a message asking you to verify the email.
The login card offers the resend action in that case.

A session lasts 30 minutes and the application renews it while the tab is open.
Signing out clears the session cookie in that browser.

### Recover a forgotten password

1. Select **Esqueceu a senha?** (Forgot your password?) on the login page.
2. Enter your account email and submit the form.
3. Open the recovery email and follow the link to `/reset-password`.
4. Choose a new password and confirm it.

The recovery link expires after 1 hour and works once. Requesting another link
invalidates the previous one. Resetting a password does not confirm an
unconfirmed account, so you still need a verification link to sign in.

The form always reports that a message was sent, whether or not the address has
an account. This prevents the platform from confirming which addresses are
registered.

## Join a class

Your professor creates a class (internally, an offering) and shares a
six-character enrollment code.

1. Open **Minhas Turmas** (My classes) in the sidebar, or the **Nova matrícula**
   (New enrollment) card on the dashboard.
2. Enter the code in **Código da turma** (Class code).
3. Select **Entrar** (Join).

The code is case-insensitive. Joining twice leaves one enrollment. After you
join, the class appears on the dashboard and in **Minhas Turmas**.

To leave a class, open **Minhas Turmas** and select **Sair** (Leave). This
deletes your enrollment, including your access to the class's exercises. You
need the code again to return. A class owner cannot leave their own class.

Enrollment can be refused when the code is wrong or rotated, when the class has
ended, when the professor closed enrollment, or when you are banned from the
class. The page shows the reason returned by the server.

## Find your work

### Dashboard

The dashboard shows:

- the number of open exercises returned (up to 50);
- the classes you belong to;
- **Próximas entregas** (Upcoming deadlines), one card per open exercise;
- a form to join another class.

An exercise card shows its class, deadline, remaining time and a **Resolver**
(Solve) button that opens the submission workspace.

### Exercises page

**Exercícios** (Exercises) lists up to 50 exercises open right now across your
classes, grouped by class and ordered by deadline. An exercise that has not
opened yet does not appear here.

### Class page

Open a class from **Minhas Turmas** to see every exercise you may view, including
one the professor chose to show before its opening date. The class page shows:

- your best score and latest status on each exercise;
- a progress summary: exercises delivered, open exercises, graded exercises and
  your average;
- filters: **Todos** (All), **Posso enviar** (I can submit), **Sem submissão**
  (No submission) and **Com nota** (Graded);
- sorting by **Prazo** (Deadline), **Maior nota** (Highest score), **Menor nota**
  (Lowest score) or **Última entrega** (Latest submission).

The **Abertura** (Opens) and **Prazo** (Deadline) times use your browser's time
zone.

## Submit a solution

1. Open an exercise with **Resolver**, **Enviar solução** (Submit solution) or
   **Enviar novamente** (Submit again).
2. Read the statement and any materials in the left panel. Select **Materiais**
   (Materials) to download files the professor attached.
3. Choose **Editor** or **Arquivo** (File) in the toolbar. If the exercise has no
   editable type, the editor control is hidden and only upload is available.
4. Write your solution or upload a file.
5. Select **Enviar solução** (Submit solution).

### Editor mode

The editor offers the languages allowed by the exercise. Select the language,
adjust the file name if needed, and write your code. The extension of the file
name determines the language the judge runs, so keep the extension consistent
with the code.

The editor saves a draft in this browser as you type, scoped to your account and
the exercise. The draft survives a reload, a lost connection or a closed tab.
It is not stored on the server, so it does not follow you to another browser or
device, and clearing the browser's site data removes it.

A draft can be discarded from the draft indicator. Restoring a previous
submission replaces the current draft after a confirmation.

### Upload mode

Upload a file with an extension the exercise allows. The page lists the accepted
extensions; if the exercise has no restriction, it says so and accepts any file.
Compound extensions such as `.omp.cpp` are valid when the professor allows that
type.

A source file can be at most 10 MiB. A zip archive is accepted only when the
exercise allows the zip type; the judge unpacks it under per-entry and total
size limits.

### What happens after you submit

The API creates a submission, called a commit, and returns immediately. The
results panel opens on that submission and follows the run live. You can close
the page and return later: the persisted state is restored.

The platform accepts at most two unfinished submissions at a time per account.
Wait for the current run to finish before submitting again.

The server also refuses a submission when:

| Response | Meaning |
| --- | --- |
| `400` | the file is missing or its type is not allowed by the exercise |
| `403` | you are not enrolled in the exercise's class |
| `404` | the exercise does not exist or is not visible to you |
| `413` | the file is larger than 10 MiB |
| `422` | the deadline has passed or the exercise has no test cases |
| `429` | you already have two unfinished submissions |
| `503` | the judge is unavailable or its queue is full |

## Read the results

The results panel lists every test case in a strip and shows the details of
the selected case. A case chip carries its number and a verdict icon. Select a
case to inspect its input, expected output and, when the professor released it,
your program's output.

### Run statuses

| Label | Meaning |
| --- | --- |
| **Na fila** (Queued) | a judge worker has not claimed the submission yet |
| **Compilando** (Compiling) | the compiler is running |
| **Executando** (Running) | test cases are executing |
| **Concluído** (Completed) | every test case was correct |
| **Não concluído** (Uncompleted) | the run finished with at least one failed case |
| **Erro de compilação** (Compilation error) | the compiler rejected the code |
| **Erro no servidor** (Server error) | an infrastructure or harness failure |
| **Tempo esgotado** (Timed out) | the run exceeded its execution budget |

### Case verdicts

| Label | Meaning |
| --- | --- |
| **Correto** (Correct) | the output matched the expected output |
| **Saída incorreta** (Wrong output) | the program ran but the output did not match |
| **Interrompido** (Interrupted) | the process crashed, was killed by a signal, or hit a resource limit |

For text cases the judge ignores trailing whitespace and a missing final
newline. Differences in blank lines, case or spacing count as **Saída
incorreta**, even when a human would read the output as the same. For file
cases the comparison is byte for byte. See [How grading works](./grading.md).

A **Interrompido** verdict can come from a non-zero exit status, a signal, a
timeout or file-size limit. The case detail shows the CPU time and, when the
environment reports it, memory usage. A missing memory figure is shown as `—`.

### Score and visibility

The score is the percentage of test cases that were exactly correct, from 0 to
100, with two decimals. The result panel also shows how many cases are correct.

The professor decides whether each case releases:

- the input (**Mostrar entrada**);
- the expected output (**Mostrar saída esperada**);
- your program's output (**Mostrar saída do aluno**).

A locked field shows a message instead of the content. File-based inputs and
expected outputs are not shown in the results panel, even when the professor
released them for the case.

### Output files

If your program writes files, the results panel offers **Arquivos gerados**
(Generated files) with a download of the output archive. The archive contains
only the files the program produced. Internal monitor reports are excluded.

## Resubmit, compare and download

Select the history icon in the submission toolbar to open **Submissões
anteriores** (Previous submissions). The dialog lists your last 50 submissions
for the exercise, newest first, with the file name, status and score.

From the history you can:

- **Baixar** (Download) the original source file;
- **Restaurar no editor** (Restore in editor) to load that source into the
  editor and continue from it.

Restoring replaces the current editor draft after a confirmation. Restoration is
unavailable for a file type the editor no longer offers.

The class page shows your **best score** on the exercise, not necessarily the
score of the latest submission. The results panel shows the score of the run
currently open in it, which may be lower than your best. You can resubmit until
the deadline; the best result stays available in the history.

## Deadlines and reminders

Each exercise has an opening date and a deadline.

- You can submit between the opening date and the deadline.
- After the deadline, the server refuses new submissions from students. Staff
  can still submit.
- An exercise may be visible before it opens if the professor enabled
  **Mostrar o exercício antes da data de abertura** (Show before the opening
  date).
- The platform can email you when an exercise closes in 7 days and again in 3
  days if you have not submitted anything. These reminders go to your account
  address and can be disabled by the platform administrator.
- The class itself has an end date. Ask your professor when a class closes.

## Manage your profile

Open your avatar at the bottom of the sidebar and select **Meu perfil** (My
profile). The profile page has three sections.

### Account data

You can change:

- **Nome** (Name);
- **Email**;
- **Identificação institucional** (Institutional ID), optional.

Changing the email requires your current password. The platform sends a
verification link to the new address and keeps the old address active until you
open it. Completing the change signs you out, so sign in again with the new
address.

The role shown on the profile (**Perfil**) is changed by an administrator, not by
you.

### Editor preference

**Usar editor de código na página do exercício** (Use the code editor on the
exercise page) controls whether the submission workspace starts in editor mode
or upload mode. The preference is stored on your account, so it follows you
between devices. You can still switch modes for one submission.

### Password

**Alterar senha** (Change password) requires the current password and a new
password that follows the same policy as signup. Changing the password signs you
out, so sign in again with the new password.

Theme and panel-layout preferences are stored in the browser, not on the
account.

## Troubleshooting

### I cannot sign in

Check the error message. An unconfirmed account needs the verification link;
the login card offers a resend. A wrong password can be recovered with
**Esqueceu a senha?**. If the session expired, sign in again.

### The verification or recovery email did not arrive

Check the spam folder, wait a few minutes, then resend. The address must match
the account. If your institution runs its own deployment and no mail arrives,
contact the platform administrator: the SMTP relay may be misconfigured.

### My file type is rejected

Open the exercise and check the accepted extensions. The extension is the part
after the last dot. The exercise may accept `.c` but not `.cpp`, or the reverse.
Renaming a file without changing its content does not make it valid.

### The deadline has passed

Students cannot submit after the deadline. Contact the professor: only staff can
submit after the deadline, and the professor can extend it by editing the
exercise.

### The submission stays queued

A queued submission waits for a judge worker. If the queue is not moving, the
judge may be unavailable. Wait a minute and reload. If it does not start,
contact the platform administrator with the submission number and the time.

### The page says the judge is unavailable

The API refuses new submissions with `503` while the judge cannot run work. This
is a platform problem, not a mistake in your code. Retry later; your source file
is not lost.

### My output looks right but the case says wrong output

The comparison is stricter than it looks. Check:

- extra or missing blank lines;
- a missing final newline is accepted, but other line differences are not;
- uppercase and lowercase are different in strict text;
- floating-point formatting and trailing spaces;
- extra text printed to standard output instead of standard error.

Open the case detail and compare **Saída do programa** with **Saída esperada**
when the professor released them.

### The result says interrupted

The program exited with a non-zero status, received a signal, exceeded the
case's time or file-size limit, or could not produce output. Common causes are
an uncaught exception, an out-of-range array index, a division by zero, reading
input past its end, or an infinite loop.

### I lost my editor draft

Drafts live in this browser's local storage. Try the same browser and device,
without clearing site data. If the draft is gone, open **Submissões anteriores**
and restore the last submission. The server keeps submissions, not drafts.

### I need to leave or change a class

Use **Sair** on **Minhas Turmas** to leave, then join again with a valid code.
If enrollment is closed, ask the professor. A banned account cannot join until
the professor clears the ban.

### I need my account deleted or my role changed

The platform has no self-service account deletion. Ask an administrator. Role
changes also require an administrator, and a student cannot be assigned as a
co-professor.

## Related pages

- [How grading works](./grading.md)
- [Submissions & live judging](./submissions.md)
- [Classes, roles & access](./classes-and-roles.md)
- [Accounts & settings](./accounts-and-settings.md)
- [Professor guide](./professor-guide.md)
