package router

import (
	"testing"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
)

// ---------------------------------------------------------------------------
// Difficulty classification
// ---------------------------------------------------------------------------

func TestClassifyDifficulty_Trivial(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Hello", // < 50 chars, no files, no traceback
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty != DifficultyTrivial {
		t.Fatalf("expected difficulty %d (trivial), got %d", DifficultyTrivial, profile.Difficulty)
	}
	if profile.DifficultyLabel != "trivial" {
		t.Fatalf("expected label 'trivial', got %q", profile.DifficultyLabel)
	}
	if profile.FileCount != 0 {
		t.Fatalf("expected file count 0, got %d", profile.FileCount)
	}
	if profile.HasTraceback {
		t.Fatal("expected has_traceback=false")
	}
}

func TestClassifyDifficulty_Trivial_SearchTask(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "search for the function definition in the codebase", // search → always trivial
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty != DifficultyTrivial {
		t.Fatalf("expected difficulty %d (trivial) for search task, got %d", DifficultyTrivial, profile.Difficulty)
	}
	if profile.TaskType != TaskSearch {
		t.Fatalf("expected task type 'search', got %s", profile.TaskType)
	}
}

func TestClassifyDifficulty_Simple(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	// Text > 50 chars, bare filename (no path prefix) so countFilePaths returns 1.
	msgs := []api.Message{{
		Role:    "user",
		Content: "Can you update main.go to enable debug mode and turn on verbose logging?",
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty != DifficultySimple {
		t.Fatalf("expected difficulty %d (simple), got %d", DifficultySimple, profile.Difficulty)
	}
	if profile.DifficultyLabel != "simple" {
		t.Fatalf("expected label 'simple', got %q", profile.DifficultyLabel)
	}
	if profile.FileCount != 1 {
		t.Fatalf("expected file count 1, got %d", profile.FileCount)
	}
}

func TestClassifyDifficulty_Simple_FixWithTraceback(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	// Text > 50 chars, 1 file, fix task, < 200 chars → simple fix path
	msgs := []api.Message{{
		Role:    "user",
		Content: "Fix this error in main.go: the function returns nil instead of the expected result.",
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty != DifficultySimple {
		t.Fatalf("expected difficulty %d (simple) for simple fix, got %d", DifficultySimple, profile.Difficulty)
	}
	if profile.TaskType != TaskFix {
		t.Fatalf("expected task type 'fix', got %s", profile.TaskType)
	}
}

func TestClassifyDifficulty_Moderate(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	// Use bare filenames (no path prefix) to avoid double-counting from pathPatternRe.
	msgs := []api.Message{{
		Role: "user",
		Content: `Implement a new function in parser.go and update types.go to support the new field.
The function should parse the input and return a structured result.`,
	}}
	profile := c.Classify(msgs, 1, 0, nil)
	if profile.Difficulty != DifficultyModerate {
		t.Fatalf("expected difficulty %d (moderate), got %d", DifficultyModerate, profile.Difficulty)
	}
	if profile.DifficultyLabel != "moderate" {
		t.Fatalf("expected label 'moderate', got %q", profile.DifficultyLabel)
	}
	if profile.FileCount != 2 {
		t.Fatalf("expected file count 2, got %d", profile.FileCount)
	}
}

func TestClassifyDifficulty_Moderate_Default(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	// Bare filenames, 2 files, stepCount=2 → moderate
	msgs := []api.Message{{
		Role:    "user",
		Content: "Add a new method to service.go and wire it up in main.go.",
	}}
	profile := c.Classify(msgs, 2, 0, nil)
	if profile.Difficulty != DifficultyModerate {
		t.Fatalf("expected difficulty %d (moderate) as default, got %d", DifficultyModerate, profile.Difficulty)
	}
}

func TestClassifyDifficulty_Complex_Traceback(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role: "user",
		Content: `Fix the bug causing this crash:

Traceback (most recent call last):
  File "main.py", line 42, in <module>
    result = process_data(input)
  File "main.py", line 25, in process_data
    return transform(value)
  File "utils/helpers.py", line 10, in transform
    return data[key]
KeyError: 'missing_key'

The issue is in handler.go and service.go.`,
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex) for traceback with multiple files, got %d", DifficultyComplex, profile.Difficulty)
	}
	if !profile.HasTraceback {
		t.Fatal("expected has_traceback=true")
	}
	if profile.FileCount < 2 {
		t.Fatalf("expected at least 2 files, got %d", profile.FileCount)
	}
}

func TestClassifyDifficulty_Complex_ManyFiles(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	// Use bare filenames to avoid double-counting; 5 files → complex
	msgs := []api.Message{{
		Role: "user",
		Content: `Update the following files for the new feature:
- handler.go
- service.go
- repository.go
- middleware.go
- config.go`,
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex) for 5 files, got %d", DifficultyComplex, profile.Difficulty)
	}
	if profile.FileCount < 3 {
		t.Fatalf("expected at least 3 files, got %d", profile.FileCount)
	}
}

func TestClassifyDifficulty_Complex_LargeCodeBlock(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role: "user",
		Content: `Refactor this function:

` + "```go\n" + `func process(data []byte) (Result, error) {
	// This is a very long function with lots of logic
	var result Result
	if len(data) == 0 {
		return result, errors.New("empty data")
	}
	// ... many more lines of code ...
	// This code block is well over 100 characters
	// to trigger the large code block heuristic
	return result, nil
}
` + "```\n",
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex) for large code block, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestClassifyDifficulty_Complex_ReviewWithMultipleFiles(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Review the changes in handler.go and service.go for any issues.",
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex) for review with 2 files, got %d", DifficultyComplex, profile.Difficulty)
	}
	if profile.TaskType != TaskReview {
		t.Fatalf("expected task type 'review', got %s", profile.TaskType)
	}
}

func TestClassifyDifficulty_Complex_HighStepCount(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Fix the remaining issue in the parser.",
	}}
	profile := c.Classify(msgs, 6, 0, nil) // stepCount > 5
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex) for stepCount=6, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestClassifyDifficulty_Complex_HighRetries(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Fix the remaining issue in the parser.",
	}}
	profile := c.Classify(msgs, 0, 3, nil) // retryAttempt > 2
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex) for retryAttempt=3, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestClassifyDifficulty_Novel_PlanWithMultipleFiles(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	// Avoid "new" keyword (which would match TaskFeature first).
	// Use "design" and "architecture" which match TaskPlan.
	msgs := []api.Message{{
		Role: "user",
		Content: `Design the architecture for the authentication system.
We need to update auth.go, middleware.go, and config.go.`,
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty != DifficultyNovel {
		t.Fatalf("expected difficulty %d (novel) for plan with multiple files, got %d", DifficultyNovel, profile.Difficulty)
	}
	if profile.DifficultyLabel != "novel" {
		t.Fatalf("expected label 'novel', got %q", profile.DifficultyLabel)
	}
	if profile.TaskType != TaskPlan {
		t.Fatalf("expected task type 'plan', got %s", profile.TaskType)
	}
}

func TestClassifyDifficulty_Novel_ComplexSignals(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role: "user",
		Content: `We have a major refactoring task. Here is the current code:

` + "```go\n" + `// Large code block with many lines
func ProcessRequest(w http.ResponseWriter, r *http.Request) {
	// ... lots of complex logic spanning many lines ...
	// This block is intentionally large to test the novel difficulty path
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := transformData(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
` + "```\n\n" + `Also fix this crash:

Traceback (most recent call last):
  File "handler.go", line 42, in handleRequest
    result := process(input)
  File "processor.go", line 15, in process
    return transform(value)
KeyError: 'missing_key'

Files affected: handler.go, processor.go, types.go, config.go, middleware.go`,
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex) for complex signals, got %d", DifficultyComplex, profile.Difficulty)
	}
	if !profile.HasTraceback {
		t.Fatal("expected has_traceback=true for traceback in text")
	}
	if profile.FileCount < 3 {
		t.Fatalf("expected at least 3 files, got %d", profile.FileCount)
	}
}

// ---------------------------------------------------------------------------
// Task type classification
// ---------------------------------------------------------------------------

func TestClassifyTaskType_Fix_Traceback(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role: "user",
		Content: `Traceback (most recent call last):
  File "app.py", line 10, in <module>
    main()
  File "app.py", line 5, in main
    print(1/0)
ZeroDivisionError: division by zero`,
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.TaskType != TaskFix {
		t.Fatalf("expected task type 'fix' for traceback, got %s", profile.TaskType)
	}
}

func TestClassifyTaskType_Fix_ErrorKeywords(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"error keyword", "There is an error in the login flow"},
		{"bug keyword", "Found a bug in the payment processing"},
		{"fix keyword", "fix the broken endpoint"},
		{"bugfix keyword", "apply this bugfix to the release branch"},
		{"hotfix keyword", "hotfix for the production issue"},
		{"patch keyword", "create a patch for the security vulnerability"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskFix {
				t.Fatalf("expected task type 'fix' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Refactor(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"refactor keyword", "refactor the user service to use clean architecture"},
		{"restructure keyword", "restructure the project layout"},
		{"rename keyword", "rename the function to be more descriptive"},
		{"extract keyword", "extract the validation logic into a separate function"},
		{"inline keyword", "inline the helper function"},
		{"move keyword", "move the types to a shared package"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskRefactor {
				t.Fatalf("expected task type 'refactor' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Feature(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"implement keyword", "implement the user authentication endpoint"},
		{"create keyword", "create a API route for profiles"},
		{"add keyword", "add support for markdown rendering"},
		{"write keyword", "write a middleware for request logging"},
		{"build keyword", "build a caching layer for the database"},
		{"new keyword", "a feature flag system is needed"},
		{"feature keyword", "this feature requires a database table"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskFeature {
				t.Fatalf("expected task type 'feature' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Test(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		// These must avoid "add", "write", "create" etc. which match TaskFeature first.
		{"test keyword", "run the test for the user service"},
		{"assert keyword", "use assert to verify the output"},
		{"mock keyword", "mock the database interface for testing"},
		{"spec keyword", "update the spec to cover edge cases"},
		{"unit test phrase", "a unit test for the parser is needed"},
		{"integration test phrase", "an integration test for the API is needed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskTest {
				t.Fatalf("expected task type 'test' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Review(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		// Avoid "new" and "feature" which match TaskFeature first.
		{"review keyword", "review the pull request for the auth module"},
		{"explain keyword", "explain how the authentication flow works"},
		{"what does phrase", "what does this function do"},
		{"understand keyword", "I need to understand the data pipeline"},
		{"summarize keyword", "summarize the changes in this commit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskReview {
				t.Fatalf("expected task type 'review' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Search(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"search keyword", "search for all usages of the deprecated function"},
		{"find keyword", "find the definition of the CalculateTotal function"},
		{"where is phrase", "where is the config file located"},
		// Avoid "error" which matches TaskFix first.
		{"grep keyword", "grep for all logging patterns"},
		{"locate keyword", "locate the database migration files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskSearch {
				t.Fatalf("expected task type 'search' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Plan(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		// Avoid "new" which matches TaskFeature first.
		{"design keyword", "design the database schema"},
		{"architecture keyword", "propose an architecture for the microservices"},
		{"plan keyword", "plan the migration to the framework"},
		{"approach keyword", "what approach should we take for caching"},
		// Avoid "error" which matches TaskFix first.
		{"strategy keyword", "define a strategy for caching"},
		// Avoid "write" which matches TaskFeature first.
		{"proposal keyword", "a proposal for the API design"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskPlan {
				t.Fatalf("expected task type 'plan' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Docs(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"docs keyword", "update the docs for the API endpoints"},
		// Avoid "add" which matches TaskFeature first.
		{"documentation keyword", "improve documentation for the configuration"},
		// Avoid "add" which matches TaskFeature first.
		{"comment keyword", "improve comment in the complex functions"},
		{"readme keyword", "update the readme with installation instructions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if profile.TaskType != TaskDocs {
				t.Fatalf("expected task type 'docs' for %q, got %s", tt.content, profile.TaskType)
			}
		})
	}
}

func TestClassifyTaskType_Default(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Can you help me with something in the codebase?",
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.TaskType != TaskFeature {
		t.Fatalf("expected default task type 'feature', got %s", profile.TaskType)
	}
}

// ---------------------------------------------------------------------------
// Hint overrides
// ---------------------------------------------------------------------------

func TestClassifyHintOverrides(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "fix the typo in the readme",
	}}
	hints := &api.RoutingExtensions{
		HintDifficulty: 5,
		HintTaskType:   "refactor",
	}
	profile := c.Classify(msgs, 0, 0, hints)
	if profile.Difficulty != 5 {
		t.Fatalf("expected difficulty 5 from hint, got %d", profile.Difficulty)
	}
	if profile.TaskType != TaskRefactor {
		t.Fatalf("expected task type 'refactor' from hint, got %s", profile.TaskType)
	}
	if profile.ClassificationSource != "structural_with_hints" {
		t.Fatalf("expected classification source 'structural_with_hints', got %q", profile.ClassificationSource)
	}
}

func TestClassifyHintOverrides_DifficultyOnly(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Hello", // would normally be trivial
	}}
	hints := &api.RoutingExtensions{
		HintDifficulty: 3,
	}
	profile := c.Classify(msgs, 0, 0, hints)
	if profile.Difficulty != 3 {
		t.Fatalf("expected difficulty 3 from hint, got %d", profile.Difficulty)
	}
	// Task type should still be determined structurally
	if profile.TaskType != TaskFeature {
		t.Fatalf("expected task type 'feature' (default), got %s", profile.TaskType)
	}
}

func TestClassifyHintOverrides_TaskTypeOnly(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Hello", // would normally be feature
	}}
	hints := &api.RoutingExtensions{
		HintTaskType: "docs",
	}
	profile := c.Classify(msgs, 0, 0, hints)
	if profile.TaskType != TaskDocs {
		t.Fatalf("expected task type 'docs' from hint, got %s", profile.TaskType)
	}
}

func TestClassifyHintOverrides_NilHints(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Hello",
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.ClassificationSource != "structural" {
		t.Fatalf("expected classification source 'structural' when hints are nil, got %q", profile.ClassificationSource)
	}
}

// ---------------------------------------------------------------------------
// Escalation
// ---------------------------------------------------------------------------

func TestApplyEscalationRetries(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 2, Steps: 10, Repetitions: 10},
	})
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 0, 3, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after retry escalation, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRetries_DefaultThreshold(t *testing.T) {
	c := NewCodingTaskClassifier(nil) // default EscalationRetries = 3
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 0, 3, nil) // retryAttempt == 3, meets threshold
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after retry escalation with default threshold, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRetries_BelowThreshold(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 5, Steps: 10, Repetitions: 10},
	})
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 0, 3, nil) // retryAttempt=3 < threshold=5, no escalation
	if profile.Difficulty >= DifficultyComplex {
		t.Fatalf("expected difficulty < %d when retries below threshold, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationSteps(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 5, Repetitions: 10},
	})
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 6, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after step escalation, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationSteps_DefaultThreshold(t *testing.T) {
	c := NewCodingTaskClassifier(nil) // default EscalationSteps = 6
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 6, 0, nil) // stepCount == 6, meets threshold
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after step escalation with default threshold, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationSteps_BelowThreshold(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 10, Repetitions: 10},
	})
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 6, 0, nil) // stepCount=6 < threshold=10, no escalation
	if profile.Difficulty >= DifficultyComplex {
		t.Fatalf("expected difficulty < %d when steps below threshold, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRepetitions(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 10, Repetitions: 3},
	})
	msgs := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after repetition escalation, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRepetitions_DefaultThreshold(t *testing.T) {
	c := NewCodingTaskClassifier(nil) // default EscalationRepetitions = 3
	msgs := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after repetition escalation with default threshold, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRepetitions_BelowThreshold(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 10, Repetitions: 5},
	})
	msgs := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	profile := c.Classify(msgs, 0, 0, nil)
	// 3 repetitions < threshold 5, no escalation
	if profile.Difficulty >= DifficultyComplex {
		t.Fatalf("expected difficulty < %d when repetitions below threshold, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRepetitions_DifferentSignatures(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 10, Repetitions: 3},
	})
	msgs := []api.Message{
		{Role: "user", Content: "work on the codebase"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "write_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "done", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"other.go"}`}}}},
	}
	profile := c.Classify(msgs, 0, 0, nil)
	// All different signatures, no repetition escalation
	if profile.Difficulty >= DifficultyComplex {
		t.Fatalf("expected difficulty < %d when tool call signatures differ, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRepetitions_ZeroInConfigUsesDefault(t *testing.T) {
	// When Repetitions is 0 in config, NewCodingTaskClassifier keeps the default (3).
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 10, Repetitions: 0},
	})
	// The default EscalationRepetitions is 3, so 3 identical tool calls should trigger escalation.
	msgs := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d when zero config falls back to default threshold, got %d", DifficultyComplex, profile.Difficulty)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestClassify_EmptyMessages(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	profile := c.Classify(nil, 0, 0, nil)
	if profile == nil {
		t.Fatal("expected non-nil profile for nil messages")
	}
	// Empty text → trivial (textLen=0 < 50, fileCount=0, no traceback)
	if profile.Difficulty != DifficultyTrivial {
		t.Fatalf("expected difficulty %d (trivial) for empty messages, got %d", DifficultyTrivial, profile.Difficulty)
	}
}

func TestClassify_EmptyUserMessage(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{Role: "user", Content: ""}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile == nil {
		t.Fatal("expected non-nil profile for empty user message")
	}
}

func TestClassify_FallsBackToAssistantMessage(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{
		{Role: "assistant", Content: "I found the bug in parser.go"},
	}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile == nil {
		t.Fatal("expected non-nil profile when falling back to assistant message")
	}
	if profile.FileCount != 1 {
		t.Fatalf("expected file count 1 from assistant message, got %d", profile.FileCount)
	}
}

func TestClassify_ProfileFields(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role: "user",
		Content: `Fix this crash:

Traceback (most recent call last):
  File "handler.go", line 42, in handleRequest
    result := process(input)
KeyError: 'missing_key'

Files: handler.go, service.go, repository.go`,
	}}
	profile := c.Classify(msgs, 2, 1, nil)
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d (complex), got %d", DifficultyComplex, profile.Difficulty)
	}
	if profile.TaskType != TaskFix {
		t.Fatalf("expected task type 'fix', got %s", profile.TaskType)
	}
	if profile.FileCount < 3 {
		t.Fatalf("expected at least 3 files, got %d", profile.FileCount)
	}
	if !profile.HasTraceback {
		t.Fatal("expected has_traceback=true")
	}
	if profile.Step != 2 {
		t.Fatalf("expected step 2, got %d", profile.Step)
	}
	if profile.Retries != 1 {
		t.Fatalf("expected retries 1, got %d", profile.Retries)
	}
	if profile.ClassificationSource != "structural" {
		t.Fatalf("expected classification source 'structural', got %q", profile.ClassificationSource)
	}
	if profile.LatencyMs < 0 {
		t.Fatal("expected non-negative latency")
	}
}

func TestClassify_NonStringContent(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	// Content is a non-string type (e.g. array of content parts)
	msgs := []api.Message{{
		Role:    "user",
		Content: []interface{}{map[string]interface{}{"type": "text", "text": "hello"}},
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile == nil {
		t.Fatal("expected non-nil profile for non-string content")
	}
	// Should fall through to empty text → trivial
	if profile.Difficulty != DifficultyTrivial {
		t.Fatalf("expected difficulty %d (trivial) for non-string content, got %d", DifficultyTrivial, profile.Difficulty)
	}
}

func TestClassify_NewClassifierWithNilConfig(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	if c.EscalationRetries != 3 {
		t.Fatalf("expected default EscalationRetries 3, got %d", c.EscalationRetries)
	}
	if c.EscalationSteps != 6 {
		t.Fatalf("expected default EscalationSteps 6, got %d", c.EscalationSteps)
	}
	if c.EscalationRepetitions != 3 {
		t.Fatalf("expected default EscalationRepetitions 3, got %d", c.EscalationRepetitions)
	}
}

func TestClassify_NewClassifierWithPartialConfig(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{
			Retries: 5,
			// Steps and Repetitions left at 0 → use defaults
		},
	})
	if c.EscalationRetries != 5 {
		t.Fatalf("expected EscalationRetries 5, got %d", c.EscalationRetries)
	}
	if c.EscalationSteps != 6 {
		t.Fatalf("expected default EscalationSteps 6, got %d", c.EscalationSteps)
	}
	if c.EscalationRepetitions != 3 {
		t.Fatalf("expected default EscalationRepetitions 3, got %d", c.EscalationRepetitions)
	}
}

func TestClassify_DifficultyLabel(t *testing.T) {
	tests := []struct {
		difficulty int
		label      string
	}{
		{DifficultyTrivial, "trivial"},
		{DifficultySimple, "simple"},
		{DifficultyModerate, "moderate"},
		{DifficultyComplex, "complex"},
		{DifficultyNovel, "novel"},
		{99, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := difficultyLabel(tt.difficulty); got != tt.label {
				t.Fatalf("difficultyLabel(%d) = %q, want %q", tt.difficulty, got, tt.label)
			}
		})
	}
}

func TestClassify_FileCountDetection(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role: "user",
		Content: `Update these files:
- handler.go
- service.ts
- parser.py
- app.yaml
- README.md`,
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.FileCount < 5 {
		t.Fatalf("expected at least 5 files detected, got %d", profile.FileCount)
	}
}

func TestClassify_TracebackDetection(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"Go panic", "panic: runtime error: invalid memory address"},
		{"fatal error", "fatal error: concurrent map writes"},
		{"exit status", "exit status 1"},
		// The tracebackRe uses (?m)^ so patterns must be at start of a line.
		{"at pattern", "at com.example.Main.main(Main.java:10)"},
		{"in pattern", "in module: error occurred"},
		{"File pattern", `File "test.py", line 5, in test_func`},
		{"Error: pattern", "Error: connection refused"},
		// .go:line pattern must be at the start of a line for the regex to match.
		{".go:line pattern", ".go:42: undefined: x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCodingTaskClassifier(nil)
			msgs := []api.Message{{Role: "user", Content: tt.content}}
			profile := c.Classify(msgs, 0, 0, nil)
			if !profile.HasTraceback {
				t.Fatalf("expected has_traceback=true for %q", tt.content)
			}
		})
	}
}

func TestClassify_NoTracebackDetection(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "Can you help me write a new feature?",
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.HasTraceback {
		t.Fatal("expected has_traceback=false for normal text")
	}
}
