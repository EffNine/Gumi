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

// NewModelsList returns a base model list containing the local:auto alias.
// Provider-discovered models are merged by the gateway handler.
func NewModelsList() *ModelsList {
	return &ModelsList{
		Object: "list",
		Data: []Model{
			{ID: "local:auto", Object: "model", Created: 0, OwnedBy: "gumi"},
		},
	}
}
