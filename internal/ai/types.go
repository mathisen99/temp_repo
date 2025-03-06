package ai

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`       // For tool messages
	ToolCallID string      `json:"tool_call_id,omitempty"` // For tool response messages
}

type ToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}