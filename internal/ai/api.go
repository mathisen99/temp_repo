package ai

import (
	"ircbot/internal/ai/tools"
)

func Initialize() error {
	if err := InitializeClient(); err != nil {
		return err
	}
	
	tools.GetRegistry() // Force registry initialization
	
	return nil
}

func RegisterCustomTool(tool tools.Tool) {
	tools.GetRegistry().RegisterTool(tool)
}

func Status() map[string]interface{} {
	cfg := GetConfig()
	registry := tools.GetRegistry()
	
	availableTools := registry.GetAllTools()
	toolNames := make([]string, 0, len(availableTools))
	for _, tool := range availableTools {
		toolNames = append(toolNames, tool.Name())
	}
	
	return map[string]interface{}{
		"initialized":        IsInitialized(),
		"model":              cfg.Model,
		"enableTools":        cfg.EnableToolCalls,
		"enableSummarization": cfg.EnableSummarization,
		"availableTools":     toolNames,
	}
}

func UpdateSystemPrompt(newPrompt string) {
	UpdateConfig(func(cfg *Config) {
		cfg.SystemPrompt = newPrompt
	})
}

func SetModel(modelName string) {
	UpdateConfig(func(cfg *Config) {
		cfg.Model = modelName
	})
}

func EnableToolCalls(enable bool) {
	UpdateConfig(func(cfg *Config) {
		cfg.EnableToolCalls = enable
	})
}

func EnableSummarization(enable bool) {
	UpdateConfig(func(cfg *Config) {
		cfg.EnableSummarization = enable
	})
}