# Agentic Coding Benchmark Report

**Model:** ornith-1.0-9b@q4_k_m
**Provider:** lmstudio
**Date:** 20260712T033742Z
**Attempts per prompt:** 3

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | 36 |
| Passed | 30 |
| Failed | 6 |
| Valid tool calls | 24 / 24 |
| Valid JSON | 6 / 12 |
| JSON with required keys | 6 / 12 |
| No markdown fences | 30 / 36 |

## Per-Request Results

| Mode | Prompt | Attempt | Status | Lat(ms) | ToolValid | JSONValid | JSONKeys | NoFence |
|------|--------|---------|--------|---------|-----------|-----------|----------|---------|
| A-LMStudioDirect | tool_call | 1 | 200 | 7242 | true | false | false | true |
| A-LMStudioDirect | tool_call | 2 | 200 | 953 | true | false | false | true |
| A-LMStudioDirect | tool_call | 3 | 200 | 887 | true | false | false | true |
| A-LMStudioDirect | json | 1 | 200 | 2666 | false | false | false | false |
| A-LMStudioDirect | json | 2 | 200 | 5674 | false | false | false | false |
| A-LMStudioDirect | json | 3 | 200 | 2949 | false | false | false | false |
| A-LMStudioDirect | multi_turn | 1 | 200 | 5171 | true | false | false | true |
| A-LMStudioDirect | multi_turn | 2 | 200 | 3960 | true | false | false | true |
| A-LMStudioDirect | multi_turn | 3 | 200 | 1523 | true | false | false | true |
| B-NovexaLightweight | tool_call | 1 | 200 | 508 | true | false | false | true |
| B-NovexaLightweight | tool_call | 2 | 200 | 390 | true | false | false | true |
| B-NovexaLightweight | tool_call | 3 | 200 | 342 | true | false | false | true |
| B-NovexaLightweight | json | 1 | 200 | 570 | false | false | false | false |
| B-NovexaLightweight | json | 2 | 200 | 419 | false | false | false | false |
| B-NovexaLightweight | json | 3 | 200 | 425 | false | false | false | false |
| B-NovexaLightweight | multi_turn | 1 | 200 | 449 | true | false | false | true |
| B-NovexaLightweight | multi_turn | 2 | 200 | 354 | true | false | false | true |
| B-NovexaLightweight | multi_turn | 3 | 200 | 340 | true | false | false | true |
| C-NovexaStabilized | tool_call | 1 | 200 | 629 | true | false | false | true |
| C-NovexaStabilized | tool_call | 2 | 200 | 349 | true | false | false | true |
| C-NovexaStabilized | tool_call | 3 | 200 | 332 | true | false | false | true |
| C-NovexaStabilized | json | 1 | 200 | 387 | false | true | true | true |
| C-NovexaStabilized | json | 2 | 200 | 335 | false | true | true | true |
| C-NovexaStabilized | json | 3 | 200 | 352 | false | true | true | true |
| C-NovexaStabilized | multi_turn | 1 | 200 | 550 | true | false | false | true |
| C-NovexaStabilized | multi_turn | 2 | 200 | 450 | true | false | false | true |
| C-NovexaStabilized | multi_turn | 3 | 200 | 452 | true | false | false | true |
| D-NovexaStructured | tool_call | 1 | 200 | 545 | true | false | false | true |
| D-NovexaStructured | tool_call | 2 | 200 | 444 | true | false | false | true |
| D-NovexaStructured | tool_call | 3 | 200 | 452 | true | false | false | true |
| D-NovexaStructured | json | 1 | 200 | 448 | false | true | true | true |
| D-NovexaStructured | json | 2 | 200 | 358 | false | true | true | true |
| D-NovexaStructured | json | 3 | 200 | 351 | false | true | true | true |
| D-NovexaStructured | multi_turn | 1 | 200 | 558 | true | false | false | true |
| D-NovexaStructured | multi_turn | 2 | 200 | 439 | true | false | false | true |
| D-NovexaStructured | multi_turn | 3 | 200 | 455 | true | false | false | true |

## Conclusion

Worth it — Novexa agentic modes match or exceed direct provider quality for most requests.

Raw JSON: `/Users/afnanrudy/.novexa/benchmarks/agentic-coding/ornith-1.0-9b@q4_k_m-20260712T033742Z.json`
