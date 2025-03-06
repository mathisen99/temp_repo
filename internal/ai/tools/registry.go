// Package tools provides a framework for creating and managing AI tools
// that can be exposed to LLM models like GPT-4.
package tools

import (
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"
	"ircbot/internal/logger"
)

// ToolRegistry manages the collection of available AI tools.
// It provides thread-safe registration, retrieval, and execution of tools.
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewToolRegistry creates a new tool registry.
// This function initializes an empty registry without any tools.
// Use RegisterTool to add tools to the registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// RegisterTool adds a new tool to the registry.
// If a tool with the same name already exists, it will be replaced.
func (r *ToolRegistry) RegisterTool(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		logger.Warnf("Replacing existing tool: %s", name)
	}
	
	r.tools[name] = tool
	logger.AIDebugf("Registered tool: %s", name)
}

// DeregisterTool removes a tool from the registry.
// If the tool doesn't exist, this operation is a no-op.
func (r *ToolRegistry) DeregisterTool(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.tools[name]; exists {
		delete(r.tools, name)
		logger.Debugf("Deregistered tool: %s", name)
	}
}

// GetTool returns a tool by name.
// If the tool doesn't exist, an error is returned.
func (r *ToolRegistry) GetTool(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}
	
	return tool, nil
}

// GetAllTools returns all registered tools as a slice.
// The order of tools in the slice is not guaranteed.
func (r *ToolRegistry) GetAllTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	
	return tools
}

// GetOpenAITools converts all registered tools to OpenAI's Tool format.
// This is used when making API requests to the OpenAI API.
func (r *ToolRegistry) GetOpenAITools() []openai.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]openai.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool.ToOpenAITool())
	}
	
	return tools
}

// ExecuteTool executes a named tool with the provided arguments.
// Arguments should be a JSON string that matches the tool's parameter schema.
// Returns the tool's output as a string, or an error if execution failed.
func (r *ToolRegistry) ExecuteTool(name string, args string) (string, error) {
	tool, err := r.GetTool(name)
	if err != nil {
		return "", err
	}
	
	logger.AIDebugf("Executing tool: %s with args: %s", name, args)
	result, err := tool.Execute(args)
	if err != nil {
		logger.Errorf("Tool execution error: %s: %v", name, err)
		return "", err
	}
	
	return result, nil
}