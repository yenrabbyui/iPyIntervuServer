# iPyInterVu grading rules

_Spec for the hard-coded grading in `grading.go`. Decided 2026-10-01. Never sent to a model._

The model's only grading job is to label each answer with a rubric level, using the level descriptions in `env/rubrics/weekN_rubric.md`. Everything below is done by Go.

## Levels and labels

- Levels, from lowest to highest: `not_ready` < `competent` < `exceptional`.
- Buckets shown to students: **Not Ready Yet**, **Competent**, **Exceptional**, and **N/A** for a mode that doesn't run.
- The labelled-line parser also accepts "Not Yet Ready", "not ready" and "not ready yet" as `not_ready`, case-insensitively. Anything else is ignored and logged.

## Step 1 — Recording labels

Each student answer can produce labels: a level for each dimension that answer gave evidence for.

| Mode | Dimensions |
| --- | --- |
| Conceptual | `conceptual` |
| Code | `decomposition`, `correctness`, `understanding`, `ai_use` |
| Bug | `strategy` |

**Where labels come from**
- The Evaluator's brief after each answer (`LEVELS:` line).
- The levels-only call at mode close, which labels the final answer.

**Labels Go sets itself**
- **Vague answers.** When `isVagueAnswer` is true, the answer gets `not_ready` on the dimension its question targeted. This applies whatever the Evaluator says.
    - Conceptual and Bug: the mode's single dimension.
    - Code: the dimension of the move that asked the question:

      | Question asked by | Dimension |
      | --- | --- |
      | Opening, or a decomposition follow-up | `decomposition` |
      | REQUEST_CODE | `correctness` |
      | CODE_FOLLOW_UP about a line or choice | `understanding` |
      | CODE_FOLLOW_UP about AI use | `ai_use` |

      Because "lowest" wins (Step 2), the one allowed redirect after a vague answer cannot raise the grade.
- **No pasted code.** If Code mode closes without `looksLikeCodeSubmission` ever being true, Go records `correctness = not_ready`.

**Labels that are not recorded:** answers handled by the CLARIFY move, where the student asked for clarification rather than answering.

## Step 2 — Mode buckets

**Combining several labels: the lowest wins.** When a dimension has more than one label in a mode, its level is the **lowest** of them.

**Conceptual**
- Bucket = the lowest `conceptual` label.
- No labels at all → Not Ready Yet.

**Bug**
- Bucket = the lowest `strategy` label.
- No labels at all → Not Ready Yet.

**Code**
1. Each dimension's level is the lowest of its labels.
2. **Dimensions with no labels are ignored**; they neither raise nor lower the bucket.
3. Not Ready Yet if `correctness` is `not_ready`, or if 2 or more dimensions are `not_ready`.
4. Otherwise, Exceptional if every dimension with labels is `exceptional`.
5. Otherwise, Competent.
6. No labels in any dimension → Not Ready Yet.

## Step 3 — Overall rating

This is unchanged from today's `computeFinalRating`:
1. Any mode bucket is Not Ready Yet → **Not Ready Yet**.
2. Otherwise, any mode bucket is Competent → **Competent**.
3. Otherwise (every bucket Exceptional) → **Exceptional**.

**Week 1:** only Conceptual runs. Code and Bug are N/A, and the overall rating equals the Conceptual bucket.

## Step 4 — Results message

This is unchanged from today's `buildServerAssessmentResultsMessage`: the selected concept, the three buckets (N/A where a mode doesn't run), the overall rating, and the offer to switch to coach mode. Evidence for each bucket is explained in coaching, not here.

## Examples (also the test table for `grading.go`)

| # | Mode | Labels in order | Bucket | Why |
| --- | --- | --- | --- | --- |
| 1 | Conceptual | competent, exceptional, exceptional | Competent | Lowest |
| 2 | Conceptual | not_ready, not_ready, competent | Not Ready Yet | Lowest; recovering later doesn't help |
| 3 | Conceptual | exceptional, exceptional, exceptional | Exceptional | |
| 4 | Conceptual | vague (→ not_ready), then redirect answer exceptional, exceptional | Not Ready Yet | The vague answer counts |
| 5 | Conceptual | (none) | Not Ready Yet | No labels |
| 6 | Bug | strategy: competent, exceptional, competent | Competent | Lowest |
| 7 | Bug | strategy: exceptional ×4 | Exceptional | |
| 8 | Code | decomposition=exceptional; correctness=exceptional; understanding=exceptional; ai_use never discussed | Exceptional | Missing dimension ignored |
| 9 | Code | decomposition=exceptional; correctness=competent; understanding=exceptional; ai_use=exceptional | Competent | Not all Exceptional |
| 10 | Code | correctness=not_ready; all others exceptional | Not Ready Yet | Major gap |
| 11 | Code | decomposition=not_ready; understanding=not_ready; correctness=competent | Not Ready Yet | 2 Not Ready |
| 12 | Code | ai_use=not_ready; all others exceptional | Competent | 1 Not Ready, not correctness |
| 13 | Code | understanding: exceptional, then not_ready; others competent | Competent | `understanding` = not_ready (lowest); only 1 Not Ready |
| 14 | Code | no code pasted; decomposition=exceptional | Not Ready Yet | Go sets correctness=not_ready |

| # | Conceptual | Code | Bug | Week | Overall |
| --- | --- | --- | --- | --- | --- |
| 15 | Competent | Exceptional | Exceptional | 5 | Competent |
| 16 | Exceptional | Exceptional | Not Ready Yet | 5 | Not Ready Yet |
| 17 | Exceptional | Exceptional | Exceptional | 5 | Exceptional |
| 18 | Competent | N/A | N/A | 1 | Competent |
