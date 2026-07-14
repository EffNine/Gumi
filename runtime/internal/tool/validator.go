package tool

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EffNine/gumi/runtime/internal/api"
)

// ValidationIssue describes a problem with a tool call.
type ValidationIssue struct {
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	ToolCall api.ToolCall `json:"tool_call"`
}

// ValidationReport is the result of checking parsed tool calls.
type ValidationReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidateToolCalls checks that every tool call references a known tool and
// that arguments are valid JSON. It performs a best-effort schema check on
// required top-level keys for V1.
func ValidateToolCalls(calls []api.ToolCall, tools []api.Tool) ValidationReport {
	report := ValidationReport{Valid: true}
	if len(calls) == 0 {
		return report
	}

	toolMap := make(map[string]api.Tool, len(tools))
	for _, t := range tools {
		if t.Type == "function" {
			toolMap[t.Function.Name] = t
		}
	}

	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name == "" {
			report.addIssue("MISSING_TOOL_NAME", "tool call is missing a function name", call)
			continue
		}
		tool, ok := toolMap[name]
		if !ok {
			report.addIssue("UNKNOWN_TOOL", fmt.Sprintf("tool %q is not in the available tools list", name), call)
			continue
		}

		args := strings.TrimSpace(call.Function.Arguments)
		if args == "" {
			report.addIssue("EMPTY_ARGUMENTS", fmt.Sprintf("tool %q call has empty arguments", name), call)
			continue
		}

		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(args), &decoded); err != nil {
			report.addIssue("INVALID_JSON_ARGUMENTS", fmt.Sprintf("tool %q arguments are not valid JSON: %v", name, err), call)
			continue
		}

		// Check required keys.
		if issue := checkRequiredKeys(tool, decoded); issue != "" {
			report.addIssue("MISSING_REQUIRED_ARGUMENT", fmt.Sprintf("tool %q: %s", name, issue), call)
		}

		// Deep schema validation: check argument types, enums, nested properties.
		if issues := checkArgumentTypes(tool, decoded); len(issues) > 0 {
			for _, iss := range issues {
				report.addIssue("INVALID_ARGUMENT_TYPE", fmt.Sprintf("tool %q: %s", name, iss), call)
			}
		}
	}

	return report
}

func (r *ValidationReport) addIssue(code, message string, call api.ToolCall) {
	r.Valid = false
	r.Issues = append(r.Issues, ValidationIssue{
		Code:     code,
		Message:  message,
		ToolCall: call,
	})
}

// checkRequiredKeys verifies that the decoded arguments contain any required
// top-level keys declared in the tool's JSON Schema parameters.
func checkRequiredKeys(tool api.Tool, args map[string]interface{}) string {
	if tool.Type != "function" || len(tool.Function.Parameters) == 0 {
		return ""
	}
	req, ok := tool.Function.Parameters["required"].([]interface{})
	if !ok {
		return ""
	}
	for _, r := range req {
		key, ok := r.(string)
		if !ok {
			continue
		}
		if _, present := args[key]; !present {
			return fmt.Sprintf("missing required argument %q", key)
		}
	}
	return ""
}

// checkArgumentTypes validates argument values against the tool's JSON Schema
// properties. It checks types, enums, nested objects, and array item types.
func checkArgumentTypes(tool api.Tool, args map[string]interface{}) []string {
	if tool.Type != "function" || len(tool.Function.Parameters) == 0 {
		return nil
	}

	props, ok := tool.Function.Parameters["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	var issues []string
	for key, value := range args {
		schema, ok := props[key].(map[string]interface{})
		if !ok {
			continue // unknown key, skip
		}
		if iss := validateValue(key, value, schema, ""); iss != "" {
			issues = append(issues, iss)
		}
	}
	return issues
}

// validateValue checks a single value against its JSON Schema definition.
// It recursively validates nested objects and array items.
func validateValue(key string, value interface{}, schema map[string]interface{}, path string) string {
	if schema == nil {
		return ""
	}

	fullPath := key
	if path != "" {
		fullPath = path + "." + key
	}

	schemaType, _ := schema["type"].(string)

	// Check enum constraints.
	if enumVals, ok := schema["enum"].([]interface{}); ok && len(enumVals) > 0 {
		if !containsValue(enumVals, value) {
			enumStr := formatEnumValues(enumVals)
			return fmt.Sprintf("argument %q has value %v which is not in allowed enum values %s", fullPath, value, enumStr)
		}
	}

	// Type check.
	if schemaType != "" {
		actual := goTypeToSchemaType(value)
		if actual != schemaType && !isTypeCoercible(actual, schemaType) {
			return fmt.Sprintf("argument %q expected type %q but got %q (value: %v)", fullPath, schemaType, actual, value)
		}
	}

	// Nested object validation.
	if schemaType == "object" {
		subProps, _ := schema["properties"].(map[string]interface{})
		subObj, ok := value.(map[string]interface{})
		if ok && subProps != nil {
			for subKey, subVal := range subObj {
				subSchema, _ := subProps[subKey].(map[string]interface{})
				if subSchema != nil {
					if iss := validateValue(subKey, subVal, subSchema, fullPath); iss != "" {
						return iss
					}
				}
			}
			// Check required nested keys.
			if subReq, ok := schema["required"].([]interface{}); ok {
				for _, r := range subReq {
					rKey, ok := r.(string)
					if !ok {
						continue
					}
					if _, present := subObj[rKey]; !present {
						return fmt.Sprintf("argument %q missing required nested key %q", fullPath, rKey)
					}
				}
			}
		}
	}

	// Array item validation.
	if schemaType == "array" {
		items, ok := schema["items"].(map[string]interface{})
		if ok {
			arr, ok := value.([]interface{})
			if ok {
				itemType, _ := items["type"].(string)
				for i, item := range arr {
					if itemType != "" {
						actual := goTypeToSchemaType(item)
						if actual != itemType && !isTypeCoercible(actual, itemType) {
							return fmt.Sprintf("argument %q[%d] expected type %q but got %q (value: %v)", fullPath, i, itemType, actual, item)
						}
					}
					// Check enum on array items.
					if enumVals, ok := items["enum"].([]interface{}); ok && len(enumVals) > 0 {
						if !containsValue(enumVals, item) {
							enumStr := formatEnumValues(enumVals)
							return fmt.Sprintf("argument %q[%d] has value %v which is not in allowed enum values %s", fullPath, i, item, enumStr)
						}
					}
				}
			}
		}
	}

	return ""
}

// goTypeToSchemaType maps Go types to JSON Schema types.
func goTypeToSchemaType(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// isTypeCoercible returns true if a value of actualType can be safely coerced
// to expectedType (e.g., number to integer).
func isTypeCoercible(actualType, expectedType string) bool {
	if actualType == "number" && expectedType == "integer" {
		return true
	}
	return false
}

// containsValue checks if a value exists in a slice of interface{} values.
func containsValue(slice []interface{}, val interface{}) bool {
	for _, item := range slice {
		if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", val) {
			return true
		}
	}
	return false
}

// formatEnumValues formats enum values for error messages.
func formatEnumValues(vals []interface{}) string {
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = fmt.Sprintf("%v", v)
	}
	return "[" + strings.Join(strs, ", ") + "]"
}

// SchemaViolations returns a human-readable summary of all validation issues
// suitable for injecting into a retry prompt. Returns empty string if valid.
func SchemaViolations(report ValidationReport) string {
	if report.Valid || len(report.Issues) == 0 {
		return ""
	}
	var lines []string
	for _, iss := range report.Issues {
		lines = append(lines, fmt.Sprintf("- %s: %s", iss.Code, iss.Message))
	}
	return strings.Join(lines, "\n")
}

// HasDuplicateToolCall returns true if the same tool call (name + arguments)
// appears in the recent assistant/tool history.
func HasDuplicateToolCall(call api.ToolCall, history []api.Message) bool {
	if call.Function.Name == "" {
		return false
	}
	target := canonicalToolSignature(call)
	for _, msg := range history {
		if msg.Role != "assistant" {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if canonicalToolSignature(tc) == target {
				return true
			}
		}
	}
	return false
}

func canonicalToolSignature(call api.ToolCall) string {
	return call.Function.Name + "|" + strings.TrimSpace(call.Function.Arguments)
}
