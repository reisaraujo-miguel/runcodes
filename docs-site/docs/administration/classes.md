---
title: Class & ownership administration
description: Find any class, edit its metadata, transfer ownership, inspect members and delete a class safely.
---

# Class & ownership administration

**Todas as Turmas** (All classes) is the platform-wide class directory. Use it to
fix class metadata, hand a class to another professor, inspect membership and
remove a class. It does not replace the teaching area: exercises, test cases and
grades belong to the owner and co-professors.

## Search and page through classes

The page lists 200 classes at a time and offers **Carregar mais** (Load more)
when more exist. The search matches, case-insensitively:

- the class name;
- the owner's name;
- the owner's email.

It does not search descriptions, institutional ids or enrollment codes. The API
supports `limit` and `offset` for scripts; the default page is 50 and the maximum
is 200.

Each row shows:

- class name and description;
- the owner's name and email;
- member and exercise counts;
- **Matrícula aberta** (Enrollment open) or **Matrícula fechada** (Enrollment
  closed);
- the end date;
- **Editar**, **Ver membros** (View members) and **Excluir** (Delete).

## Edit a class

Select **Editar** (Edit) to change:

| Field | Effect |
| --- | --- |
| **Nome** (Name) | class name |
| **Descrição** (Description) | optional text shown to members |
| **Data de encerramento** (End date) | when enrollment closes |
| **Matrícula aberta para os alunos** (Enrollment open) | whether the code can be used |
| **Novo responsável** (New owner) | transfer ownership by email or institutional id |

The end date closes enrollment; it does not change exercise deadlines. Students
already enrolled keep access after the end date, but the class is shown as
closed. Tell the owner to align exercise deadlines with the class period.

Clearing **Matrícula aberta** blocks new students immediately. Existing members
are unaffected. Rotating the enrollment code is an owner action on the class
page; the panel does not have a rotate button.

## Transfer ownership

Enter the new owner's **email** or **institutional id** in **Novo responsável**
and save. The field accepts one of the two, not both:

- an address contains `@`, so it is treated as an email;
- anything else is treated as an institutional id, such as a university
  registration number.

The new owner must have the `professor` or `admin` platform role. A student
cannot receive a class. The new owner does not need to be a member of the class:
ownership grants access on its own.

What changes:

- the new owner can edit the class, manage members, author exercises and see the
  gradebook;
- the previous owner loses owner privileges immediately;
- the previous owner keeps any enrollment they had, including a co-professor
  enrollment, but otherwise loses access;
- no enrollment is created for the new owner and the previous owner is not
  removed from the member list.

Transfer is one action with the field edit. If the new owner cannot be resolved,
the whole update fails and no field changes.

::: warning Demoting an owner leaves the class unmanaged
If you change an owner's platform role to `student` without transferring the
class first, the class keeps the same `owner_id` but the account can no longer
use owner privileges. Transfer the class before demoting the account.
:::

## Inspect members

Select **Ver membros** (View members) to see every participant with their role
and ban status. The dialog is read-only. Class membership is managed by the
owner on the class's **Participantes** tab:

- students join with the code;
- monitors and co-professors are added by email;
- the owner changes roles, bans members and removes them.

The panel cannot add or remove a member. If an administrator must fix
membership, ask the owner. If the owner is unavailable, transfer ownership to a
professor who can manage it.

## View grades

The admin panel has no gradebook. The gradebook route belongs to the class's
teaching routes, which an administrator can use only when they own the class or
were added as a co-professor.

To inspect a class you do not teach, ask the owner to add you as a co-professor.
A temporary ownership transfer also works, but it removes the owner's privileges
until you transfer it back; prefer the co-professor route.

The class owner and co-professors can open **Notas** (Grades) on the class page.
The rules behind the numbers are in [How grading works](../guide/grading.md).

## Delete a class

Deleting a class removes:

- the class row;
- its exercises and test cases;
- compilation files and attached materials;
- every submission in the class and its stored source file.

Output archives are not rows in the database; the judge removes them on its own
artifact retention schedule.

The action cannot be undone. Ask the class owner to export it from **Ferramentas**
before deleting it. The export contains the course material but not student
submissions or grades.

If the class belongs to a professor who is leaving, decide whether to transfer it
instead of deleting it. Deleting the professor's account also deletes every class
they own.

## Class lifecycle

A typical class moves through these states:

1. **Created.** A professor creates it; it gets a six-character code and starts
   with enrollment open.
2. **Teaching.** The owner adds co-professors and monitors and publishes
   exercises.
3. **Enrollment closed.** The owner clears the enrollment setting, the end date
   passes, or the owner rotates the code so the old one stops working. Existing
   students keep access.
4. **Archived.** The owner exports the class material for the next term.
5. **Deleted.** The owner or an administrator removes the class. This is
   irreversible.

The admin panel is involved mainly in steps 3 and 5, and in ownership changes
between terms. Class creation and exports happen in the teaching area.

## Common tasks

| Task | Steps |
| --- | --- |
| Find a class by owner | **Todas as Turmas**, search the owner's email |
| Fix a typo in a class name | **Editar**, change **Nome** |
| Close enrollment early | **Editar**, clear **Matrícula aberta** |
| Extend a class | **Editar**, set a later end date (exercise deadlines are separate) |
| Hand a class to another professor | **Editar**, set **Novo responsável** |
| See who is enrolled | **Ver membros** |
| Read the gradebook | ask the owner to add you as a co-professor |
| Export the material | ask the owner to use **Ferramentas → Exportar turma** |
| Delete a class | **Excluir**, after exporting anything worth keeping |
| Delete a professor who owns classes | transfer or export the classes first |

## Related pages

- [Administrator guide](./index.md)
- [Users & role administration](./users.md)
- [Professor guide](../guide/professor-guide.md)
- [Classes, roles & access](../guide/classes-and-roles.md)
- [API: administration](../api/admin.md)
