# IPyIntervu Week Scope (CSE 110 syllabus)

The server state JSON includes `assessmentWeekScope` with `allowedWeekNumbers`, `primaryWeekNumber`, and `forbiddenConceptsFromLaterWeeks`. That object is **authoritative** for every assessment mode.

## Rule

- **Allowed:** concepts from weeks 1 through `primaryWeekNumber` (selected week plus all prior weeks as support).
- **Forbidden:** any concept whose **first introduction** is in a week after `primaryWeekNumber`.
- **Mandatory:** before presenting a conceptual question, code task, bug snippet, or follow-up, verify it does not require forbidden concepts. **Revise the draft** if it does.

Prior-week concepts may appear as supporting ingredients (e.g. Week 8 code may use `if`/`elif`/`else` from Weeks 5–6 and `for` from Week 7). Later-week concepts must not appear—not even as optional extras or “nice to have” features.

## Dictionary prohibition (all weeks)

**Dictionaries are never in scope** for any assessment mode or coaching session — not even in Week 9 or Week 10.

- Do not use, require, compare to, or mention dictionaries (or *dict*, keyed maps, `{}` lookup patterns) in conceptual questions, code tasks, bug snippets, follow-ups, or coaching.
- Week 9 assesses **lists only**. Do not ask “list or dictionary?” or similar.
- Week 10 combines **lists and file I/O**; organize file data with lists, not keyed lookup structures.

See **Dictionary prohibition** in IPyIntervu-protocols.md.

## Concepts introduced by week

| Week | Key concept | First introduced in this week |
|------|-------------|-------------------------------|
| 1 | Problem Decomposition | input/process/output, breaking tasks into steps (no code) |
| 2 | Variables & Expressions | variables, assignment, expressions, int/float/str/bool |
| 3 | Input & Type Casting | `input()`, type casting, `int()`/`float()`/`str()` |
| 4 | String Methods | string methods (`strip`, `upper`, `lower`, `split`, etc.) |
| 5 | if Statements | `if`, comparisons, boolean logic (`and`/`or`/`not`) |
| 6 | elif/else | `elif`, `else`, multi-branch conditionals |
| 7 | for Loops | `for`, `range()`, iterating sequences |
| 8 | while Loops & Menus | `while` with condition in header, menus, repeat-until-quit |
| 9 | Lists | lists, list indexing/methods, list iteration |
| 10 | Lists and Files | file I/O (`open`, read/write, `with open`) |

## Pre-presentation checklist (all assessment modes)

1. Identify constructs the student must use to complete the task or answer the question.
2. For each construct, confirm it appears in weeks 1..`primaryWeekNumber`.
3. If any construct is from a later week, **rewrite** using only allowed constructs.
4. Do not rely on general Python knowledge outside this syllabus when generating tasks.

## Week 8 examples

**Allowed code tasks:** menu-driven calculator (`while choice != 'Q'`); interactive tool that repeats until user quits; ATM or lab-instrument menu using `if`/`elif` inside a `while` loop; tasks that use variables, `input()`, strings, conditionals, `for`, and condition-driven `while`.

**Forbidden in Week 8** (require Week 9+):

- Storing multiple items in a **list** or history log (`[]`, `.append()`, iterating a collection of records).
- **Reading or writing files** (`open()`, file paths, CSV/JSON file loading).
- Tasks framed as “process a dataset”, “load records from a file”, or “maintain a list of transactions”.

If a Week 8 task needs repeated actions, use a **menu loop** and scalar variables—not collections from later weeks.
