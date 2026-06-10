# Week 9 Assessment Rubric

## Not Yet Ready

### Conceptual Answer

- significant errors about lists or dictionaries
- confuses list vs dict or indexing/key access
- does not understand file open/read/close or with
- cannot articulate when to use list vs dict
- fundamental misunderstanding of data processing from files

### Code Answer

- non-functional or major syntax/logic errors
- wrong list or dict usage (e.g. wrong indexing or key access)
- file not opened correctly (e.g. write mode when reading)
- no with statement or file not closed
- does not process or format file data correctly
- completely misses the prompt

### AI Use Answer

- copying code without understanding
- cannot explain how they verified the AI's output
- unethical use or dependency that inhibits learning

### Example

**Rating:** Not Yet Ready

```python
first = items(0)
last = items(-1)
```

**Issues:**

- uses items(0) instead of items[0]—confuses function call with indexing
- items(-1) same error—indexing uses square brackets
- fundamental confusion between list indexing and function syntax

## Competent

### Conceptual Answer

- factually correct
- can describe list vs dict and when to use each
- can describe file open/read/readlines and with
- can describe basic data processing (loop, look up)
- may lack depth on edge cases or efficient patterns

### Code Answer

- complete and functional
- correct list/dict usage
- correct file open and read (with open, readlines or read)
- correct processing (loop, convert, aggregate)
- correct formatted output
- may have minimal error handling or comments

### AI Use Answer

- using AI for file I/O, data structures, debugging, or explanation
- can articulate how they tested and integrated the suggestion
- healthy, supplemental use of the tool

### Example

**Rating:** Competent

```python
first = items[0]
last = items[-1]
```

**Strengths:**

- correct indexing with square brackets
- items[0] for first, items[-1] for last
- functional and correct

## Exceptional

### Conceptual Answer

- correct, clear, comprehensive
- concrete examples
- connects to problem decomposition (input file, process, output)
- may discuss choosing list vs dict
- may discuss empty file or bad data handling

### Code Answer

- complete, functional, clear
- appropriate list/dict usage
- correct file handling (with open, clear reading)
- clear processing and variable names
- readable formatted output
- may handle empty file or invalid lines

### AI Use Answer

- AI to critique data structures, file handling, or generate test files
- strategic use to deepen understanding
- improve code quality

### Example

**Rating:** Exceptional

```python
first = items[0]   # first element (index 0)
last = items[-1]   # last element (negative index)
```

**Strengths:**

- correct indexing [0] and [-1]
- brief comments clarify meaning
- clear and correct

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
