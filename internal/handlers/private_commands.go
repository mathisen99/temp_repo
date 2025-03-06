package handlers

import (
	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"ircbot/internal/userlevels"
	"strings"
)

func checkIfOwner(hostmask string) bool {
	settings, err := config.LoadSettings()
	if err != nil {
		return false
	}
	
	return settings.OwnerVerified && 
		   settings.OwnerHostmask != "" && 
		   hostmask == settings.OwnerHostmask
}

func handleSetLevelCommand(c *irc.Client, m *irc.Message, message string) {
	userNick := m.Prefix.Name
	parts := strings.Fields(message)

	if len(parts) != 3 {
		response := "Invalid format. Use: !setlevel <hostmask> <level>"
		if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, userNick, response); err != nil {
			logger.Errorf("Failed to send invalid format message to %s: %v", userNick, err)
		} else {
			logger.LogPrivateMessage(userNick, "TO", response)
		}
		return
	}

	targetHostmask := parts[1]
	levelStr := strings.ToLower(parts[2])

	var level userlevels.UserLevel
	switch levelStr {
	case "owner":
		level = userlevels.Owner
	case "admin":
		level = userlevels.Admin
	case "regular":
		level = userlevels.Regular
	case "badboy":
		level = userlevels.BadBoy
	default:
		response := "Invalid level. Use: owner, admin, regular, or badboy"
		if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, userNick, response); err != nil {
			logger.Errorf("Failed to send invalid level message to %s: %v", userNick, err)
		} else {
			logger.LogPrivateMessage(userNick, "TO", response)
		}
		return
	}

	userlevels.SetUserLevelByHostmask(targetHostmask, level)

	if err := userlevels.SaveHostmasks(); err != nil {
		logger.Errorf("Failed to save hostmask levels: %v", err)
		response := "Error saving hostmask levels: " + err.Error()
		if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, userNick, response); err != nil {
			logger.Errorf("Failed to send error message to %s: %v", userNick, err)
		} else {
			logger.LogPrivateMessage(userNick, "TO", response)
		}
		return
	}

	response := "User level for " + targetHostmask + " set to " + levelStr
	if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, userNick, response); err != nil {
		logger.Errorf("Failed to send confirmation to %s: %v", userNick, err)
	} else {
		logger.LogPrivateMessage(userNick, "TO", response)
	}
	logger.Successf("User level for %s set to %s by owner", targetHostmask, levelStr)
}

func handleUnauthorizedCommand(c *irc.Client, userNick, hostmask string) {
	logger.Warnf("Potential command attempt from unauthorized user: %s, Hostmask: %s", userNick, hostmask)
	response := "You are not authorized to use bot commands."
	if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, userNick, response); err != nil {
		logger.Errorf("Failed to send unauthorized command warning to %s: %v", userNick, err)
	} else {
		logger.LogPrivateMessage(userNick, "TO", response)
	}
}

func sendDefaultResponse(c *irc.Client, userNick string) {
	response := "I am just a bot, And i am only answering in channels right now.."
	if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, userNick, response); err != nil {
		logger.Errorf("Failed to respond to private message from %s: %v", userNick, err)
	} else {
		logger.LogPrivateMessage(userNick, "TO", response)
	}
}