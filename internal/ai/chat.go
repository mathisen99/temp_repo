package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sashabaranov/go-openai"
	"ircbot/internal/ai/tools"
	botconfig "ircbot/internal/config"
	"ircbot/internal/logger"
)

func processToolCalls(message openai.ChatCompletionMessage, currentUser, currentChannel string) []openai.ChatCompletionMessage {
	var toolResponses []openai.ChatCompletionMessage
	
	// Get full hostmask from owner settings if this is the owner
	settings, _ := botconfig.LoadSettings()
	userHostmask := currentUser // Default to just the nickname
	if settings != nil && settings.OwnerHostmask != "" && strings.HasPrefix(settings.OwnerHostmask, currentUser+"!") {
		// If this user is the owner, use the full hostmask from settings
		userHostmask = settings.OwnerHostmask
		logger.AIDebugf("Using owner hostmask for tool calls: %s", userHostmask)
	}
	
	for _, toolCall := range message.ToolCalls {
		logger.AIDebugf("Processing tool call: %s", toolCall.Function.Name)
		
		// Parse the arguments to a map for potential modification
		args := toolCall.Function.Arguments
		var argsMap map[string]interface{}
		if err := json.Unmarshal([]byte(args), &argsMap); err == nil {
			// Special handling for tools that need authorization
			if toolCall.Function.Name == "getErrorLog" || 
			   toolCall.Function.Name == "createPlugin" ||
			   strings.Contains(args, "\"userHostmask\"") {
				
				// Update the userHostmask parameter with the full hostmask
				argsMap["userHostmask"] = userHostmask
				
				// Convert back to JSON
				if newArgs, err := json.Marshal(argsMap); err == nil {
					args = string(newArgs)
					logger.AIDebugf("Updated args with full hostmask for %s: %s", toolCall.Function.Name, args)
				}
			}
		}
		
		// Special handling for note tools
		if strings.HasPrefix(toolCall.Function.Name, "save_note") ||
		   strings.HasPrefix(toolCall.Function.Name, "delete_note") ||
		   strings.HasPrefix(toolCall.Function.Name, "list_notes") ||
		   strings.HasPrefix(toolCall.Function.Name, "search_notes") {
		
			// Special case for deleting notes by content instead of ID
			if strings.HasPrefix(toolCall.Function.Name, "delete_note") {
				// If we're trying to delete a note but only have content
				var deleteArgs map[string]interface{}
				if err := json.Unmarshal([]byte(args), &deleteArgs); err == nil {
					// If there's an ID that doesn't look like a proper UUID/numeric ID
					if idStr, ok := deleteArgs["id"].(string); ok {
						// If it's likely just the content, not an ID
						if !strings.HasPrefix(idStr, "17") && len(idStr) < 15 {
							// Try to find the note by content
							var tempArgs = fmt.Sprintf(`{"query": %q, "user": %q, "channel": %q}`, 
								idStr, currentUser, currentChannel)
							
							// First list notes to find the ID
							registry := tools.GetRegistry()
							searchResult, err := registry.ExecuteTool("search_notes", tempArgs)
							if err == nil && strings.Contains(searchResult, "ID:") {
								// Extract ID from search result
								idLine := regexp.MustCompile(`ID:\s*(\d+)`).FindStringSubmatch(searchResult)
								if len(idLine) > 1 {
									extractedID := idLine[1]
									logger.Infof("Found note ID %s matching content: %s", extractedID, idStr)
									
									// Replace with the proper ID
									deleteArgs["id"] = extractedID
									newArgs, _ := json.Marshal(deleteArgs)
									args = string(newArgs)
								}
							}
						}
					}
				}
			}
			
			// Add user and channel if not present
			if strings.Contains(args, "\"user\":") == false && currentUser != "" {
				// Add the current user to args
				args = strings.TrimSuffix(args, "}")
				if len(args) > 2 {
					args += ","
				}
				args += fmt.Sprintf("\"user\":\"%s\",\"channel\":\"%s\"}", currentUser, currentChannel)
				logger.AIDebugf("Enhanced args for note tool: %s", args)
			}
		}
		
		toolResponse, err := tools.GetRegistry().ExecuteTool(
			toolCall.Function.Name,
			args,
		)
		
		if err != nil {
			logger.Errorf("Tool execution error: %v", err)
			toolResponse = "Error executing tool: " + err.Error()
		} else {
			logger.AIDebugf("Tool %s executed, response length: %d chars", 
				toolCall.Function.Name, len(toolResponse))
		}
		
		toolResponses = append(toolResponses, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    toolResponse,
			Name:       toolCall.Function.Name,
			ToolCallID: toolCall.ID,
		})
	}
	
	return toolResponses
}

func createChatRequest(messages []openai.ChatCompletionMessage, availableTools []openai.Tool) openai.ChatCompletionRequest {
	cfg := GetConfig()
	
	request := openai.ChatCompletionRequest{
		Model:       MapModelName(cfg.Model),
		Messages:    messages,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxResponseTokens,
	}
	
	if cfg.EnableToolCalls && len(availableTools) > 0 {
		request.Tools = availableTools
	}
	
	return request
}

func createToolFallbackResponse(messages []openai.ChatCompletionMessage) string {
	toolNames := make([]string, 0)
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleTool {
			toolNames = append(toolNames, msg.Name)
		}
	}
	
	if len(toolNames) > 0 {
		return "I've completed your request using: " + strings.Join(toolNames, ", ")
	}
	
	return "I've completed the operations but couldn't generate a final response."
}

func ProcessMessage(message string, channelPersonality string, currentChannel string, user string) (string, error) {
	if !IsInitialized() {
		return "AI processing is not available (missing OPENAI_API_KEY)", nil
	}
	
	cfg := GetConfig()
	message = strings.TrimSpace(message)
	
	// Add current channel context to the message
	if currentChannel != "" {
		// Always include the channel info, even if previously mentioned
		// This ensures the model has the current context in every message
		message = message + "\n\nCURRENT CHANNEL: " + currentChannel + 
			"\nUSER CONTEXT: This is a direct message to you (Jacey) in the IRC channel"
	}
	
	var availableTools []openai.Tool
	if cfg.EnableToolCalls {
		availableTools = tools.GetRegistry().GetOpenAITools()
	}
	
	// Create system prompt with channel personality if provided
	systemPrompt := cfg.SystemPrompt
	if channelPersonality != "" {
		systemPrompt = systemPrompt + "\n\nChannel-specific personality: " + channelPersonality
	}
	
	// Add user notes to system prompt if available
	if user != "" {
		userNotes := tools.GetUserNotes(user)
		if userNotes != "" {
			systemPrompt = systemPrompt + "\n\n" + userNotes
		}
	}
	
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: message},
	}
	
	// Initial API call
	ctx, cancel := CreateContext()
	defer cancel()
	
	request := createChatRequest(messages, availableTools)
	resp, err := GetClient().CreateChatCompletion(ctx, request)
	if err != nil {
		logger.Errorf("OpenAI API error: %v", err)
		return "Sorry, I encountered an error processing your request.", err
	}
	
	aiMessage := resp.Choices[0].Message
	messages = append(messages, aiMessage)
	
	// If no tool calls, return the response
	if len(aiMessage.ToolCalls) == 0 {
		return aiMessage.Content, nil
	}
	
	// Process initial tool calls
	toolResponses := processToolCalls(aiMessage, user, currentChannel)
	messages = append(messages, toolResponses...)
	
	// Handle multiple iterations of tool calls
	const maxIterations = 3
	for iteration := 0; iteration < maxIterations; iteration++ {
		ctx, cancel := CreateContext()
		
		request := createChatRequest(messages, availableTools)
		resp, err := GetClient().CreateChatCompletion(ctx, request)
		cancel()
		
		if err != nil {
			logger.Errorf("OpenAI API error (iteration %d): %v", iteration, err)
			return "Sorry, I encountered an error processing the tool response.", err
		}
		
		aiMessage := resp.Choices[0].Message
		messages = append(messages, aiMessage)
		
		// If more tool calls, process them and continue
		if len(aiMessage.ToolCalls) > 0 {
			logger.Infof("Found %d additional tool calls in iteration %d", 
				len(aiMessage.ToolCalls), iteration)
			
			toolResponses := processToolCalls(aiMessage, user, currentChannel)
			messages = append(messages, toolResponses...)
			continue
		}
		
		// No more tool calls, return the final response
		if aiMessage.Content == "" {
			logger.Warnf("Empty AI response after tool execution (iteration %d)", iteration)
			return createToolFallbackResponse(messages), nil
		}
		
		return aiMessage.Content, nil
	}
	
	// If we've reached the maximum iterations
	logger.Warnf("Reached maximum tool call iterations (%d)", maxIterations)
	return createToolFallbackResponse(messages), nil
}