# Benchmark Report

**Model:** qwen2.5-coder-7b-instruct
**Provider:** lmstudio
**Date:** 2026-07-17T18:26:03Z
**Attempts per prompt:** 3
**Modes tested:** A-LMStudioDirect, B-GumiDirect, C-GumiLightweight, D-GumiStabilized, E-GumiStructured

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | 39 |
| Passed | 33 |
| Failed | 6 |
| Empty responses | 0 |
| HTTP/curl errors | 0 |
| Exact instruction following | 24 / 24 |
| Valid JSON (all JSON prompts) | 9 / 15 |
| JSON with required keys | 9 / 15 |
| No markdown fences | 33 / 39 |

## Per-Mode Latency

| Mode | p50 (ms) | p95 (ms) | Count |
|------|----------|----------|-------|
| A-LMStudioDirect | 3738 | 37560 | 9 |
| B-GumiDirect | 1996 | 38004 | 9 |
| C-GumiLightweight | 3518 | 36629 | 9 |
| D-GumiStabilized | 3555 | 24055 | 9 |
| E-GumiStructured | 30674 | 30706 | 3 |

## Per-Request Results

| Mode | Prompt | # | Status | Lat(ms) | Pass | Exact | JSON | Keys | NoFence | Note |
|------|--------|---|--------|---------|------|-------|------|------|---------|------|
| A-LMStudioDirect | concise | 1 | 200 | 1957 | true | true | false | false | true | ok (1 chars) |
| A-LMStudioDirect | factual | 1 | 200 | 3761 | true | true | false | false | true | ok (6 chars) |
| A-LMStudioDirect | json | 1 | 200 | 37494 | false | false | false | false | false | ok (47 chars) |
| A-LMStudioDirect | concise | 2 | 200 | 1967 | true | true | false | false | true | ok (1 chars) |
| A-LMStudioDirect | factual | 2 | 200 | 1964 | true | true | false | false | true | ok (5 chars) |
| A-LMStudioDirect | json | 2 | 200 | 37560 | false | false | false | false | false | ok (47 chars) |
| A-LMStudioDirect | concise | 3 | 200 | 1965 | true | true | false | false | true | ok (1 chars) |
| A-LMStudioDirect | factual | 3 | 200 | 3738 | true | true | false | false | true | ok (6 chars) |
| A-LMStudioDirect | json | 3 | 200 | 37342 | false | false | false | false | false | ok (47 chars) |
| B-GumiDirect | concise | 1 | 200 | 1998 | true | true | false | false | true | ok (1 chars) |
| B-GumiDirect | factual | 1 | 200 | 1952 | true | true | false | false | true | ok (5 chars) |
| B-GumiDirect | json | 1 | 200 | 37237 | true | false | true | true | true | ok (26 chars) |
| B-GumiDirect | concise | 2 | 200 | 1959 | true | true | false | false | true | ok (1 chars) |
| B-GumiDirect | factual | 2 | 200 | 1948 | true | true | false | false | true | ok (5 chars) |
| B-GumiDirect | json | 2 | 200 | 38004 | true | false | true | true | true | ok (26 chars) |
| B-GumiDirect | concise | 3 | 200 | 1996 | true | true | false | false | true | ok (1 chars) |
| B-GumiDirect | factual | 3 | 200 | 1994 | true | true | false | false | true | ok (5 chars) |
| B-GumiDirect | json | 3 | 200 | 37087 | true | false | true | true | true | ok (26 chars) |
| C-GumiLightweight | concise | 1 | 200 | 2273 | true | true | false | false | true | ok (1 chars) |
| C-GumiLightweight | factual | 1 | 200 | 2097 | true | true | false | false | true | ok (5 chars) |
| C-GumiLightweight | json | 1 | 200 | 36629 | false | false | false | false | false | ok (47 chars) |
| C-GumiLightweight | concise | 2 | 200 | 3518 | true | true | false | false | true | ok (1 chars) |
| C-GumiLightweight | factual | 2 | 200 | 2002 | true | true | false | false | true | ok (5 chars) |
| C-GumiLightweight | json | 2 | 200 | 36030 | false | false | false | false | false | ok (47 chars) |
| C-GumiLightweight | concise | 3 | 200 | 3523 | true | true | false | false | true | ok (1 chars) |
| C-GumiLightweight | factual | 3 | 200 | 1993 | true | true | false | false | true | ok (5 chars) |
| C-GumiLightweight | json | 3 | 200 | 36198 | false | false | false | false | false | ok (47 chars) |
| D-GumiStabilized | concise | 1 | 200 | 2216 | true | true | false | false | true | ok (1 chars) |
| D-GumiStabilized | factual | 1 | 200 | 2177 | true | true | false | false | true | ok (5 chars) |
| D-GumiStabilized | json | 1 | 200 | 22707 | true | false | true | true | true | ok (29 chars) |
| D-GumiStabilized | concise | 2 | 200 | 3478 | true | true | false | false | true | ok (1 chars) |
| D-GumiStabilized | factual | 2 | 200 | 3612 | true | true | false | false | true | ok (5 chars) |
| D-GumiStabilized | json | 2 | 200 | 24052 | true | false | true | true | true | ok (29 chars) |
| D-GumiStabilized | concise | 3 | 200 | 3458 | true | true | false | false | true | ok (1 chars) |
| D-GumiStabilized | factual | 3 | 200 | 3555 | true | true | false | false | true | ok (5 chars) |
| D-GumiStabilized | json | 3 | 200 | 24055 | true | false | true | true | true | ok (29 chars) |
| E-GumiStructured | json | 1 | 200 | 17618 | true | false | true | true | true | ok (26 chars) |
| E-GumiStructured | json | 2 | 200 | 30674 | true | false | true | true | true | ok (35 chars) |
| E-GumiStructured | json | 3 | 200 | 30706 | true | false | true | true | true | ok (35 chars) |

## Conclusion

Worth it — Gumi modes match or exceed direct provider quality with acceptable latency overhead.
