# Hint Profile Analysis — Sprint 17R3

**Date:** 2026-08-10  
**Sprint:** 17R3

---

## Profile Distribution by GEP Suite

### Instruction Following Easy (5 tests)

| Test | Constraints | Score | Profile | Hint Tokens |
|------|------------|-------|---------|-------------|
| inst-easy-01 (2 sentences + forbidden word) | 2 | 2 | STANDARD | ~20 |
| inst-easy-02 (end with specific word) | 1 | 1 | MINIMAL | ~8 |
| inst-easy-03 (single word answer) | 1-2 | 1-2 | MINIMAL/STANDARD | ~5-10 |
| inst-easy-04 (bullet list) | 1 | 1 | MINIMAL | ~5 |
| inst-easy-05 (yes/no answer) | 1 | 1 | MINIMAL | ~5 |

**Average hint tokens (Easy): ~8**

### Instruction Following Medium (5 tests)

| Test | Constraints | Score | Profile | Hint Tokens |
|------|------------|-------|---------|-------------|
| inst-med-01 (3 sentences + forbidden + capital + end) | 4 | 4 | EXPLICIT | ~50 |
| inst-med-02 (bullets + forbidden + chars + year) | 3 | 3 | STANDARD | ~35 |
| inst-med-03 (JSON + keys + no markdown) | 2-3 | 3-4 | STANDARD/EXPLICIT | ~25-40 |
| inst-med-04 (min/max chars + forbidden + required) | 3 | 3 | STANDARD | ~35 |
| inst-med-05 (lines + commas + chars + numbers) | 3-4 | 3-4 | STANDARD/EXPLICIT | ~35-50 |

**Average hint tokens (Medium): ~35**

### Structured Output Easy (5 tests)

| Test | Constraints | Score | Profile | Hint Tokens |
|------|------------|-------|---------|-------------|
| struct-easy-01 (JSON object) | 1-2 | 1-2 | MINIMAL/STANDARD | ~5-10 |
| struct-easy-02 (JSON array) | 1 | 1 | MINIMAL | ~5 |
| struct-easy-03 (nested JSON) | 1 | 1 | MINIMAL | ~5 |
| struct-easy-04 (tabular JSON) | 1 | 1 | MINIMAL | ~5 |
| struct-easy-05 (enum JSON) | 1 | 1 | MINIMAL | ~5 |

**Average hint tokens (Structured Easy): ~5**

### Consistency Easy (5 tests)

| Test | Constraints | Score | Profile | Hint Tokens |
|------|------------|-------|---------|-------------|
| cons-easy-01 to 05 | 0 | 0 | NONE | 0 |

All consistency tests are simple factual/math questions with no formatting constraints.

**Average hint tokens (Consistency): 0**

### Context Retention Easy (5 tests)

| Test | Constraints | Score | Profile | Hint Tokens |
|------|------------|-------|---------|-------------|
| ctx-easy-01 to 05 | 0-1 | 0-1 | NONE/MINIMAL | 0-5 |

Most context tests have no hard constraints (simple multi-turn Q&A).

**Average hint tokens (Context Easy): ~1**

---

## Token Budget Analysis

### Sprint 17 vs 17R2 vs 17R3

| Prompt Type | Sprint 17 | Sprint 17R2 | Sprint 17R3 |
|-------------|-----------|-------------|-------------|
| Simple question | ~180 tokens | 0 | 0 |
| Single constraint | ~120 tokens | ~8 | ~8 |
| 2-3 constraints | ~250 tokens | ~20 | ~25 |
| 4+ constraints | ~300 tokens | ~40 | ~50 |
| JSON only | ~200 tokens | ~5 | ~5 |
| JSON + others | ~250 tokens | ~15 | ~30 |

### Hint Token Counts by Profile

| Profile | Min Tokens | Max Tokens | Typical |
|---------|-----------|------------|---------|
| NONE | 0 | 0 | 0 |
| MINIMAL | 3 | 15 | 8 |
| STANDARD | 10 | 40 | 25 |
| EXPLICIT | 25 | 80 | 50 |

Hard upper bound: 100 tokens (documented exceptional cases only).

---

## Overhead Ratio

Instruction overhead = hint tokens / original prompt tokens

| Scenario | Original Tokens | Hint Tokens | Overhead |
|----------|----------------|-------------|----------|
| Simple question | ~20 | 0 | 0% |
| Single constraint | ~30 | ~8 | 27% |
| Multi-constraint (2-3) | ~50 | ~25 | 50% |
| Complex (4+) | ~80 | ~50 | 63% |
| JSON only | ~40 | ~5 | 13% |
| JSON + constraints | ~60 | ~30 | 50% |

Compared to Sprint 17 (400-900% overhead), Sprint 17R3 maintains the 60-80% reduction.

---

## Profile Selection Determinism

The same prompt always produces the same profile and score:

```
"Answer in exactly 2 sentences." → MINIMAL, score=1
"Explain Go in exactly 2 sentences. Do not use 'programming'." → STANDARD, score=2
"Return JSON with name and age. No markdown." → STANDARD, score=3 (JSON bonus)
"3 sentences, no 'tech', capital start, end with 'future'" → EXPLICIT, score=4
```

No randomness, no model-dependent routing, no external state.

---

## Comparison: Minimal vs Standard vs Explicit

### Single Constraint (MINIMAL)

```
Prompt: "Answer in exactly 2 sentences."
MINIMAL: exactly 2 sentences
STANDARD: exactly 2 sentences: exactly 2 sentences
EXPLICIT: 1. exactly 2 sentences: exactly 2 sentences
          verify all requirements before responding
```

For single constraints, MINIMAL is sufficient. Both Qwen3 8B and Gemma 3 4B handle it.

### Two Constraints (STANDARD)

```
Prompt: "Explain Go in exactly 2 sentences. Do not use 'programming'."
MINIMAL: exactly 2 sentences
         no 'programming'
STANDARD: exactly 2 sentences: exactly 2 sentences
          do not use 'programming': no 'programming'
EXPLICIT: 1. exactly 2 sentences: exactly 2 sentences
          2. do not use 'programming': no 'programming'
          verify all requirements before responding
```

STANDARD provides clearer grouping without the verbosity of EXPLICIT.

### Four Constraints (EXPLICIT)

```
Prompt: "3 sentences, no 'technology', capital start, end with 'future'"
STANDARD: exactly 3 sentences: exactly 3 sentences
          do not use 'technology': no 'technology'
          start with capital: each line starts with capital
          end with 'future': end with 'future'
EXPLICIT: 1. exactly 3 sentences: exactly 3 sentences
          2. do not use 'technology': no 'technology'
          3. start with capital: each line starts with capital
          4. end with 'future': end with 'future'
          verify all requirements before responding
```

EXPLICIT adds numbering and verification for high-complexity cases where Gemma 3 4B needs extra guidance.
