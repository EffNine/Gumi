# Agentic Coding Benchmark Report

**Model:** qwen/qwen3.5-9b
**Provider:** lmstudio
**Date:** 20260712T041101Z
**Attempts per prompt:** 3

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | 36 |
| Passed | 31 |
| Failed | 5 |
| Valid tool calls | 24 / 24 |
| Valid JSON | 7 / 12 |
| JSON with required keys | 7 / 12 |
| No markdown fences | 31 / 36 |

## Per-Request Results

| Mode | Prompt | Attempt | Status | Lat(ms) | ToolValid | JSONValid | JSONKeys | NoFence |
|------|--------|---------|--------|---------|-----------|-----------|----------|---------|
| A-LMStudioDirect | tool_call | 1 | 200 | 14378 | true | false | false | true |
| A-LMStudioDirect | tool_call | 2 | 200 | 960 | true | false | false | true |
| A-LMStudioDirect | tool_call | 3 | 200 | 963 | true | false | false | true |
| A-LMStudioDirect | json | 1 | 200 | 5879 | false | true | true | true |
| A-LMStudioDirect | json | 2 | 200 | 12314 | false | false | false | false |
| A-LMStudioDirect | json | 3 | 200 | 5815 | false | false | false | false |
| A-LMStudioDirect | multi_turn | 1 | 200 | 6904 | true | false | false | true |
| A-LMStudioDirect | multi_turn | 2 | 200 | 4698 | true | false | false | true |
| A-LMStudioDirect | multi_turn | 3 | 200 | 4290 | true | false | false | true |
| B-NovexaLightweight | tool_call | 1 | 200 | 469 | true | false | false | true |
| B-NovexaLightweight | tool_call | 2 | 200 | 377 | true | false | false | true |
| B-NovexaLightweight | tool_call | 3 | 200 | 445 | true | false | false | true |
| B-NovexaLightweight | json | 1 | 200 | 548 | false | false | false | false |
| B-NovexaLightweight | json | 2 | 200 | 462 | false | false | false | false |
| B-NovexaLightweight | json | 3 | 200 | 480 | false | false | false | false |
| B-NovexaLightweight | multi_turn | 1 | 200 | 447 | true | false | false | true |
| B-NovexaLightweight | multi_turn | 2 | 200 | 348 | true | false | false | true |
| B-NovexaLightweight | multi_turn | 3 | 200 | 450 | true | false | false | true |
| C-NovexaStabilized | tool_call | 1 | 200 | 650 | true | false | false | true |
| C-NovexaStabilized | tool_call | 2 | 200 | 343 | true | false | false | true |
| C-NovexaStabilized | tool_call | 3 | 200 | 347 | true | false | false | true |
| C-NovexaStabilized | json | 1 | 200 | 386 | false | true | true | true |
| C-NovexaStabilized | json | 2 | 200 | 260 | false | true | true | true |
| C-NovexaStabilized | json | 3 | 200 | 313 | false | true | true | true |
| C-NovexaStabilized | multi_turn | 1 | 200 | 508 | true | false | false | true |
| C-NovexaStabilized | multi_turn | 2 | 200 | 337 | true | false | false | true |
| C-NovexaStabilized | multi_turn | 3 | 200 | 334 | true | false | false | true |
| D-NovexaStructured | tool_call | 1 | 200 | 508 | true | false | false | true |
| D-NovexaStructured | tool_call | 2 | 200 | 444 | true | false | false | true |
| D-NovexaStructured | tool_call | 3 | 200 | 348 | true | false | false | true |
| D-NovexaStructured | json | 1 | 200 | 444 | false | true | true | true |
| D-NovexaStructured | json | 2 | 200 | 272 | false | true | true | true |
| D-NovexaStructured | json | 3 | 200 | 278 | false | true | true | true |
| D-NovexaStructured | multi_turn | 1 | 200 | 503 | true | false | false | true |
| D-NovexaStructured | multi_turn | 2 | 200 | 451 | true | false | false | true |
| D-NovexaStructured | multi_turn | 3 | 200 | 453 | true | false | false | true |

## Conclusion

Worth it — Novexa agentic modes match or exceed direct provider quality for most requests.

Raw JSON: `/Users/afnanrudy/.novexa/benchmarks/agentic-coding/qwen_qwen3.5-9b-20260712T041101Z.json`
