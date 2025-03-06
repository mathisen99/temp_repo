package tools

import (
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() jsonschema.Definition
	Execute(args string) (string, error)
	ToOpenAITool() openai.Tool
}

type BaseTool struct {
	ToolName        string
	ToolDescription string
	ToolParameters  jsonschema.Definition
}

func (b *BaseTool) Name() string {
	return b.ToolName
}

func (b *BaseTool) Description() string {
	return b.ToolDescription
}

func (b *BaseTool) Parameters() jsonschema.Definition {
	return b.ToolParameters
}

func (b *BaseTool) ToOpenAITool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        b.Name(),
			Description: b.Description(),
			Parameters:  b.Parameters(),
		},
	}
}