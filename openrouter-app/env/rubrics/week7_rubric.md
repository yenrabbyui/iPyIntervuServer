# Week 7 Assessment Rubric

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

You use for when you have a list and while when you don't. They're kind of the same.

**Issues:**

- vague and partially wrong
- for is for iterating over a known sequence (or range); while is for repeating until a condition is false
- does not give clear criteria (e.g. known number of items vs 'until user quits')
- saying they're the same shows misunderstanding

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

Use a for loop when you know how many times to iterate—like going through each item in a list or a range of numbers. Use a while loop when you're repeating until a condition becomes false, like when the user types 'quit' or until a value meets some condition. So for loops are for fixed sequences, while loops are for 'keep going until something changes.'

**Strengths:**

- correct distinction (known iteration vs until condition)
- gives concrete examples (list/range vs user quits)
- accurate and practical

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

Use a for loop when you're iterating over a known sequence—a list, a range(n), or anything you can step through one item at a time with a fixed number of iterations. Use a while loop when the number of iterations isn't known in advance and depends on a condition (e.g. 'repeat until the user enters Q' or 'repeat until score reaches 100'). In problem decomposition: if you can say 'do this for each X,' use for; if you say 'keep doing this until Y happens,' use while. Be careful with while—you need a condition that eventually becomes false or you get an infinite loop.

**Strengths:**

- clear criteria (known sequence vs condition-based)
- concrete examples (list, range vs user quits, score)
- connects to problem decomposition
- cautions about infinite loops

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
