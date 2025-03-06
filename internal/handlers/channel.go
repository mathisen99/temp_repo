package handlers

import (
	"bufio"
	"fmt"
	"ircbot/internal/commands"
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"ircbot/internal/security"
	"ircbot/internal/userlevels"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/irc.v4"
)

func HandleChannelMessage(c *irc.Client, m *irc.Message) {
	message := m.Trailing()
	userNick := m.Prefix.Name
	channel := m.Params[0]
	
	// Check if user is ignored - if so, don't process any interactions
	hostmask := m.Prefix.String()
	userLevel := userlevels.GetUserLevelByHostmask(hostmask)
	if userLevel == userlevels.Ignored {
		// Still log the message but don't interact with ignored users
		logger.ChanMsgf("%s | %s (IGNORED): %s", channel, userNick, message)
		return
	}
	
	// Spam protection - check message rate
	isSpamming, messageCount := security.GlobalMessageTracker.TrackMessage(hostmask)
	if isSpamming {
		// Log excessive messages
		logger.Warnf("Rate limit exceeded for %s in %s: %d messages in window", 
			userNick, channel, messageCount)
		
		// If this is their first offense, warn them
		if security.GlobalMessageTracker.AddWarning(hostmask) {
			// If they've hit the warning threshold, auto-ignore them
			logger.Warnf("User %s (%s) has been automatically ignored for spam", 
				userNick, hostmask)
			
			// Automatically set them to ignored level
			userlevels.SetUserLevel(hostmask, userlevels.Ignored)
			userlevels.SaveHostmasks()
			
			// Notify the channel about the auto-ignore
			c.WriteMessage(&irc.Message{
				Command: "PRIVMSG",
				Params: []string{
					channel,
					fmt.Sprintf("User %s has been automatically ignored for spam protection.", userNick),
				},
			})
			return
		} else {
			// Just warn the user about rate limiting
			warningCount := security.GlobalMessageTracker.GetWarningCount(hostmask)
			threshold := security.GlobalMessageTracker.GetWarningThreshold()
			
			c.WriteMessage(&irc.Message{
				Command: "NOTICE",
				Params: []string{
					userNick,
					fmt.Sprintf("Warning (%d/%d): Please slow down to avoid being automatically ignored.", 
						warningCount, threshold),
				},
			})
			
			// Still log and process the message
			logger.ChanMsgf("%s | %s (WARNING %d/%d): %s", 
				channel, userNick, warningCount, threshold, message)
		}
	}
	
	isBotMentioned := containsNick(c.CurrentNick(), message)
	isCommand := strings.HasPrefix(message, "!")
	hasURL := CheckForURL(message)

	// Log all channel messages to file
	logger.LogChannelMessage(channel, userNick, message)

	switch {
	case isBotMentioned:
		// Check if AI is enabled for nick mentions in this channel
		useAI := false
		if commands.BotConfig != nil {
			// Check if AI should be used for nick mentions (default to true)
			aiSetting := config.GetChannelSetting(commands.BotConfig, channel, "use_ai_for_mentions", true)
			if boolValue, ok := aiSetting.(bool); ok {
				useAI = boolValue
			} else if strValue, ok := aiSetting.(string); ok {
				useAI = strValue == "true" || strValue == "1" || strValue == "yes"
			}
		}

		if useAI {
			// Keep original message with bot name for natural processing
			originalQuestion := message

			// For cases where the message only contains the bot name, add a greeting
			if strings.TrimSpace(removeBotNick(c.CurrentNick(), message)) == "" {
				originalQuestion = "Hello"
				logger.Debugf("Bot mentioned with no content, adding default greeting")
			}

			// Get recent channel context
			channelContext, err := getRecentChannelContext(channel, 50)
			if err != nil {
				logger.Errorf("Failed to get channel context: %v", err)
			}

			// Format the final message with context, making it clearer to the AI
			contextualQuestion := fmt.Sprintf(
				"Reply to the user query below in a direct conversational style. Don't reference yourself in third person. Keep your response concise and match the brevity of the user's message. Don't add follow-up questions unless absolutely necessary.\n\n"+
				"USER QUERY: %s\n\n"+
				"CURRENT CHANNEL: %s\n\n"+
				"CHANNEL CONTEXT (for your awareness only, don't reference this directly):\n%s",
				originalQuestion, channel, channelContext)

			// Use AI with the contextual message
			commands.HandleAIResponse(c, channel, userNick, contextualQuestion, channel)
		} else {
			// Log the mention but don't respond if AI is disabled
			logger.Debugf("Ignored mention in %s from %s (AI for mentions disabled)", channel, userNick)
		}
		return
	case isCommand:
		commands.HandleCommand(c, m)
		return
	case hasURL:
		// Check if URL logging is enabled for this channel
		logURLs := true
		if commands.BotConfig != nil {
			// Get the channel setting, default to true if not specified
			setting := config.GetChannelSetting(commands.BotConfig, channel, "log_urls", true)
			// Try to convert to a boolean
			if boolValue, ok := setting.(bool); ok {
				logURLs = boolValue
			} else if strValue, ok := setting.(string); ok {
				logURLs = !(strValue == "false" || strValue == "0" || strValue == "no")
			}
		}

		if logURLs {
			logger.Infof("User %s sent a URL in channel %s: %s", userNick, channel, message)
		}
		return
	default:
		logger.ChanMsgf("%s | %s: %s", channel, userNick, message)
	}
}

// removeBotNick removes the bot's nickname from the message to extract the question
func removeBotNick(botNick string, message string) string {
	// Convert both to lowercase for case-insensitive matching
	lowerMessage := strings.ToLower(message)
	lowerNick := strings.ToLower(botNick)

	// Handle common patterns of addressing the bot
	patterns := []string{
		lowerNick + ":",
		lowerNick + ",",
		lowerNick + " ",
		lowerNick + "?",
		lowerNick + "!",
		"@" + lowerNick,
		lowerNick + ".",
	}

	// Replace each pattern with an empty string
	for _, pattern := range patterns {
		lowerMessage = strings.ReplaceAll(lowerMessage, pattern, "")
	}

	// If none of the patterns matched, try a simple replacement
	if strings.Contains(lowerMessage, lowerNick) {
		lowerMessage = strings.ReplaceAll(lowerMessage, lowerNick, "")
	}

	// Use the processed lowercase message to extract the same portion from the original message
	// This preserves the original case in the extracted question
	startIndex := len(message) - len(lowerMessage)
	if startIndex >= 0 && startIndex < len(message) {
		message = message[startIndex:]
	}

	// Trim any leftover whitespace or punctuation
	message = strings.TrimSpace(message)
	message = strings.TrimLeft(message, ",:;.!?@ ")
	message = strings.TrimSpace(message)

	return message
}

func CheckForURL(message string) bool {
	if strings.Contains(message, "http") {
		return true
	}
	return false
}

// getRecentChannelContext retrieves the recent context from a channel's log
func getRecentChannelContext(channel string, lineCount int) (string, error) {
	// Construct the log file path for today
	baseDir := filepath.Join("logs", "CHANNEL")
	today := time.Now().Format("2006-01-02")
	
	// Make sure channel name starts with #
	if !strings.HasPrefix(channel, "#") {
		channel = "#" + channel
	}
	
	logDir := filepath.Join(baseDir, channel)
	logFile := filepath.Join(logDir, today+".log")
	
	// Check if log file exists
	fileExists := false
	if _, err := os.Stat(logFile); err == nil {
		fileExists = true
	} else if os.IsNotExist(err) {
		// Try looking for the channel with different case
		entries, readErr := os.ReadDir(baseDir)
		if readErr != nil {
			return "No previous messages found.", nil
		}
		
		for _, entry := range entries {
			if entry.IsDir() && strings.EqualFold(entry.Name(), channel) {
				logDir = filepath.Join(baseDir, entry.Name())
				logFile = filepath.Join(logDir, today+".log")
				
				// Check if this file exists
				if _, statErr := os.Stat(logFile); statErr == nil {
					fileExists = true
					break
				}
				
				// Try yesterday's log if today's doesn't exist
				yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
				yesterdayFile := filepath.Join(logDir, yesterday+".log")
				if _, statErr := os.Stat(yesterdayFile); statErr == nil {
					logFile = yesterdayFile
					fileExists = true
					break
				}
			}
		}
	}
	
	if !fileExists {
		// No logs exist yet, return empty context
		return "No previous messages found.", nil
	}
	
	// Open the log file
	file, err := os.Open(logFile)
	if err != nil {
		return "", fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()
	
	// Read the file line by line and store the last N lines
	scanner := bufio.NewScanner(file)
	var lines []string
	
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		
		// Keep only the last N lines
		if len(lines) > lineCount {
			lines = lines[1:]
		}
	}
	
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading log file: %v", err)
	}
	
	// If no lines were found
	if len(lines) == 0 {
		return "No previous messages found.", nil
	}
	
	// Format the context
	context := strings.Join(lines, "\n")
	
	return context, nil
}
