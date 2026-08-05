package llm

// ToolDef describes a function the LLM can call.
type ToolDef struct {
	Name        string
	Description string
	// For OpenAI/DeepSeek: Parameters follow the JSON Schema format (set via SetOpenAIParams).
	Parameters  map[string]*SchemaParam
	Required    []string
	// Internal: Schema format indicator
	isJSONSchema bool
}

// SchemaParam holds parameter schema info for both Gemini and OpenAI formats.
type SchemaParam struct {
	Type        string                   `json:"type"`
	Description string                   `json:"description"`
	Properties  map[string]*SchemaParam  `json:"properties,omitempty"`
	Items       *SchemaParam             `json:"items,omitempty"`
	Required    []string                 `json:"required,omitempty"`
	Enum        []string                 `json:"enum,omitempty"`
}

// ToolCall represents a function call requested by the model.
type ToolCall struct {
	ID      string
	Name    string
	Args    map[string]any
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string // "user", "model", "tool", "system"
	Content string
	ToolCalls []ToolCall
	ToolName  string // for tool-result messages
}
