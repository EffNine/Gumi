# Managed Thinking Benchmark Report

**Model:** qwen2.5-coder-7b-instruct
**Provider:** lmstudio
**Date:** 20260717T185425Z
**Attempts per prompt:** 1

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | 9 |
| Passed (non-empty + no leak) | 7 |
| Failed | 2 |
| Non-empty responses | 7 / 9 |
| No reasoning leak | 9 / 9 |

## Per-Request Results

| Mode | Prompt | Attempt | Status | Lat(ms) | NonEmpty | NoLeak | Len | Preview |
|------|--------|---------|--------|---------|----------|--------|-----|---------|
| A-LMStudioDirect-ThinkingOn | debugging | 1 | 200 | 4765 | true | true | 2265 | "The likely bug in the Go function is due to concurrent access and modification o" |
| A-LMStudioDirect-ThinkingOn | planning | 1 | 200 | 6642 | true | true | 3570 | "To refactor a 500-line monolithic HTTP handler into three smaller handlers while" |
| A-LMStudioDirect-ThinkingOn | json | 1 | 200 | 250 | true | true | 47 | "```json\n{\n  \"name\": \"test\",\n  \"value\": 42\n}\n```\n" |
| B-Gumi-ThinkingOn | debugging | 1 | 200 | 4927 | true | true | 976 | "The likely bug is a race condition. When multiple goroutines access and modify t" |
| B-Gumi-ThinkingOn | planning | 1 | 422 | 7637 | false | true | 0 | "\n" |
| B-Gumi-ThinkingOn | json | 1 | 200 | 238 | true | true | 29 | "{\"name\": \"test\", \"value\": 42}\n" |
| C-Gumi-ThinkingOff | debugging | 1 | 200 | 4084 | true | true | 807 | "The likely bug is race condition. When multiple goroutines access and modify the" |
| C-Gumi-ThinkingOff | planning | 1 | 422 | 4160 | false | true | 0 | "\n" |
| C-Gumi-ThinkingOff | json | 1 | 200 | 213 | true | true | 29 | "{\"name\": \"test\", \"value\": 42}\n" |

## Conclusion

Managed thinking infrastructure is working: responses are non-empty and no explicit reasoning markers (\<thinking\>, \<reasoning\>, fenced blocks) leak to the client.

**Important:** This benchmark only detects explicit reasoning/thinking markers. Local models may still emit reasoning prose inside the main content. Gumi strips reasoning when the provider returns it in a separate field (reasoning_content) or when the model wraps it in explicit markers. Models that emit reasoning as plain prose inside content are not fully stripped yet.

Raw JSON: `/home/dev/.gumi/benchmarks/managed-thinking/qwen2.5-coder-7b-instruct-20260717T185425Z.json`
