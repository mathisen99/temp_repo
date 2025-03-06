package commands

import (
	"fmt"
	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/ai"
	"ircbot/internal/ai/tools"
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"regexp"
	"strings"
	"time"
)

// HandleAIResponse processes an AI query and sends the response to IRC
// This function is exported so it can be used from other packages
func HandleAIResponse(c *irc.Client, channel string, nick string, question string, replyTarget string) {
	// Log that we're processing an AI request (to AI log file instead of error.log)
	logger.AIDebugf("Processing AI request from %s in %s: %s", nick, channel, question)

	// Format the question with context - improved prompt format
	formattedQuestion := "Reply to this user query in a direct conversational style. Don't mention yourself in third person or explain what you're doing. Keep your response concise and don't add unnecessary follow-up questions at the end.\n\n" +
		"USER QUERY: " + question + "\n\n" +
		"CURRENT CHANNEL: " + channel

	var response string
	var err error

	// Special case handling for "use your channel log tool"
	if strings.HasPrefix(strings.ToLower(formattedQuestion), "use your channel log tool") {
		// Extract channel name from the command
		channelName := extractChannelName(formattedQuestion)
		if channelName == "" && channel != c.CurrentNick() {
			// Default to current channel if no channel specified
			channelName = channel
		}

		if channelName != "" {
			response, err = handleChannelLogTool(channelName)
			if err != nil {
				c.Writef("%s %s :Error accessing channel logs: %v", internal.CMD_PRIVMSG, replyTarget, err)
				return
			}
		}
	} else {
		// Get channel-specific personality if available
		channelPersonality := ""
		logger.AIDebugf("BotConfig is nil: %v", BotConfig == nil)
		if BotConfig != nil {
			personality := config.GetChannelSetting(BotConfig, channel, "personality", "")
			logger.AIDebugf("Got personality for channel %s: %v (type: %T)", channel, personality, personality)
			if strValue, ok := personality.(string); ok && strValue != "" {
				channelPersonality = strValue
				logger.AIDebugf("Using channel personality: %s", channelPersonality)
			}
		}
		
		// Normal AI processing for other queries with channel personality
		response, err = ai.ProcessMessage(formattedQuestion, channelPersonality, channel, nick)
	}

	if err != nil {
		logger.Errorf("Error processing AI request: %v", err)
		c.Writef("%s %s :Error processing AI request: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// Handle empty responses
	if response == "" {
		logger.Warnf("Received empty response from AI processing")
		errorMsg := "The AI completed your request but didn't provide a text response."
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, errorMsg)
		// Log the bot's error message
		logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), errorMsg)
		return
	}

	// Sanitize the response for IRC
	cleanedResponses := sanitizeForIRC(response)
	if len(cleanedResponses) == 0 {
		logger.Warnf("Sanitized response is empty")
		emptyMsg := "The AI completed your request but returned an empty response."
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, emptyMsg)
		// Log the bot's error message
		logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), emptyMsg)
		return
	}

	// Constants for IRC response
	const maxIRCMessages = 3
	messageDelay := 800 * time.Millisecond

	// If response is longer than maxIRCMessages, use paste service
	if len(cleanedResponses) > maxIRCMessages {
		pasteURL, pasteErr := PasteService(response)

		if pasteErr != nil {
			// If paste service fails, fallback to truncated IRC messages
			logger.Warnf("Paste service error: %v", pasteErr)
			
			// First message
			firstMsg := cleanedResponses[0]
			c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, firstMsg)
			// Log the bot's message
			logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), firstMsg)

			// Send some of the remaining messages with delay
			for i := 1; i < len(cleanedResponses) && i < maxIRCMessages; i++ {
				time.Sleep(messageDelay)
				nextMsg := cleanedResponses[i]
				c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, nextMsg)
				// Log each of the bot's messages
				logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), nextMsg)
			}

			// Add note about truncation
			if len(cleanedResponses) > maxIRCMessages {
				time.Sleep(messageDelay)
				truncMsg := fmt.Sprintf("(Response truncated, %d more parts omitted)", 
					len(cleanedResponses)-maxIRCMessages)
				c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, truncMsg)
				// Log the truncation notice
				logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), truncMsg)
			}
		} else {
			// Send summary and paste URL
			pasteMsg := fmt.Sprintf("My full response at %s", pasteURL)
			c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, pasteMsg)
			// Log the paste URL message
			logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), pasteMsg)
		}
	} else {
		// For shorter responses, send directly to IRC with delay between messages
		firstMsg := cleanedResponses[0]
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, firstMsg)
		// Log the bot's first message
		logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), firstMsg)

		// Send remaining messages with delay
		for i := 1; i < len(cleanedResponses); i++ {
			time.Sleep(messageDelay)
			nextMsg := cleanedResponses[i]
			c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, nextMsg)
			// Log each of the bot's messages
			logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), nextMsg)
		}
	}

	// Log that we've completed processing
	logger.AIDebugf("Completed AI request for %s in %s", nick, channel)
}

func aiCmd(c *irc.Client, m *irc.Message, args []string) {
	channel := m.Params[0]
	nick := m.Prefix.Name
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	if len(args) == 0 {
		usage := "Usage: !ai <your question or message>"
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, usage)
		// Log the bot's message
		logger.LogBotChannelMessage(replyTarget, c.CurrentNick(), usage)
		return
	}

	question := strings.Join(args, " ")

	// Use the shared AI response handler
	HandleAIResponse(c, channel, nick, question, replyTarget)
}

// Note: sanitizeForIRC is already defined in helpers.go

// extractChannelName tries to extract a channel name from the user's command
func extractChannelName(command string) string {
	// Look for "for #channel" pattern
	forPattern := regexp.MustCompile(`for\s+#(\S+)`)
	matches := forPattern.FindStringSubmatch(command)
	if len(matches) > 1 {
		return "#" + matches[1]
	}

	// Look for "in #channel" pattern
	inPattern := regexp.MustCompile(`in\s+#(\S+)`)
	matches = inPattern.FindStringSubmatch(command)
	if len(matches) > 1 {
		return "#" + matches[1]
	}

	// Look for "#channel" pattern
	channelPattern := regexp.MustCompile(`#(\S+)`)
	matches = channelPattern.FindStringSubmatch(command)
	if len(matches) > 0 {
		return "#" + matches[1]
	}

	return ""
}

// handleChannelLogTool directly invokes the channel log tool
func handleChannelLogTool(channelName string) (string, error) {
	logger.AIDebugf("Direct channel log tool call for: %s", channelName)

	// Get today's date
	today := time.Now().Format("2006-01-02")

	// Create arguments for the tool
	args := fmt.Sprintf(`{"channelName": "%s", "date": "%s"}`, channelName, today)

	// Get the tool registry and execute the tool
	registry := tools.GetRegistry()
	toolResponse, err := registry.ExecuteTool("getChannelLog", args)
	if err != nil {
		logger.Errorf("Error executing channel log tool: %v", err)
		return "", err
	}

	logger.AIDebugf("Channel log tool response length: %d", len(toolResponse))

	// If the response is just a "no logs found" message, try yesterday
	if strings.Contains(toolResponse, "No logs found") {
		logger.AIDebugf("No logs found for today, trying yesterday")
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		args = fmt.Sprintf(`{"channelName": "%s", "date": "%s"}`, channelName, yesterday)
		toolResponse, err = registry.ExecuteTool("getChannelLog", args)
		if err != nil {
			logger.Errorf("Error executing channel log tool for yesterday: %v", err)
			return "", err
		}
		logger.AIDebugf("Yesterday's channel log tool response length: %d", len(toolResponse))
	}

	return toolResponse, nil
}

// personalityCmd allows setting a channel-specific personality for the AI
func personalityCmd(c *irc.Client, m *irc.Message, args []string) {
	if BotConfig == nil {
		errorMsg := "Bot configuration is not available"
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, m.Params[0], errorMsg)
		// Log the bot's message
		logger.LogBotChannelMessage(m.Params[0], c.CurrentNick(), errorMsg)
		return
	}

	channel := m.Params[0]
	if channel == c.CurrentNick() {
		// If this is a private message, we can't set channel personality
		errorMsg := "This command must be used in a channel"
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, m.Prefix.Name, errorMsg)
		return
	}

	// Clear personality if no arguments provided
	if len(args) == 0 {
		config.UpdateChannelSetting(BotConfig, channel, "personality", "")
		if err := config.SaveChannelSettings(BotConfig); err != nil {
			logger.Errorf("Error saving channel settings: %v", err)
			errorMsg := fmt.Sprintf("Error saving channel settings: %v", err)
			c.Writef("%s %s :%s", internal.CMD_PRIVMSG, channel, errorMsg)
			// Log the bot's error message
			logger.LogBotChannelMessage(channel, c.CurrentNick(), errorMsg)
			return
		}
		successMsg := "Channel personality has been cleared"
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, channel, successMsg)
		// Log the bot's message
		logger.LogBotChannelMessage(channel, c.CurrentNick(), successMsg)
		return
	}

	// Set the new personality
	personality := strings.Join(args, " ")
	config.UpdateChannelSetting(BotConfig, channel, "personality", personality)
	
	// Save channel settings to ensure persistence
	if err := config.SaveChannelSettings(BotConfig); err != nil {
		logger.Errorf("Error saving channel settings: %v", err)
		errorMsg := fmt.Sprintf("Error saving channel settings: %v", err)
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, channel, errorMsg)
		// Log the bot's error message
		logger.LogBotChannelMessage(channel, c.CurrentNick(), errorMsg)
		return
	}

	successMsg := "Channel personality has been set"
	c.Writef("%s %s :%s", internal.CMD_PRIVMSG, channel, successMsg)
	// Log the bot's message
	logger.LogBotChannelMessage(channel, c.CurrentNick(), successMsg)
}