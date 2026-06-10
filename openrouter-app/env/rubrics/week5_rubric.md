# Week 5 Assessment Rubric

## Not Yet Ready

### Conceptual Answer

- significant errors about conditionals or loops
- confuses if/elif/else or when each applies
- cannot articulate when to use for vs while
- does not understand boolean expressions (and, or, not)
- fundamental misunderstanding of control flow

### Code Answer

- non-functional or major logic errors
- wrong or missing if/elif/else branches
- infinite loop or loop that never runs
- wrong comparison operators (e.g. = instead of ==)
- no clear menu or control structure
- completely misses the prompt

### AI Use Answer

- copying code without understanding
- cannot explain how they verified the AI's output
- unethical use or dependency that inhibits learning

### Example

**Rating:** Not Yet Ready

```python
if age = 18:
  print('adult')
else print('minor')
```

**Issues:**

- uses = instead of == (assignment not comparison)
- missing colon after else
- does not use >= for 18 or more
- syntax and logic errors

## Competent

### Conceptual Answer

- factually correct
- can describe if/elif/else and when each applies
- can describe boolean expressions and comparison operators
- can explain for vs while with reasonable criteria
- can describe menu loop (repeat until quit)
- may lack depth on edge cases or combining structures

### Code Answer

- complete and functional
- correct if/elif/else and loop logic
- menu repeats until user quits
- correct comparison operators (==, !=, etc.)
- may have long if/elif chains or minimal error handling

### AI Use Answer

- using AI for conditionals/loops, debugging, or explanation
- can articulate how they tested and integrated the suggestion
- healthy, supplemental use of the tool

### Example

**Rating:** Competent

```python
if age >= 18:
    print('adult')
else:
    print('minor')
```

**Strengths:**

- correct >= for 18 or more
- correct if/else with proper colons
- functional and clear

## Exceptional

### Conceptual Answer

- correct, clear, comprehensive
- concrete examples
- connects to problem decomposition (decision points, repetition, exit condition)
- may discuss combining conditionals and loops
- may discuss handling invalid input

### Code Answer

- complete, functional, clear
- well-structured conditionals and loops
- clear menu with explicit exit condition
- sensible handling of invalid input
- readable variable names and structure

### AI Use Answer

- AI to critique control flow, suggest conditions, or generate tests
- strategic use to deepen understanding
- improve structure and edge-case handling

### Example

**Rating:** Exceptional

```python
if age >= 18:
    print('adult')
else:
    print('minor')
# Edge case: if age could be invalid, we might check age >= 0 first.
```

**Strengths:**

- correct >= and if/else
- clear and readable
- brief comment on edge case

## Code Answer Integrated Dimensions

### Decomposition

| Level | Guidance |
| --- | --- |
| Not Ready Yet | Cannot break problem into meaningful parts or gives no rationale |
| Competent | Breaks problem into reasonable parts with some justification |
| Exceptional | Clear, well-structured decomposition with strong justification and awareness of alternatives |

### Implementation Correctness

| Level | Guidance |
| --- | --- |
| Not Ready Yet | Code does not run or fails core requirements |
| Competent | Code runs and solves main problem with minor issues |
| Exceptional | Code is correct, robust, and handles edge cases appropriately |

### Code Understanding

| Level | Guidance |
| --- | --- |
| Not Ready Yet | Cannot explain key lines or logic |
| Competent | Can explain general flow and most lines |
| Exceptional | Can clearly explain specific lines, control flow, and data transformations |

### AI Use Reasoning

| Level | Guidance |
| --- | --- |
| Not Ready Yet | Cannot describe how AI was used or relies blindly on output |
| Competent | Describes basic AI use and some verification |
| Exceptional | Demonstrates intentional AI use, validation, and thoughtful modification |

## Overall Guidance

| Level | Guidance |
| --- | --- |
| Not Ready Yet | Weakness in multiple dimensions or major gap in understanding |
| Competent | Solid correctness with reasonable understanding and decomposition |
| Exceptional | Strong across all dimensions including deep understanding and intentional AI use |
