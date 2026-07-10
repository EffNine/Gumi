package api

// ModelsList is the OpenAI-compatible response envelope for /v1/models.
type ModelsList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model represents a single model entry.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// NewModelsList returns a populated model list with the Sprint 2 placeholder
// models. Real provider discovery will replace this in Sprint 3.
func NewModelsList() *ModelsList {
	return &ModelsList{
		Object: "list",
		Data: []Model{
			{ID: "local:auto", Object: "model", Created: 0, OwnedBy: "novexa"},
			{ID: "ollama:local:auto", Object: "model", Created: 0, OwnedBy: "ollama"},
			{ID: "lmstudio:local:auto", Object: "model", Created: 0, OwnedBy: "lmstudio"},
			{ID: "openai-compatible:local:auto", Object: "model", Created: 0, OwnedBy: "openai-compatible"},
		},
	}
}
