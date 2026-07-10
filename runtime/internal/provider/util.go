package provider

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// randomID returns a short random identifier safe for response IDs.
func randomID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// messageContentToString converts an OpenAI message content field to a string.
// It accepts strings directly and falls back to JSON encoding for other types.
func messageContentToString(content interface{}) (string, error) {
	if content == nil {
		return "", nil
	}
	if s, ok := content.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", content), nil
}
