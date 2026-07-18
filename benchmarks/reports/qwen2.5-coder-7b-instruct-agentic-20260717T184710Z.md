# Agentic Coding Benchmark Report

**Model:** qwen2.5-coder-7b-instruct
**Provider:** lmstudio
**Date:** 20260717T184710Z
**Attempts per prompt:** 3

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | 36 |
| Passed | 24 |
| Failed | 12 |
| Valid tool calls | 18 / 24 |
| Valid JSON | 6 / 12 |
| JSON with required keys | 6 / 12 |
| No markdown fences | 24 / 36 |

## Per-Request Results

| Mode | Prompt | Attempt | Status | Lat(ms) | ToolValid | JSONValid | JSONKeys | NoFence |
|------|--------|---------|--------|---------|-----------|-----------|----------|---------|
| A-LMStudioDirect | tool_call | 1 | 200 | 63633 | false | false | false | false |
| A-LMStudioDirect | tool_call | 2 | 200 | 64923 | false | false | false | false |
| A-LMStudioDirect | tool_call | 3 | 200 | 52920 | false | false | false | false |
| A-LMStudioDirect | json | 1 | 200 | 47871 | false | false | false | false |
| A-LMStudioDirect | json | 2 | 200 | 37555 | false | false | false | false |
| A-LMStudioDirect | json | 3 | 200 | 37519 | false | false | false | false |
| A-LMStudioDirect | multi_turn | 1 | 200 | 48022 | false | false | false | false |
| A-LMStudioDirect | multi_turn | 2 | 200 | 49457 | false | false | false | false |
| A-LMStudioDirect | multi_turn | 3 | 200 | 37703 | false | false | false | false |
| B-GumiLightweight | tool_call | 1 | 200 | 29483 | true | false | false | true |
| B-GumiLightweight | tool_call | 2 | 200 | 30659 | true | false | false | true |
| B-GumiLightweight | tool_call | 3 | 200 | 30811 | true | false | false | true |
| B-GumiLightweight | json | 1 | 200 | 35917 | false | false | false | false |
| B-GumiLightweight | json | 2 | 200 | 37463 | false | false | false | false |
| B-GumiLightweight | json | 3 | 200 | 37496 | false | false | false | false |
| B-GumiLightweight | multi_turn | 1 | 200 | 29385 | true | false | false | true |
| B-GumiLightweight | multi_turn | 2 | 200 | 30672 | true | false | false | true |
| B-GumiLightweight | multi_turn | 3 | 200 | 30818 | true | false | false | true |
| C-GumiStabilized | tool_call | 1 | 200 | 29726 | true | false | false | true |
| C-GumiStabilized | tool_call | 2 | 200 | 30844 | true | false | false | true |
| C-GumiStabilized | tool_call | 3 | 200 | 30673 | true | false | false | true |
| C-GumiStabilized | json | 1 | 200 | 22661 | false | true | true | true |
| C-GumiStabilized | json | 2 | 200 | 23854 | false | true | true | true |
| C-GumiStabilized | json | 3 | 200 | 30742 | false | true | true | true |
| C-GumiStabilized | multi_turn | 1 | 200 | 29409 | true | false | false | true |
| C-GumiStabilized | multi_turn | 2 | 200 | 30800 | true | false | false | true |
| C-GumiStabilized | multi_turn | 3 | 200 | 37508 | true | false | false | true |
| D-GumiStructured | tool_call | 1 | 200 | 29547 | true | false | false | true |
| D-GumiStructured | tool_call | 2 | 200 | 30772 | true | false | false | true |
| D-GumiStructured | tool_call | 3 | 200 | 30927 | true | false | false | true |
| D-GumiStructured | json | 1 | 200 | 22493 | false | true | true | true |
| D-GumiStructured | json | 2 | 200 | 30642 | false | true | true | true |
| D-GumiStructured | json | 3 | 200 | 30632 | false | true | true | true |
| D-GumiStructured | multi_turn | 1 | 200 | 29493 | true | false | false | true |
| D-GumiStructured | multi_turn | 2 | 200 | 30719 | true | false | false | true |
| D-GumiStructured | multi_turn | 3 | 200 | 30739 | true | false | false | true |

## Conclusion

Worth it — Gumi agentic modes match or exceed direct provider quality for most requests.

Raw JSON: `/home/dev/.gumi/benchmarks/agentic-coding/qwen2.5-coder-7b-instruct-20260717T184710Z.json`
