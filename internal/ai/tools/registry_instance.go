// Package tools provides a framework for creating and managing AI tools
// that can be exposed to LLM models like GPT-4.
package tools

import (
	"ircbot/internal/logger"
	"sync"
)

var (
	// Global singleton registry instance
	registry     *ToolRegistry
	registryOnce sync.Once
)

// GetRegistry returns the singleton registry instance.
// On the first call, this initializes the registry and registers all default tools.
// Subsequent calls return the same instance.
//
// The "New" prefix in tool constructor functions (e.g., NewIRCChannelTool) follows
// the standard Go convention for constructors that return pointers to newly created
// instances. The actual function names exposed to AI models are defined separately
// in each constructor and typically use camelCase (e.g., getIRCChannelInfo).
func GetRegistry() *ToolRegistry {
	registryOnce.Do(func() {
		logger.AIDebugf("Initializing global tool registry")
		registry = NewToolRegistry()

		// Register default tools
		defaultTools := []Tool{
			NewWebsiteTool(),
			NewImageTool(),
			NewImageGenerationTool(),
			NewPasteTool(),
			NewChannelLogTool(),
			NewErrorLogTool(),
			NewPluginCreatorTool(),
			NewPythonDockerTool(),
			NewGoogleSearchTool(),
			NewSaveNoteTool(),
			NewDeleteNoteTool(),
			NewListNotesTool(),
			NewSearchNotesTool(),
		}

		for _, tool := range defaultTools {
			registry.RegisterTool(tool)
		}

		logger.Successf("Initialized global tool registry with %d default tools", len(defaultTools))
	})
	return registry
}

// RegisterCustomTool adds a custom tool to the global registry.
// This function is a convenience wrapper around GetRegistry().RegisterTool().
func RegisterCustomTool(tool Tool) {
	GetRegistry().RegisterTool(tool)
}

// GetAllRegisteredTools returns all tools in the global registry.
// This function is a convenience wrapper around GetRegistry().GetAllTools().
func GetAllRegisteredTools() []Tool {
	return GetRegistry().GetAllTools()
}