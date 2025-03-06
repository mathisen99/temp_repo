package handlers

import (
	"fmt"
	"gopkg.in/irc.v4"
	"ircbot/internal/logger"
	"ircbot/internal/security"
	"ircbot/internal/userlevels"
	"strings"
)

func HandlePrivateMessage(c *irc.Client, m *irc.Message) {
	message := m.Trailing()
	userNick := m.Prefix.Name
	hostmask := m.Prefix.String()
	
	// Check if user is ignored - if so, don't process any interactions
	userLevel := userlevels.GetUserLevelByHostmask(hostmask)
	if userLevel == userlevels.Ignored {
		// Just log the message without responding
		logger.LogPrivateMessage(userNick, "FROM (IGNORED)", message)
		return
	}
	
	// Spam protection - check message rate
	isSpamming, messageCount := security.GlobalMessageTracker.TrackMessage(hostmask)
	if isSpamming {
		// Log excessive private messages
		logger.Warnf("Private message rate limit exceeded for %s: %d messages in window", 
			userNick, messageCount)
		
		// If this is their first offense, warn them
		if security.GlobalMessageTracker.AddWarning(hostmask) {
			// If they've hit the warning threshold, auto-ignore them
			logger.Warnf("User %s (%s) has been automatically ignored for private message spam", 
				userNick, hostmask)
			
			// Automatically set them to ignored level
			userlevels.SetUserLevel(hostmask, userlevels.Ignored)
			userlevels.SaveHostmasks()
			
			// Notify the user about the auto-ignore
			c.WriteMessage(&irc.Message{
				Command: "NOTICE",
				Params: []string{
					userNick,
					fmt.Sprintf("You have been automatically ignored for spam protection."),
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
			logger.Debugf("User %s warned for private message spam (%d/%d)", 
				userNick, warningCount, threshold)
		}
	}
	
	isPending := checkPendingVerification(userNick)
	isOwner := checkIfOwner(hostmask)
	isSetLevelCmd := strings.HasPrefix(message, "!setlevel ")
	isCommand := strings.HasPrefix(message, "!")

	// Log incoming private message
	logger.LogPrivateMessage(userNick, "FROM", message)

	switch {
	case isPending:
		handleOwnerVerification(c, userNick, message, m)
	case isOwner && isSetLevelCmd:
		handleSetLevelCommand(c, m, message)
	case isCommand && !isOwner:
		handleUnauthorizedCommand(c, userNick, hostmask)
	case strings.TrimSpace(message) == "":
		logger.Debugf("Received empty private message from %s; skipping response.", userNick)
	default:
		sendDefaultResponse(c, userNick)
	}
}
