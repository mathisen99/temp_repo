package commands

import (
	"fmt"
	"strings"

	"gopkg.in/irc.v4"
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"ircbot/internal/security"
	"ircbot/internal/userlevels"
)

type CommandFunc func(c *irc.Client, m *irc.Message, args []string)

type Command struct {
	Name          string
	Description   string
	Handler       CommandFunc
	RequiredLevel userlevels.UserLevel
}

var commandRegistry = make(map[string]Command)

func RegisterCommand(name string, description string, requiredLevel userlevels.UserLevel, handler CommandFunc) {
	commandRegistry[name] = Command{
		Name:          name,
		Description:   description,
		Handler:       handler,
		RequiredLevel: requiredLevel,
	}
}

func GetCommand(name string) (Command, bool) {
	cmd, exists := commandRegistry[name]
	return cmd, exists
}

var CheckPluginCommands func(c *irc.Client, m *irc.Message, cmd string, args []string) bool

// BotConfig is a global variable to hold the current bot configuration
var BotConfig *config.Config

// HandleCommand processes a command and checks both user permissions and channel permissions
func HandleCommand(c *irc.Client, m *irc.Message) {
	cmdText := strings.TrimSpace(m.Trailing())
	parts := strings.Fields(cmdText)
	if len(parts) == 0 {
		return
	}

	replyTarget := m.Params[0]
	isChannelMsg := strings.HasPrefix(replyTarget, "#")
	
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
		isChannelMsg = false
	}

	baseCommand := strings.TrimPrefix(parts[0], "!")
	args := parts[1:]
	userNick := m.Prefix.Name
	hostmask := m.Prefix.String()
	
	// Check if the user is already ignored
	userLevel := userlevels.GetUserLevelByHostmask(hostmask)
	if userLevel == userlevels.Ignored {
		// Don't process commands from ignored users
		logger.Debugf("Ignored command from %s: %s", userNick, cmdText)
		return
	}
	
	// Spam protection - check command rate
	isSpamming, commandCount := security.GlobalMessageTracker.TrackCommand(hostmask, baseCommand)
	if isSpamming {
		// Log excessive command usage
		logger.Warnf("Command rate limit exceeded for %s: %d uses of '%s' in window", 
			userNick, commandCount, baseCommand)
		
		// If this is their first offense, warn them
		if security.GlobalMessageTracker.AddWarning(hostmask) {
			// If they've hit the warning threshold, auto-ignore them
			logger.Warnf("User %s (%s) has been automatically ignored for command spam", 
				userNick, hostmask)
			
			// Automatically set them to ignored level
			userlevels.SetUserLevel(hostmask, userlevels.Ignored)
			userlevels.SaveHostmasks()
			
			// Notify the user about the auto-ignore
			c.WriteMessage(&irc.Message{
				Command: "NOTICE",
				Params: []string{
					userNick,
					fmt.Sprintf("You have been automatically ignored for command spam protection."),
				},
			})
			
			// Also notify in channel if this was a channel message
			if isChannelMsg {
				c.WriteMessage(&irc.Message{
					Command: "PRIVMSG",
					Params: []string{
						replyTarget,
						fmt.Sprintf("User %s has been automatically ignored for command spam protection.", userNick),
					},
				})
			}
			return
		} else {
			// Just warn the user about rate limiting
			warningCount := security.GlobalMessageTracker.GetWarningCount(hostmask)
			threshold := security.GlobalMessageTracker.GetWarningThreshold()
			
			c.WriteMessage(&irc.Message{
				Command: "NOTICE",
				Params: []string{
					userNick,
					fmt.Sprintf("Warning (%d/%d): Please slow down command usage to avoid being automatically ignored.", 
						warningCount, threshold),
				},
			})
			
			// We'll still process this command since it's just a warning
			logger.Debugf("User %s warned for command spam (%d/%d): %s", 
				userNick, warningCount, threshold, cmdText)
		}
	}
	
	// Check if command exists
	cmd, exists := GetCommand(baseCommand)
	if !exists {
		// Try plugin commands
		if CheckPluginCommands != nil {
			handled := CheckPluginCommands(c, m, baseCommand, args)
			if handled {
				return
			}
		}
		return
	}
	
	// Check channel-specific permissions if this is a channel message
	if isChannelMsg && BotConfig != nil {
		if !config.IsCommandEnabledForChannel(BotConfig, replyTarget, baseCommand) {
			// Command is disabled for this channel
			return
		}
	}
	
	// Check user permission level
	if !userlevels.HasPermission(hostmask, cmd.RequiredLevel) {
		requiredLevel := userlevels.LevelName(cmd.RequiredLevel)
		userLevelName := userlevels.LevelName(userLevel)
		
		if cmd.RequiredLevel == userlevels.Owner {
			err := c.Writef("PRIVMSG %s :Access denied. Command '%s' requires owner access.", 
				replyTarget, baseCommand)
			if err != nil {
				return
			}
		} else {
			err := c.Writef("PRIVMSG %s :Access denied. Command '%s' requires %s level. Your level: %s", 
				replyTarget, baseCommand, requiredLevel, userLevelName)
			if err != nil {
				return
			}
		}
		return
	}
	
	// Execute the command
	cmd.Handler(c, m, args)
}
