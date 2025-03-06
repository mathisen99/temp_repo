package api

import (
	"gopkg.in/irc.v4"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CommandHelp represents documentation for a plugin command
type CommandHelp struct {
	Command     string
	Description string
	Usage       string
	Examples    []string
	Permission  UserLevel
}

// PluginMetadata contains information about a plugin
type PluginMetadata struct {
	Name        string
	Version     string
	Author      string
	Description string
	Commands    []CommandHelp
}

// EnsurePluginDataDir ensures the data directory for a plugin exists
func EnsurePluginDataDir(pluginName string) (string, error) {
	dir := GetPluginDataDir(pluginName)
	err := os.MkdirAll(dir, 0755)
	return dir, err
}

// SavePluginData saves data to a file in the plugin's data directory
func SavePluginData(pluginName, fileName string, data []byte) error {
	dir, err := EnsurePluginDataDir(pluginName)
	if err != nil {
		return err
	}
	
	filePath := filepath.Join(dir, fileName)
	return os.WriteFile(filePath, data, 0644)
}

// LoadPluginData loads data from a file in the plugin's data directory
func LoadPluginData(pluginName, fileName string) ([]byte, error) {
	dir := GetPluginDataDir(pluginName)
	filePath := filepath.Join(dir, fileName)
	return os.ReadFile(filePath)
}

// MessageHandler is a function that handles IRC messages
type MessageHandler func(client *irc.Client, message *irc.Message)

// CommandHandler is a function that handles bot commands
type CommandHandler func(client *irc.Client, message *irc.Message, args []string)

// ScheduledTask represents a task that executes on a schedule
type ScheduledTask struct {
	Name     string
	Interval time.Duration
	LastRun  time.Time
	Action   func() error
}

// IsAdmin checks if an IRC message is from an admin
func IsAdmin(message *irc.Message) bool {
	return IsUserAdmin(message.Prefix.String())
}

// IsOwner checks if an IRC message is from the bot owner
func IsOwner(message *irc.Message) bool {
	return IsUserOwner(message.Prefix.String())
}

// ParseIRCMessage parses an IRC message into a structured format
// Like extracting the nickname, channel, and command components
func ParseIRCMessage(message *irc.Message) (nick, channel, text string) {
	if message.Prefix != nil {
		nick = message.Prefix.Name
	}
	
	if len(message.Params) > 0 {
		channel = message.Params[0]
	}
	
	text = message.Trailing()
	
	return
}

// ParseCommandArgs splits a command string into arguments, respecting quotes
func ParseCommandArgs(commandText string) []string {
	// Simple argument parser that respects quotes
	var args []string
	var currentArg strings.Builder
	inQuotes := false
	
	for _, char := range commandText {
		switch char {
		case '"':
			inQuotes = !inQuotes
		case ' ':
			if inQuotes {
				currentArg.WriteRune(char)
			} else if currentArg.Len() > 0 {
				args = append(args, currentArg.String())
				currentArg.Reset()
			}
		default:
			currentArg.WriteRune(char)
		}
	}
	
	if currentArg.Len() > 0 {
		args = append(args, currentArg.String())
	}
	
	return args
}

// ReplyToMessage sends a reply to an IRC message
// It figures out whether to reply to a channel or directly to a user
func ReplyToMessage(client *irc.Client, message *irc.Message, reply string) error {
	target := GetReplyTarget(client, message)
	return SendMessage(client, target, reply)
}

// ModifyChannelTopic gets or sets a channel topic
func ModifyChannelTopic(client *irc.Client, channel, newTopic string) error {
	if newTopic == "" {
		// Just request the current topic
		return client.Writef("TOPIC %s", channel)
	}
	
	// Set a new topic
	return client.Writef("TOPIC %s :%s", channel, newTopic)
}

// PunishUser performs various moderation actions on a user
// mode can be one of: kick, ban, mute
func PunishUser(client *irc.Client, channel, nick, reason, mode string) error {
	switch strings.ToLower(mode) {
	case "kick":
		return KickUser(client, channel, nick, reason)
	case "ban":
		// Set +b mode for user
		hostmask := nick + "!*@*" // Simple hostmask ban
		err := SetMode(client, channel, "+b "+hostmask)
		if err != nil {
			return err
		}
		// Also kick the user
		return KickUser(client, channel, nick, reason)
	case "mute":
		// Set +q mode for user (quiet)
		hostmask := nick + "!*@*"
		return SetMode(client, channel, "+q "+hostmask)
	default:
		return nil
	}
}

// GetCurrentDate returns the current date in the specified format
// For example, GetCurrentDate("2006-01-02") returns the date as YYYY-MM-DD
func GetCurrentDate(format string) string {
	return time.Now().Format(format)
}