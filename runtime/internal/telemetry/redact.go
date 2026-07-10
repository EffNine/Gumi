// Package telemetry provides local observability recording and privacy-safe
// redaction for the Novexa runtime.
package telemetry

import (
	"encoding/json"
	"strings"
)

// redactedPlaceholder is the value used to mask sensitive data.
const redactedPlaceholder = "***REDACTED***"

// sensitiveKeyFragments matches keys that likely contain secrets. Comparison is
// case-insensitive and the fragment may appear anywhere in the key.
var sensitiveKeyFragments = []string{
	"authorization",
	"api_key",
	"apikey",
	"secret",
	"token",
	"password",
	"credential",
	"key",
}

// RedactJSON redacts sensitive values inside arbitrary JSON. It returns the
// redacted JSON unchanged on failure, so callers never receive nil.
func RedactJSON(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	v = redactValue(v)
	out, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return out
}

// RedactString returns a redacted placeholder if the key is sensitive.
func RedactString(key, value string) string {
	if isSensitiveKey(key) {
		return redactedPlaceholder
	}
	return value
}

// redactValue recursively redacts maps and slices.
func redactValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			if isSensitiveKey(k) {
				out[k] = redactedPlaceholder
				continue
			}
			out[k] = redactValue(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, val := range x {
			out[i] = redactValue(val)
		}
		return out
	default:
		return x
	}
}

// isSensitiveKey reports whether the key indicates a secret value.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, frag := range sensitiveKeyFragments {
		if strings.Contains(lower, frag) {
			return true
		}
	}
	return false
}
