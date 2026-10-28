---
title: Professor guide
description: Create and reuse classes, manage members, author exercises and test cases, grade a class and use the teaching tools.
---

# Professor guide

This guide covers the teaching side of RunCodes: creating a class, enrolling
students, authoring exercises and test cases, checking your own work, reading the
gradebook and reusing the course material. It is written for an account with the
`professor` platform role and for co-professors invited to a class.

The RunCodes interface is in Brazilian Portuguese. This page shows the labels as
they appear on screen and gives the English meaning the first time a label is
used. The grading rules are in [How grading works](./grading.md).

## Before you start

- An administrator must change your account role to `professor`. A student
  account cannot create a class.
- **Gerenciar Turmas** (Manage classes) appears under **Ensino** (Teaching) in
  the sidebar.
- A class is called an **offering** in the API and database. The UI calls it a
  **turma**.
- Students join with a six-character enrollment code. You cannot add a student
  by email.
- An administrator can transfer the ownership of a class. Only the owner can
  edit or delete the class, rotate its code and manage members. An administrator
  can also edit or delete any class from the admin panel.

::: tip Check your role first
If **Gerenciar Turmas** is missing, the account is not a professor or an admin
(usually it is still a student). Ask an administrator to promote it. An `admin`
role is not enough to see a class you do not own or teach; the admin panel uses
separate routes.
:::

## The teaching workspace

### Class list

**Gerenciar Turmas** lists the classes you own and the classes in which you were
invited as a co-professor. Each card shows:

- **Responsável** (Owner) or **Professor convidado** (Invited professor);
- the number of students and exercises;
- the class end date;
- **Matrícula aberta** (Enrollment open) or **Matrícula encerrada** (Enrollment
  closed);
- the enrollment code;
- **Abrir turma** (Open class).

The page also has **Nova turma** (New class) and **Importar turma** (Import
class).

### Class tabs

A class has five tabs:

| Tab | Purpose |
| --- | --- |
| **Detalhes** (Details) | code, end date, enrollment status, edit and delete |
| **Exercícios** (Exercises) | create, open and manage exercises |
| **Notas** (Grades) | gradebook, statistics and CSV export |
| **Ferramentas** (Tools) | export the class, download submissions, recompile, email |
| **Participantes** (Participants) | members, roles, bans and invitations |

### Exercise tabs

An exercise has four tabs:

| Tab | Purpose |
| --- | --- |
| **Detalhes** (Details) | dates, visibility, allowed types, statement |
| **Testes** (Tests) | test cases and batch import/export |
| **Compilação** (Compilation) | files placed in the workspace before the build |
| **Materiais** (Materials) | files students can read or download |

### Permissions in a class

| Action | Owner | Co-professor | Monitor |
| --- | :-: | :-: | :-: |
| Open the class and exercises | ✓ | ✓ | ✓ |
| Submit a solution | ✓ | ✓ | ✓ |
| Create and edit exercises | ✓ | ✓ | — |
| Manage test cases and files | ✓ | ✓ | — |
| View the gradebook | ✓ | ✓ | — |
| Export the class, recompile, email | ✓ | ✓ | — |
| Edit or delete the class | ✓ | — | — |
| Rotate the enrollment code | ✓ | — | — |
| Manage participants | ✓ | — | — |

A monitor can read the class and exercises and submit solutions, but cannot
author content or see grades. A co-professor can author everything and see the
gradebook, but cannot manage members or delete the class.

## Create a class

1. Select **Nova turma** on **Gerenciar Turmas**.
2. Enter a name and an optional description.
3. Choose **Disponível até** (Available until), the class end date.
4. Select **Criar turma** (Create class).

The new class opens on its management page. The server generated a six-character
enrollment code; copy it with **Copiar** (Copy) and share it with the students.

### Edit a class

On **Detalhes**, select **Editar turma** (Edit class). You can change:

- **Nome da turma** (Class name);
- **Descrição** (Description);
- **Disponível até** (End date);
- **Permitir que novos alunos se matriculem com o código** (Allow new students to
  enroll with the code).

Clearing the enrollment option closes enrollment but keeps existing students and
their work. Enrollment also stops when the class end date passes.

::: warning The class end date does not extend exercise deadlines
The end date closes enrollment. Each exercise has its own deadline, and the
submission rule uses that deadline. Existing students keep access after the end
date, but the class is shown as closed. Set exercise deadlines inside the class
period so the class does not close while work is still due.
:::

### Rotate the enrollment code

Select **Gerar novo código** (Generate new code) on **Detalhes**. The current
code stops working immediately. Students who already joined keep their
enrollment; students who have not joined need the new code. Use this when a code
leaks or when a class is reused.

### Delete a class

Deleting a class deletes its exercises, test cases, files and every student
submission. The action is irreversible. Export the class first if you may need
the material.

## Import a class

Use this to reuse a course in a new term or to restore an exported class as a
new class.

1. Select **Importar turma** on **Gerenciar Turmas**.
2. Upload a `.zip` produced by **Ferramentas → Exportar turma**.
3. Optionally set a new name, description and end date.
4. Select **Importar turma**.

The import restores the class configuration, exercises, test cases, compilation
files and materials. Students, submissions and grades are deliberately not part
of the archive. The importer becomes the owner and the new class gets a new
enrollment code.

If you set a new end date, every exercise date shifts by the same interval, so
the relative schedule survives a new semester. Without a new date, the original
end date is kept; if that date has passed, the imported class gets a six-month
period. This prevents importing a class that is already closed.

The allowed file types must exist in the platform catalog. If the archive uses
an extension an administrator has not registered, the import fails before
creating the class. Ask the administrator to register it.

## Manage participants

Open the **Participantes** tab. Only the owner sees the management controls.

### Add a monitor or co-professor

1. Enter the person's account email in **E-mail**.
2. Choose **Monitor** or **Professor** in **Papel** (Role).
3. Select **Adicionar** (Add).

The account must already exist. Assigning the professor role requires the
account's platform role to be `professor` or `admin`; a student account cannot
become a co-professor without an administrator changing the platform role first.

Adding an existing member updates the role and clears any ban.

### Change a role, ban or remove

Use the role selector and **Salvar** (Save) to change a monitor to co-professor
or the reverse. To make someone a student again, remove the enrollment and let
them rejoin with the code. **Banir** (Ban) keeps the role and the historical work
but removes access; **Desbanir** (Unban) restores it. **Remover** (Remove)
deletes the enrollment.

The owner cannot be banned or removed, and the management controls are hidden
for the owner's row. A co-professor cannot manage participants at all.

### How students join

Students join with the enrollment code, not with an email invitation. Share the
code, or write it on the board. Joining is idempotent, so a student who enters it
twice stays enrolled once.

A student cannot join when the class has ended, enrollment is closed, the code
was rotated, the student is banned or the student is rate limited after five
failed attempts in a minute.

## Create an exercise

1. Open the class and select the **Exercícios** tab.
2. Select **Novo exercício** (New exercise).
3. Fill in:
   - **Título** (Title);
   - **Abertura** (Opening date and time);
   - **Prazo** (Deadline date and time);
   - **Mostrar o exercício antes da data de abertura** (Show the exercise before
     its opening date);
   - **Tipos de arquivo permitidos** (Allowed file types);
   - **Instruções do exercício** (Exercise instructions), written in Markdown.
4. Select **Criar exercício** (Create exercise).

The new exercise opens on its management page so you can add test cases and
files immediately.

### Dates and visibility

- The exercise is hidden from students until its opening date unless you enable
  **Mostrar o exercício antes da data de abertura**.
- Students cannot submit before the opening date, even when they can read the
  statement early.
- Students cannot submit after the deadline. Owners and co-professors can, so
  staff can test the exercise before and after releasing it.
- The class page shows the exercise to enrolled students according to these
  rules; the dashboard and **Exercícios** list only what is open right now.

### Allowed file types

Select the extensions the exercise accepts. The list comes from the platform
catalog, which an administrator maintains. Compound extensions such as
`omp.cpp` and `mpi.c` are supported.

If you leave every type unchecked, the exercise accepts any file. Prefer
selecting the exact types: a broad exercise accepts a file the judge may not map
to the language you intended.

The extension of the submitted file selects the language image. Check the
[language matrix](../reference/languages.md) before choosing a type, and test
with a real file.

### Instructions

The instructions field accepts Markdown. The student sees the rendered text in
the statement panel. Use headings, lists, code blocks and links. A PDF attached
under **Materiais** can hold a longer statement; the first PDF is shown inline in
the statement panel.

## Write test cases

Open the exercise and select **Testes**. The **Novo caso** (New case) form
creates one case.

### Input and expected output

Each side can be **Texto** (text) or **Arquivo** (file):

- **Entrada** (Input): the data the program reads.
- **Saída esperada** (Expected output): what a correct program must print or
  write.
- **Arquivos adicionais do caso** (Extra case files): files placed beside the
  case input.

A file-based input or expected output is stored as an object. A text input or
expected output lives in the database and is shown inline in the results panel
when you release it.

### Limits

Set the four limits for the case:

| Field | Unit | Notes |
| --- | --- | --- |
| **Tempo limite** (Time limit) | seconds | bounds the wall-clock execution time of the case |
| **Limite de memória** (Memory limit) | bytes | caps the address space |
| **Limite de pilha** (Stack limit) | bytes | caps the process stack |
| **Limite de arquivo** (File-size limit) | bytes | caps a file the program writes |

A value of 0 uses the platform default. The judge also applies deployment-wide
cgroup caps, so a case limit above the deployment cap still hits the cap. Ask the
administrator when a case needs a larger backstop.

### Visibility

- **Mostrar entrada** (Show input): students can read the case input.
- **Mostrar saída esperada** (Show expected output): students can read the
  expected output.
- **Mostrar saída do aluno** (Show student output): students can read their own
  program's output for the case.

All three are off by default except the student output, which starts on. Use the
flags deliberately: showing the expected output of every case makes the exercise
trivial, while hiding the input of a large case keeps the statement readable.
File-based inputs and outputs are never displayed inline, even when their flag is
on.

### Case order and types

Cases are numbered per exercise in display order, starting at 1. The number is
not the global database id.

The input and output types are fixed after creation. To change a case from text
to file, create a new case and delete the old one. The case values can be edited
in place. Extra files are replaced as a whole when you upload new ones; the form
does not remove an individual extra file.

### Edit or remove a case

Select **Editar** (Edit) on a row to edit it inline. Removing a case deletes it
from the exercise and from future runs. Removing all cases makes the exercise
unsubmittable.

### Import cases in bulk

Select **Importar em lote** (Batch import) to upload a zip with one folder per
case. The archive must follow this layout:

```text
caso1/
  entrada.in
  saida.out
  ajuda.txt
caso2/
  entrada.in
  saida.out
```

Every folder must contain exactly one `.in` file and exactly one `.out` file.
Every other file in the folder becomes an extra case file. Put an extra file
whose name ends in `.in` or `.out` in a `files/` subfolder, so the importer does
not mistake it for the case input or output.

The form applies one configuration to every case in the batch: input and output
type, visibility flags and limits. Check the values before importing.

The archive is validated completely before anything is written. A broken folder
is reported by name and the whole import is refused. Limits: 500 cases, 64 MiB
compressed, 256 MiB expanded, 32 MiB per file and 4000 zip entries.

### Export cases

Select **Exportar .zip** (Export zip) to download every case in the same layout.
The export uses `caso_001/input.in`, `caso_001/output.out` and the extra files,
so you can import it into another exercise. The import form applies its own
configuration; the export carries the input and output content, not the limit
and visibility settings.

## Add compilation files

Open **Compilação**. Upload headers, Makefiles, library files or any other file
the build needs. The judge places these files in the workspace before it compiles
the submission.

- Uploading a file whose basename already exists replaces the stored file.
- Deleting a file affects future runs; existing submissions are unchanged until
  you recompile them.
- A `Makefile` in the workspace lets the harness build a multi-file project. Keep
  the entry point and the source file name consistent with the exercise's allowed
  types.

## Add materials

Open **Materiais**. Upload the statement, datasets, starter code or reference
material. Students see the files in the statement panel and can download them.

- A PDF statement is shown inline in the statement panel.
- Uploading the same file name replaces the stored material.
- Materials are visible to every student who can see the exercise, so do not put
  solutions or private test data there.

## Test the exercise yourself

Before releasing an exercise:

1. Open the class and select **Enviar solução** (Submit solution) on the exercise
   row, or open the exercise and use the submission workspace.
2. Submit a known-correct solution and a known-wrong solution.
3. Check the case verdicts, the score and the compilation message.
4. Check the limits with a deliberately slow or memory-heavy solution.
5. Check that the statement and materials render correctly for a student.

Staff submissions are not part of the class gradebook because the gradebook
lists students only. Use them as a check and ignore them in the grade review.

If the submission is refused with `422`, the exercise has no test cases. Add at
least one case before testing.

## Use the gradebook

Open the class and select **Notas**.

### What the table shows

- one row per enrolled, non-banned student;
- one column per non-removed exercise;
- the best score of that student on that exercise;
- the number of submissions when it is greater than one;
- the student's **Média** (Class grade).

The class grade is the mean of the best scores over every exercise, counting an
exercise without a submission as zero. A missing submission is shown as `—` in
the table but still counts as zero in the class grade. Removed exercises are
excluded. Monitors and co-professors are not students and do not appear.

### Statistics

The page also shows class and per-exercise statistics: students with a
submission, students above 50, the class average, the exercise average, the
average among students who submitted, the approval rate and the best and lowest
scores. The exact definitions are in
[How grading works](./grading.md#best-score-and-class-grade).

### Download the CSV

Select **Baixar CSV** (Download CSV). The file contains:

- student name and email;
- one column per exercise, with the best score;
- **Média** (Class grade);
- **Submissões** (Submission count);
- **Última submissão** (Last submission time).

An exercise with no submission is an empty cell, not a zero, so a spreadsheet can
distinguish the two. The class grade column still counts it as zero.

Student names and exercise titles that start with a spreadsheet formula
character are prefixed with an apostrophe. Do not remove it if you import the CSV
into a spreadsheet.

### When grades change

The gradebook reads the stored scores. If you change test cases, compilation
files or limits, existing scores do not change until the submissions run again.
Use **Recompilar** (Recompile) in **Ferramentas** to queue a fresh run.

## Use the class tools

Open the class and select **Ferramentas**.

### Export the class

**Exportar turma** downloads a zip with the class configuration, exercises, test
cases, compilation files and materials. It does not include students,
submissions or grades. Import it with **Importar turma**.

### Download the latest submissions

**Baixar submissões** downloads a zip with the latest submission of every
student on every exercise, grouped by student and exercise, plus:

- `notas.csv` with the class, student, email, exercise, score of that submission,
  status, time and submission number;
- `LEIA-ME.txt` explaining the layout.

The score in `notas.csv` is the score of the latest submission, not the best
historical score. A source object that is no longer in storage is replaced by an
`.indisponivel.txt` note, so one missing object does not abort the download.

### Recompile submissions

**Recompilar** queues a fresh run of the latest submission of every student on
every exercise. Use it after fixing test cases, compilation files, limits or the
runner image.

- A new commit is created; the old result stays in the history.
- The gradebook keeps the best score, so a recompilation can raise a grade but
  cannot lower a previous best.
- Up to 5000 submissions are queued per request.
- The judge must be ready. If it is not, the request fails and nothing is queued.
- The response reports how many runs were queued; the runs execute in the
  background. The class page shows the best score while the new runs are pending.

Do not recompile during a submission window if the class is active; it competes
with student runs for judge workers.

### Email the class

**Escrever email** (Write email) sends one message to every non-banned member of
the class, including students, monitors, co-professors and the owner.

- **Assunto** (Subject) is required and at most 200 characters; it cannot contain
  a line break.
- **Mensagem** (Message) is required and at most 20,000 characters.
- The message is sent as plain text with an HTML version. Attachments are not
  supported.
- Delivery runs in the background; the dialog reports how many recipients were
  queued. Delivery failures are logged by the server.

Use the class email for schedule changes and reminders. The platform also sends
automatic deadline reminders to students who have not submitted when an exercise
closes in 7 days and again in 3 days.

## Reuse and collaborate

### Reuse a class next term

1. Export the class before the term ends.
2. Import it next term with a new end date.
3. Update the name, description and any material that changed.
4. Check the allowed file types against the current catalog.
5. Share the new enrollment code.

The archive shifts every exercise by the change in the end date, so the relative
schedule is preserved. Students and grades are not copied.

### Share teaching duties

Ask the owner to add a co-professor with the professor role. A co-professor can
author exercises and see grades but cannot manage members. An administrator can
transfer ownership by email or institutional id; the previous owner loses owner
privileges but keeps any enrollment.

### Back up before destructive changes

Export the class before deleting it or before changing many test cases. The
recompile tool preserves old commits, but it does not preserve the old version of
the test cases themselves.

## Troubleshooting

| Symptom | Likely cause and fix |
| --- | --- |
| **Gerenciar Turmas** is missing | the account role is `student`; ask an administrator to promote it |
| A co-professor cannot edit the class or members | only the owner can; ask the owner or an administrator |
| A student cannot enroll | wrong or rotated code, enrollment closed, class ended, ban, or five failed attempts in a minute |
| A student sees no exercises | the opening date has not arrived and **Mostrar antes** is off, the exercise is removed, or the student is not enrolled |
| A submission is refused with `422` | the deadline passed for a student, or the exercise has no test cases |
| A recompilation fails with `503` | the judge is not ready; check with the administrator, then retry |
| An imported class fails on a file type | the extension is not in the platform catalog; ask an administrator to register it |
| A batch import fails | a folder has no `.in`, no `.out`, more than one of either, a nested folder, or the archive exceeds a limit |
| A CSV grade looks different from the table | missing cells are empty in the CSV but count as zero in the class grade; the table shows the best score, while **Baixar submissões** reports the latest |
| The class email did not arrive | SMTP is misconfigured, or recipients are on a blocked relay; check with the administrator and the server logs |
| Students cannot see the expected output | **Mostrar saída esperada** is off for the case |
| The wrong language ran | the file extension does not match the intended language; check the allowed types and the language matrix |
| Grades did not change after editing cases | existing runs are not rescored; use **Recompilar** |

## Related pages

- [How grading works](./grading.md)
- [Student guide](./student-guide.md)
- [Classes, roles & access](./classes-and-roles.md)
- [Submissions & live judging](./submissions.md)
- [Languages & images](../reference/languages.md)
- [Administrator guide](../administration/index.md)
