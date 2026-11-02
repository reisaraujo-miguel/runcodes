---
title: Language statistics
description: How the platform and class language reports measure adoption, professor allowance and CPU cost fairly.
---

# Language statistics

**Linguagens** (Languages) is the admin report on the file-type catalog. It
answers three questions that look similar but must not be read as one ranking:

- **What do students choose?** Submissions per language, the students behind
  them, and each language's share of the submissions in the exercises that
  actually offered it.
- **What do professors allow?** How many exercises, classes and distinct
  professors list each language as an allowed file type.
- **How much CPU does each language use?** A paired comparison that uses each
  test case as its own baseline, so harder exercises do not make a language look
  slower.

The platform report lives at `/admin/languages` and only administrators can open
it; it aggregates every class. The owner and co-professors of a class get the
same metrics scoped to their class in the **Linguagens** tab of the class page.
The rest of this page describes the shared method; the last section explains
what changes in the class view.

## Where the language comes from

The catalog entry is recorded when the submission is accepted: the backend
matches the filename against the exercise's allowed file types and stores the
matched id on the commit. When more than one allowed extension matches, the
longest wins, so a file named `main.omp.c` counts as **C (OpenMP)** if the
exercise allows both `.c` and `.omp.c`. A `.zip` submission counts as the
**Makefile** catalog entry; the language the judge later detects inside the
archive is not recovered for statistics.

Exercises without allowed file types, and rows whose stored filename matches no
allowed extension, have no language. They appear as **unclassified submissions**
and stay out of every per-language rate. Historical rows and legacy imports are
backfilled with the same longest-extension rule when the migration or importer
runs. The report never guesses a language from a filename that the professor did
not allow.

## Adoption: choice relative to opportunity

The raw number of submissions is the easiest metric to read and the easiest to
misinterpret: a language offered in two exercises cannot collect as many
submissions as one offered in twenty.

The page therefore shows two adoption numbers:

| Metric | Denominator | What it answers |
| --- | --- | --- |
| **Share of submissions** | all classified student submissions | how much of the platform's activity used the language |
| **Choice rate when offered** | submissions inside exercises that allowed the language | how often students pick the language when it is available |

The choice rate is computed per exercise, then averaged with each exercise
counting once. For an exercise that allows C, Python and Java, a language's share
is its submissions divided by all student submissions in that exercise. Exercises
where the language was allowed but nobody submitted anything count as zero — the
language had the opportunity and was not chosen. Exercises where the language was
not allowed do not count at all.

`pick_rate_samples` reports how many exercises are behind the average. A choice
rate built on two exercises is a much weaker signal than one built on twenty, and
the table shows the count so the reader can judge.

Only student accounts count here. A professor trying an exercise, a monitor's
test run or an administrator's preview does not shape the "what students choose"
view.

## Allowance: professors, not just exercises

A single professor who authors a hundred exercises can make a language look
universally allowed if only exercises are counted. The allowance table separates:

- **Exercises**: distinct non-removed exercises that list the type;
- **Professors**: distinct authors of those exercises;
- **Classes**: distinct offerings that contain at least one such exercise.

The exercise percentage uses every non-removed exercise on the platform as the
denominator. An exercise with no allowed types still counts in the denominator,
which is why the percentages do not need to add up to 100%.

## CPU: a paired comparison

Comparing raw CPU time across languages is misleading because each language is
used on a different mix of exercises. A language chosen mostly for the heavy
exercises will look slow even if its runtime is efficient. The report removes
that mix by comparing only submissions that ran the **same test case**.

For every test case:

1. Reduce each language to one number: the median CPU time of its results on that
   case.
2. Compute the median of those per-language medians. That is the case's baseline.
3. Divide each language's median by the baseline. A ratio of `0.80×` means the
   language used 80% of the typical CPU on that case; `1.20×` means 20% more.

The language's headline index is the **median of its ratios across test cases**,
not the ratio of its totals. Each comparable case counts once, so a language with
a thousand submissions on one exercise cannot dominate the index. The p25–p75
range shows how consistent the result is: a narrow range is a steadier estimate
than a wide one.

### Why single-language cases are excluded

A test case with only one language has no peer. Treating it as a `1.00×` ratio
would pull the index toward 1.0 for whichever language happens to be the only
choice on many exercises, which is exactly the bias the paired method is meant to
remove. Such cases appear in `results` and `test_cases`, but not in
`compared_test_cases`.

### What the numbers can and cannot say

The comparison is **observational**, not a controlled benchmark:

- Students are not randomly assigned to languages. A cohort that is more
  experienced, or more hurried, can make a language look better or worse.
- Test cases are written by professors and may not stress every language's
  weak spots equally.
- The CPU index covers execution CPU. Compile time is measured separately by the
  compile monitor. Memory has its own paired index, but only for results with a
  known memory figure; coverage is reported because old runs and hosts without a
  readable memory controller record `-1`.
- A language with fewer than five comparable cases is marked as a small sample.
  The table lists reliable samples first; small samples and unmeasured
  languages come after them, so a one-case outlier does not lead the ranking.
  Read a small sample as a hint, not a result.
- The report is cached for up to one minute, so a brand-new submission may not
  appear in the next page load.

Use the index to find languages worth a closer look, not to declare a winner.
When a number matters for a decision, reproduce it with a controlled run of the
same programs on the same test cases.

## Memory

The **Memória** table applies the same paired method to peak memory: per test
case, each language is reduced to its median peak, the case's baseline is the
median across the languages that ran it, and the language index is the median of
those ratios.

Only results with a known memory figure enter. The monitor samples the
container cgroup's `memory.current` every 10 ms; `-1` means the host exposed no
readable controller or the image predates the measurement. The coverage column
shows known results over every classified result, so a comparison built on a
small slice is visible.

The number is a cgroup peak, not pure RSS: it includes the harness, the runtime
and page cache, and a peak shorter than 10 ms can be missed. Read it as a
comparison between languages on the same test case, not as an absolute memory
profile.

## Where the time goes

The **Onde o tempo vai** table breaks a run into measured phases. These are
medians, not a budget that adds up to the total: each phase is measured on the
runs where it exists, and a median of phases is not the median of their sum.

| Phase | Source | What it includes |
| --- | --- | --- |
| Queue | judge claim time minus submission time | waiting for a worker |
| Preparation | judge, around `prepareWorkspace` | source download, ZIP extraction, compilation files, test-case fetch, config |
| Image | judge, around `EnsureImage` | local image check or pull; cold pulls are counted separately and excluded from the median |
| Startup | judge, container start to first phase milestone | container boot and harness initialization up to `compilation.start` or `run.start` |
| Compilation | compile monitor report | the compiler command's wall and CPU time; not available for interpreted languages |
| Execution | judge, `run.start` to `run.done` | all test cases, including per-case pre/post commands |
| Correction | judge, around grading | reading outputs and comparing them |

Every row also shows the sample count and coverage: measured runs over classified
runs in the scope. Historical submissions have no phase rows and appear as “sem
dados”. The image median deliberately excludes cold pulls, because a first pull
is an operational event, not the language image's warm startup.

The compile time is the monitor's own measurement of the compiler command, so it
does not include workspace preparation or container startup. That is why the
table is more precise than subtracting the old `compilation_started` and
`compilation_finished` columns, which bracket the whole claim-to-event path.

## The class view

The **Linguagens** tab on a class page runs the same report scoped to that class.
Only the owner and co-professors see it. The differences are about scope, not
method:

- every denominator counts only the class's non-removed exercises and its
  student submissions;
- the **Professores** card counts the authors of that class's exercises;
- the tables omit catalog entries the class neither offers nor used, so a class
  that uses three languages does not list the other twenty;
- the **Turmas** column is hidden because every row belongs to the same class;
- the cache is per class and still expires within a minute.

The fairness rules are unchanged: the choice rate is conditional on the
exercises that offered the language, and the CPU index is paired by test case
within the class.

## Reading the page

The detail views are tabs: **Escolha**, **Oferta**, **CPU**, **Memória** and
**Fases**. Only the selected view is visible, and each table scrolls inside its
panel with the header pinned, so switching views does not require scrolling
past every metric.

| Section | Shows |
| --- | --- |
| Summary cards | classified submissions, languages chosen, exercises, professor authors |
| **Escolha dos alunos** | submissions, share, students and the choice rate when offered |
| **Oferta dos professores** | exercises, professors and classes per language |
| **Tempo de CPU** | relative index, p25–p75, compared cases, median CPU per case, executions |
| **Memória** | relative peak-memory index, range, compared cases, median bytes and coverage |
| **Onde o tempo vai** | queue, preparation, image, startup, compilation, execution and grading medians |

A dash (`—`) in the CPU table means the language has no comparable test case
yet. On the platform report, a language with zero submissions is still listed
under allowance and CPU so the catalog is complete; the class report hides
entries the class never offered or used. The choice section hides
zero-submission languages and states the total.

## Related pages

- [Administrator guide](./index.md)
- [Settings & file types](./platform.md)
- [Languages & images](../reference/languages.md)
- [How grading works](../guide/grading.md)
