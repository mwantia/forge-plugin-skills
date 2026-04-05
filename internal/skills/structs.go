package skills

type Skill struct {
	Path               string               `json:"path"`
	Content            string               `json:"content"`
	Name               string               `json:"name"`
	Description        string               `json:"description"`
	Parameters         map[string]Parameter `json:"parameters"`
	Tags               []string             `json:"tags,omitempty"`
	Readonly           bool                 `json:"read_only,omitempty"`
	Destructive        bool                 `json:"destructive,omitempty"`
	Idempotent         bool                 `json:"idempotent,omitempty"`
	Version            string               `json:"version,omitempty"`
	Deprecated         bool                 `json:"deprecated,omitempty"`
	DeprecationMessage string               `json:"deprecation_message,omitempty"`
}

type Parameter struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Format      string   `json:"format,omitempty"`
}
