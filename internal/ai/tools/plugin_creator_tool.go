package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
	"ircbot/internal/userlevels"
)

// PluginCreatorArgs represents the arguments for the createPlugin tool
type PluginCreatorArgs struct {
	UserHostmask   string `json:"userHostmask"`           // Required to check admin permission
	PluginName     string `json:"pluginName"`             // Name of the plugin to create (e.g., "GreeterPlugin")
	PluginFilename string `json:"pluginFilename"`         // Filename for the plugin (e.g., "greeter_plugin.go")
	Version        string `json:"version,omitempty"`      // Optional version (default: "1.0.0")
	Description    string `json:"description"`            // Brief description of the plugin's functionality
	Commands       string `json:"commands,omitempty"`     // Comma-separated list of commands this plugin should handle
	HandleEvents   string `json:"handleEvents,omitempty"` // Comma-separated list of events to handle (join, part, kick, etc.)
	Requirements   string `json:"requirements,omitempty"` // Any specific requirements or features needed
	LoadAfterBuild bool   `json:"loadAfterBuild"`         // Whether to load the plugin after building
}

// PluginCreatorTool creates Go plugin source code and builds it into a loadable plugin
type PluginCreatorTool struct {
	BaseTool
}

// NewPluginCreatorTool creates a new plugin creator tool
func NewPluginCreatorTool() *PluginCreatorTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"userHostmask": {
				Type:        jsonschema.String,
				Description: "The hostmask of the user requesting to create a plugin (used for permission check)",
			},
			"pluginName": {
				Type:        jsonschema.String,
				Description: "Name of the plugin to create (e.g., 'GreeterPlugin')",
			},
			"pluginFilename": {
				Type:        jsonschema.String,
				Description: "Filename for the plugin (e.g., 'greeter_plugin.go')",
			},
			"version": {
				Type:        jsonschema.String,
				Description: "Version of the plugin (default: '1.0.0')",
			},
			"description": {
				Type:        jsonschema.String,
				Description: "Brief description of the plugin's functionality",
			},
			"commands": {
				Type:        jsonschema.String,
				Description: "Comma-separated list of commands this plugin should handle (e.g., 'greet,welcome')",
			},
			"handleEvents": {
				Type:        jsonschema.String,
				Description: "Comma-separated list of events to handle (e.g., 'join,part,kick')",
			},
			"requirements": {
				Type:        jsonschema.String,
				Description: "Any specific requirements or features needed for the plugin",
			},
			"loadAfterBuild": {
				Type:        jsonschema.Boolean,
				Description: "Whether to load the plugin after building (default: false)",
			},
		},
		Required: []string{"userHostmask", "pluginName", "pluginFilename", "description"},
	}

	return &PluginCreatorTool{
		BaseTool: BaseTool{
			ToolName:        "createPlugin",
			ToolDescription: "Create a new IRC bot plugin with the specified functionality. The tool will generate appropriate Go code, build the plugin, and optionally load it into the bot. This tool is restricted to administrators and owners only.",
			ToolParameters:  params,
		},
	}
}

// Execute processes the tool call with the provided arguments
func (t *PluginCreatorTool) Execute(args string) (string, error) {
	logger.Debugf("PluginCreatorTool.Execute called with args: %s", args)

	var params PluginCreatorArgs
	err := json.Unmarshal([]byte(args), &params)
	if err != nil {
		logger.Errorf("Failed to parse plugin creator args: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	// Check permission (admin or owner)
	if !userlevels.HasPermission(params.UserHostmask, userlevels.Admin) && !userlevels.IsVerifiedOwner(params.UserHostmask) {
		logger.Warnf("Unauthorized access attempt to plugin creator from %s", params.UserHostmask)
		return "", fmt.Errorf("access denied: plugin creation is restricted to administrators and owners")
	}

	// Set default version if not specified
	if params.Version == "" {
		params.Version = "1.0.0"
	}

	// Get appropriate prompt for the AI
	prompt, err := FormatPluginPrompt(params)
	if err != nil {
		logger.Errorf("Failed to format plugin prompt: %v", err)
		return "", fmt.Errorf("failed to format plugin prompt: %v", err)
	}

	// Use the AI to generate plugin code
	pluginCode, err := generateAIPluginCode(prompt, params)
	if err != nil {
		logger.Errorf("Failed to generate plugin code with AI: %v", err)
		
		// Fallback to template-based code generation if AI fails
		logger.Warnf("Falling back to template-based code generation")
		pluginCode, err = generatePluginCode(params)
		if err != nil {
			logger.Errorf("Template fallback also failed: %v", err)
			return "", fmt.Errorf("failed to generate plugin code: %v", err)
		}
	}

	// Save the plugin to file
	pluginPath := filepath.Join("plugins_src", params.PluginFilename)
	logger.Debugf("Saving plugin to %s", pluginPath)

	err = os.WriteFile(pluginPath, []byte(pluginCode), 0644)
	if err != nil {
		logger.Errorf("Failed to write plugin file: %v", err)
		return "", fmt.Errorf("failed to save plugin file: %v", err)
	}

	// Build the plugin
	buildOutput, err := buildPlugin(params.PluginFilename)
	if err != nil {
		logger.Errorf("Failed to build plugin: %v", err)
		return "", fmt.Errorf("failed to build plugin: %v\nBuild output: %s", err, buildOutput)
	}

	// Load the plugin if requested
	if params.LoadAfterBuild {
		// Extract plugin base name without extension for loading
		pluginBaseName := strings.TrimSuffix(params.PluginFilename, filepath.Ext(params.PluginFilename))
		loadOutput, err := loadPlugin(pluginBaseName)
		if err != nil {
			return fmt.Sprintf("Plugin created and built successfully, but failed to load: %v\n\nBuild output: %s\n\nLoad output: %s",
				err, buildOutput, loadOutput), nil
		}
		return fmt.Sprintf("Plugin '%s' created, built, and loaded successfully!\n\nBuild output: %s\n\nLoad output: %s",
			params.PluginName, buildOutput, loadOutput), nil
	}

	return fmt.Sprintf("Plugin '%s' created and built successfully!\n\nBuild output: %s", params.PluginName, buildOutput), nil
}

// generateAIPluginCode generates plugin code using the AI system
func generateAIPluginCode(prompt string, params PluginCreatorArgs) (string, error) {
	// Get the API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	// Create OpenAI client
	client := openai.NewClient(apiKey)
	
	// Create system prompt with the plugin creator instructions
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: PluginCreatorSystemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	// Create a request context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create chat completion request
	request := openai.ChatCompletionRequest{
		Model:       "gpt-4o", // Use a suitable model for code generation
		Messages:    messages,
		Temperature: 0.2, // Lower temperature for more deterministic code generation
		MaxTokens:   4000, // Allow sufficient tokens for code generation
	}

	// Send request to OpenAI
	logger.Debugf("Sending plugin creation request to OpenAI")
	resp, err := client.CreateChatCompletion(ctx, request)
	if err != nil {
		logger.Errorf("OpenAI API error during plugin creation: %v", err)
		return "", fmt.Errorf("AI code generation failed: %v", err)
	}

	// Extract code from response
	aiResponse := resp.Choices[0].Message.Content
	logger.Debugf("Received response from OpenAI, length: %d characters", len(aiResponse))
	
	// Look for Go code block in markdown response
	codePattern := "(?s)```go\n(.*?)```"
	re := regexp.MustCompile(codePattern)
	matches := re.FindStringSubmatch(aiResponse)
	
	if len(matches) > 1 {
		// Found Go code block
		logger.Debugf("Extracted code block from AI response")
		return matches[1], nil
	}
	
	// If no code block found, return the full response
	// This handles cases where the AI doesn't format with markdown
	logger.Debugf("No code block found, using full AI response")
	return aiResponse, nil
}

// generatePluginCode creates the Go source code for the plugin based on the provided parameters
func generatePluginCode(params PluginCreatorArgs) (string, error) {
	// Sanitize plugin filename to create build tag
	buildTag := strings.TrimSuffix(params.PluginFilename, ".go")

	// Parse command list
	commands := []string{}
	if params.Commands != "" {
		commands = strings.Split(params.Commands, ",")
		for i, cmd := range commands {
			commands[i] = strings.TrimSpace(cmd)
		}
	}

	// Parse event handlers
	eventHandlers := []string{}
	if params.HandleEvents != "" {
		eventHandlers = strings.Split(params.HandleEvents, ",")
		for i, event := range eventHandlers {
			eventHandlers[i] = strings.TrimSpace(event)
		}
	}

	// Generate code template
	var code strings.Builder

	// Build tags
	code.WriteString(fmt.Sprintf("//go:build %s\n", buildTag))
	code.WriteString(fmt.Sprintf("// +build %s\n\n", buildTag))

	// Package declaration
	code.WriteString("package main\n\n")

	// Imports
	code.WriteString("import (\n")
	code.WriteString("\t\"gopkg.in/irc.v4\"\n")
	code.WriteString("\t\"ircbot/pkg/api\"\n")
	code.WriteString(")\n\n")

	// Plugin struct
	code.WriteString(fmt.Sprintf("// %s implements a plugin that %s\n", params.PluginName, params.Description))
	code.WriteString(fmt.Sprintf("type %s struct{}\n\n", params.PluginName))

	// Name method
	code.WriteString(fmt.Sprintf("// Name returns the plugin name\n"))
	code.WriteString(fmt.Sprintf("func (p *%s) Name() string {\n", params.PluginName))
	code.WriteString(fmt.Sprintf("\treturn \"%s\"\n", params.PluginName))
	code.WriteString("}\n\n")

	// Version method
	code.WriteString(fmt.Sprintf("// Version returns the plugin version\n"))
	code.WriteString(fmt.Sprintf("func (p *%s) Version() string {\n", params.PluginName))
	code.WriteString(fmt.Sprintf("\treturn \"%s\"\n", params.Version))
	code.WriteString("}\n\n")

	// OnLoad method
	code.WriteString(fmt.Sprintf("// OnLoad is called when the plugin is loaded\n"))
	code.WriteString(fmt.Sprintf("func (p *%s) OnLoad() error {\n", params.PluginName))
	code.WriteString(fmt.Sprintf("\tapi.LogSuccess(\"%s version %%s loaded!\", p.Version())\n", params.PluginName))
	code.WriteString("\treturn nil\n")
	code.WriteString("}\n\n")

	// Command handler if commands are requested
	if len(commands) > 0 {
		// GetCommands method
		code.WriteString(fmt.Sprintf("// GetCommands returns the list of commands this plugin handles\n"))
		code.WriteString(fmt.Sprintf("func (p *%s) GetCommands() []string {\n", params.PluginName))
		code.WriteString(fmt.Sprintf("\treturn []string{%s}\n", formatStringList(commands)))
		code.WriteString("}\n\n")

		// HandleCommand method
		code.WriteString(fmt.Sprintf("// HandleCommand processes commands directed to this plugin\n"))
		code.WriteString(fmt.Sprintf("func (p *%s) HandleCommand(c *irc.Client, m *irc.Message, cmd string, args []string) {\n", params.PluginName))
		code.WriteString("\ttarget := api.GetReplyTarget(c, m)\n\n")

		// Switch statement for different commands
		if len(commands) > 1 {
			code.WriteString("\tswitch cmd {\n")
			for _, cmd := range commands {
				code.WriteString(fmt.Sprintf("\tcase \"%s\":\n", cmd))
				code.WriteString(fmt.Sprintf("\t\tapi.SendMessage(c, target, \"Handling %s command\")\n", cmd))
				code.WriteString("\t\t// TODO: Implement command logic\n\n")
			}
			code.WriteString("\t}\n")
		} else if len(commands) == 1 {
			code.WriteString(fmt.Sprintf("\t// Handle the %s command\n", commands[0]))
			code.WriteString(fmt.Sprintf("\tapi.SendMessage(c, target, \"Handling %s command\")\n", commands[0]))
			code.WriteString("\t// TODO: Implement command logic\n")
		}

		code.WriteString("}\n\n")
	}

	// OnMessage method
	code.WriteString(fmt.Sprintf("// OnMessage handles all IRC messages - THIS METHOD IS REQUIRED FOR ALL PLUGINS\n"))
	code.WriteString(fmt.Sprintf("func (p *%s) OnMessage(c *irc.Client, m *irc.Message) {\n", params.PluginName))
	if len(commands) > 0 {
		code.WriteString("\t// This method is required even if the plugin uses HandleCommand for command processing\n")
		code.WriteString("\t// You can leave it empty, but it must be implemented\n")
	} else {
		code.WriteString("\t// Process messages here - this method is required for all plugins\n")
		code.WriteString("\t// At minimum, implement with empty body\n")
	}
	code.WriteString("}\n\n")

	// Event handlers if requested
	for _, event := range eventHandlers {
		var methodName, eventParam, comment string

		switch strings.ToLower(event) {
		case "join":
			methodName = "OnJoin" // CRITICAL: This must be OnJoin, not HandleJoin
			eventParam = "channel := m.Params[0]"
			comment = "When users join channels"
		case "part":
			methodName = "OnPart"
			eventParam = "channel := m.Params[0]"
			comment = "When users leave channels"
		case "quit":
			methodName = "OnQuit"
			eventParam = "reason := \"\"\nif len(m.Params) > 0 {\n\t\treason = m.Params[0]\n\t}"
			comment = "When users quit IRC"
		case "kick":
			methodName = "OnKick"
			eventParam = "channel := m.Params[0]\nkicked := m.Params[1]"
			comment = "When users are kicked"
		case "topic":
			methodName = "OnTopicChange"
			eventParam = "channel := m.Params[0]\nnewTopic := m.Params[1]"
			comment = "When channel topics change"
		case "nick":
			methodName = "OnNickChange"
			eventParam = "oldNick := m.Prefix.Name\nnewNick := m.Params[0]"
			comment = "When users change nicks"
		case "invite":
			methodName = "OnInvite"
			eventParam = "invitedChannel := m.Params[1]"
			comment = "When the bot is invited to channels"
		case "notice":
			methodName = "OnNotice"
			eventParam = "target := m.Params[0]"
			comment = "When the bot receives notices"
		case "nickmention":
			methodName = "OnNickMention"
			eventParam = "channel := m.Params[0]"
			comment = "When the bot's nick is mentioned"
		default:
			continue
		}

		code.WriteString(fmt.Sprintf("// %s handles %s\n", methodName, comment))
		code.WriteString(fmt.Sprintf("func (p *%s) %s(c *irc.Client, m *irc.Message) {\n", params.PluginName, methodName))

		// Skip own events in join, part, kick handlers
		if methodName == "OnJoin" || methodName == "OnPart" || methodName == "OnKick" {
			code.WriteString("\t// Don't respond to our own events\n")
			code.WriteString("\tif m.Prefix.Name == c.CurrentNick() {\n")
			code.WriteString("\t\treturn\n")
			code.WriteString("\t}\n\n")
		}

		// Add the event parameter code
		code.WriteString(fmt.Sprintf("\t%s\n\n", eventParam))

		// Add a sample action
		if methodName == "OnJoin" {
			code.WriteString("\t// Send welcome message\n")
			code.WriteString("\tapi.SendMessage(c, channel, \"Welcome to \" + channel + \", \" + m.Prefix.Name + \"!\")\n")
		} else {
			code.WriteString("\t// TODO: Implement event handler logic\n")
		}

		code.WriteString("}\n\n")
	}

	// OnUnload method
	code.WriteString(fmt.Sprintf("// OnUnload is called when the plugin is unloaded\n"))
	code.WriteString(fmt.Sprintf("func (p *%s) OnUnload() error {\n", params.PluginName))
	code.WriteString(fmt.Sprintf("\tapi.LogSuccess(\"%s version %%s unloaded!\", p.Version())\n", params.PluginName))
	code.WriteString("\treturn nil\n")
	code.WriteString("}\n\n")

	// Plugin export variable
	code.WriteString(fmt.Sprintf("// Export the plugin\n"))
	code.WriteString(fmt.Sprintf("var Plugin %s\n", params.PluginName))

	return code.String(), nil
}

// formatStringList formats a slice of strings as Go string literals for code generation
func formatStringList(items []string) string {
	if len(items) == 0 {
		return ""
	}

	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("\"%s\"", item)
	}
	return strings.Join(quoted, ", ")
}

// buildPlugin builds the plugin directly without using build_plugins.sh
func buildPlugin(pluginFilename string) (string, error) {
	// Extract plugin base name without extension for build tag
	buildTag := strings.TrimSuffix(pluginFilename, filepath.Ext(pluginFilename))

	logger.Debugf("Building plugin with build tag: %s", buildTag)

	// Get plugin version from source file
	pluginVersion, err := extractPluginVersion(filepath.Join("plugins_src", pluginFilename))
	if err != nil {
		// Default to 1.0.0 if version extraction fails
		pluginVersion = "1.0.0"
		logger.Warnf("Could not extract plugin version: %v, using default: %s", err, pluginVersion)
	}

	// Create versioned output filename
	versionedFilename := fmt.Sprintf("%s_v%s.so", buildTag, pluginVersion)
	outputPath := filepath.Join("plugins", versionedFilename)
	symlinkPath := filepath.Join("plugins", buildTag+".so")

	// Build the plugin directly using go build
	cmd := exec.Command("go", "build", 
		"-buildmode=plugin", 
		"-tags", buildTag, 
		"-o", outputPath,
		filepath.Join("plugins_src", pluginFilename))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("go build failed: %v", err)
	}

	// Create symlink to the latest version
	// Remove existing symlink if it exists
	_ = os.Remove(symlinkPath)
	err = os.Symlink(versionedFilename, symlinkPath)
	if err != nil {
		return string(output) + "\nPlugin built successfully, but failed to create symlink", 
			fmt.Errorf("symlink creation failed: %v", err)
	}

	return fmt.Sprintf("Plugin built successfully:\n- Created: %s\n- Symlinked to: %s", 
		outputPath, symlinkPath), nil
}

// extractPluginVersion extracts the version string from a plugin source file
func extractPluginVersion(filePath string) (string, error) {
	// Read the plugin source file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read plugin file: %v", err)
	}

	// Search for the Version function
	versionPattern := `func\s+\(\w+\s+\*\w+\)\s+Version\(\)\s+string\s+{\s*return\s+"([0-9]+\.[0-9]+\.[0-9]+)"\s*}`
	re := regexp.MustCompile(versionPattern)
	matches := re.FindSubmatch(content)

	if len(matches) < 2 {
		return "", fmt.Errorf("version not found in plugin source")
	}

	return string(matches[1]), nil
}

// loadPlugin loads a plugin into the running bot
func loadPlugin(pluginName string) (string, error) {
	// Wait a short time for the file system to register the new file
	time.Sleep(500 * time.Millisecond)

	// Check if the plugin file exists
	pluginPath := filepath.Join("plugins", pluginName+".so")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return "", fmt.Errorf("plugin file not found: %s", pluginPath)
	}

	// This is a convenience function that uses the plugin manager
	// In a real implementation, you would call the plugin manager's LoadPlugin method
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '!load %s' > data/command_pipe", pluginName))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to load plugin: %v", err)
	}

	// Give the bot a moment to load the plugin
	time.Sleep(1 * time.Second)

	return fmt.Sprintf("Plugin '%s' loaded successfully", pluginName), nil
}
