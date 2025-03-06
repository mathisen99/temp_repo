package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"ircbot/internal/security"
	"ircbot/internal/userlevels"
)

func setLevelCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	if len(args) < 2 {
		c.Writef("%s %s :Usage: !setlevel <user> <level>", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :Available levels: owner, admin, regular, badboy, ignored", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	target := args[0]
	levelStr := strings.ToLower(args[1])

	currentLevel := userlevels.Regular

	allHostmasks := userlevels.GetAllHostmasks()
	for hostmask, level := range allHostmasks {
		if strings.HasPrefix(hostmask, target+"!") {
			if level > currentLevel {
				currentLevel = level
			}
		}
	}

	currentLevelName := userlevels.LevelName(currentLevel)

	var level userlevels.UserLevel
	var validLevel bool

	switch levelStr {
	case "owner":
		level = userlevels.Owner
		validLevel = true
	case "admin":
		level = userlevels.Admin
		validLevel = true
	case "regular", "user", "normal":
		level = userlevels.Regular
		validLevel = true
	case "badboy", "bad":
		level = userlevels.BadBoy
		validLevel = true
	case "ignored", "ignore":
		level = userlevels.Ignored
		validLevel = true
	default:
		validLevel = false
	}

	if !validLevel {
		c.Writef("%s %s :Unknown level: %s. Available levels: owner, admin, regular, badboy, ignored",
			internal.CMD_PRIVMSG, replyTarget, levelStr)
		return
	}

	callerHostmask := m.Prefix.String()
	if !userlevels.HasPermission(callerHostmask, userlevels.Owner) {
		c.Writef("%s %s :You need owner privileges to change user levels.", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	var targetHostmasks []string
	for hostmask := range allHostmasks {
		if strings.HasPrefix(hostmask, target+"!") {
			targetHostmasks = append(targetHostmasks, hostmask)
		}
	}

	if len(targetHostmasks) == 0 {
		wildcardHostmask := target + "!*@*"
		userlevels.SetUserLevelByHostmask(wildcardHostmask, level)
		
		userlevels.SetUserLevel(target, level)
		
		c.Writef("%s %s :No hostmask found for user %s. Setting wildcard hostmask (%s).",
			internal.CMD_PRIVMSG, replyTarget, target, wildcardHostmask)
	} else {
		for _, hostmask := range targetHostmasks {
			userlevels.SetUserLevelByHostmask(hostmask, level)
		}
		
		userlevels.SetUserLevel(target, level)
	}

	if err := userlevels.SaveHostmasks(); err != nil {
		logger.Errorf("Failed to save hostmask levels: %v", err)
		c.Writef("%s %s :Error saving user levels: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	c.Writef("%s %s :User %s level changed from %s to %s",
		internal.CMD_PRIVMSG, replyTarget, target, currentLevelName, userlevels.LevelName(level))
}

func restartCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	hostmask := m.Prefix.String()
	if !userlevels.HasPermission(hostmask, userlevels.Admin) {
		c.Writef("%s %s :You need admin privileges to restart the bot.", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	logger.Infof("Restart requested by %s, restarting now...", m.Prefix.Name)
	c.Writef("%s %s :Restarting bot as requested by %s...", internal.CMD_PRIVMSG, replyTarget, m.Prefix.Name)

	workingDir, err := os.Getwd()
	if err != nil {
		logger.Errorf("Failed to get working directory: %v", err)
		c.Writef("%s %s :Failed to restart: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	possiblePaths := []string{
		"./mbot",                 // Linux/Mac binary
		"./mbot.exe",             // Windows binary
		workingDir + "/mbot",     // Absolute path Linux/Mac
		workingDir + "/mbot.exe", // Absolute path Windows
	}

	var execPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			execPath = path
			break
		}
	}

	if execPath == "" {
		path, err := os.Executable()
		if err != nil {
			logger.Errorf("Failed to get executable path: %v", err)
			c.Writef("%s %s :Failed to restart: Could not find executable", internal.CMD_PRIVMSG, replyTarget)
			return
		}
		execPath = path
	}

	logger.Infof("Restarting using binary: %s", execPath)

	env := os.Environ()

	cmd := exec.Command(execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = env
	cmd.Dir = workingDir

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		logger.Errorf("Failed to start new instance: %v", err)
		c.Writef("%s %s :Failed to restart: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func dieCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	hostmask := m.Prefix.String()
	if !userlevels.HasPermission(hostmask, userlevels.Owner) {
		c.Writef("%s %s :You need owner privileges to shut down the bot.", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	shutdownMessage := "Shutting down as requested by " + m.Prefix.Name
	if len(args) > 0 {
		shutdownMessage = strings.Join(args, " ")
	}

	logger.Infof("Shutdown requested by %s: %s", m.Prefix.Name, shutdownMessage)
	c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, shutdownMessage)

	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

// handleChannelCommand manages channel-specific settings
func channelCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	if len(args) < 1 {
		c.Writef("%s %s :Usage: !channel <list|info|enable|disable|set|save> [channel] [args...]", 
			internal.CMD_PRIVMSG, replyTarget)
		return
	}

	// Make sure we have a configuration
	if BotConfig == nil {
		c.Writef("%s %s :Error: Bot configuration is not available", 
			internal.CMD_PRIVMSG, replyTarget)
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		// List all channels with settings
		if BotConfig.ChannelSettings == nil || len(BotConfig.ChannelSettings) == 0 {
			c.Writef("%s %s :No channel-specific settings configured", 
				internal.CMD_PRIVMSG, replyTarget)
			return
		}

		var channels []string
		for channel := range BotConfig.ChannelSettings {
			channels = append(channels, channel)
		}

		c.Writef("%s %s :Channels with specific settings: %s", 
			internal.CMD_PRIVMSG, replyTarget, strings.Join(channels, ", "))

	case "info":
		// Show settings for a specific channel
		if len(args) < 2 {
			c.Writef("%s %s :Usage: !channel info <channel>", 
				internal.CMD_PRIVMSG, replyTarget)
			return
		}

		channel := args[1]
		if !strings.HasPrefix(channel, "#") {
			channel = "#" + channel
		}
		
		channelCfg, exists := BotConfig.ChannelSettings[channel]
		if !exists {
			c.Writef("%s %s :No settings found for channel %s", 
				internal.CMD_PRIVMSG, replyTarget, channel)
			return
		}

		// Show enabled commands
		enabled := "all"
		if len(channelCfg.EnabledCommands) > 0 {
			enabled = strings.Join(channelCfg.EnabledCommands, ", ")
		}
		
		// Show disabled commands
		disabled := "none"
		if len(channelCfg.DisabledCommands) > 0 {
			disabled = strings.Join(channelCfg.DisabledCommands, ", ")
		}

		// Show custom settings
		settings := "none"
		if len(channelCfg.Settings) > 0 {
			settingsList := make([]string, 0, len(channelCfg.Settings))
			for k, v := range channelCfg.Settings {
				settingsList = append(settingsList, fmt.Sprintf("%s=%v", k, v))
			}
			settings = strings.Join(settingsList, ", ")
		}

		c.Writef("%s %s :Channel %s settings:", 
			internal.CMD_PRIVMSG, replyTarget, channel)
		c.Writef("%s %s :Enabled commands: %s", 
			internal.CMD_PRIVMSG, replyTarget, enabled)
		c.Writef("%s %s :Disabled commands: %s", 
			internal.CMD_PRIVMSG, replyTarget, disabled)
		c.Writef("%s %s :Custom settings: %s", 
			internal.CMD_PRIVMSG, replyTarget, settings)

	case "enable":
		// Enable a command for a specific channel
		if len(args) < 3 {
			c.Writef("%s %s :Usage: !channel enable <channel> <command>", 
				internal.CMD_PRIVMSG, replyTarget)
			return
		}

		channel := args[1]
		if !strings.HasPrefix(channel, "#") {
			channel = "#" + channel
		}
		
		command := strings.TrimPrefix(args[2], "!")

		// Initialize channel settings if needed
		if BotConfig.ChannelSettings == nil {
			BotConfig.ChannelSettings = make(map[string]config.ChannelConfig)
		}

		channelCfg, exists := BotConfig.ChannelSettings[channel]
		if !exists {
			channelCfg = config.ChannelConfig{
				EnabledCommands: []string{},
				DisabledCommands: []string{},
				Settings: make(map[string]interface{}),
			}
		}

		// Remove from disabled commands if present
		var newDisabled []string
		for _, cmd := range channelCfg.DisabledCommands {
			if cmd != command {
				newDisabled = append(newDisabled, cmd)
			}
		}
		channelCfg.DisabledCommands = newDisabled

		// Add to enabled commands if not already present
		alreadyEnabled := false
		for _, cmd := range channelCfg.EnabledCommands {
			if cmd == command {
				alreadyEnabled = true
				break
			}
		}
		if !alreadyEnabled {
			channelCfg.EnabledCommands = append(channelCfg.EnabledCommands, command)
		}

		BotConfig.ChannelSettings[channel] = channelCfg

		// Save the updated settings
		if err := config.SaveChannelSettings(BotConfig); err != nil {
			logger.Errorf("Failed to save channel settings: %v", err)
			c.Writef("%s %s :Error saving channel settings: %v", 
				internal.CMD_PRIVMSG, replyTarget, err)
			return
		}

		c.Writef("%s %s :Command '%s' enabled for channel %s", 
			internal.CMD_PRIVMSG, replyTarget, command, channel)

	case "disable":
		// Disable a command for a specific channel
		if len(args) < 3 {
			c.Writef("%s %s :Usage: !channel disable <channel> <command>", 
				internal.CMD_PRIVMSG, replyTarget)
			return
		}

		channel := args[1]
		if !strings.HasPrefix(channel, "#") {
			channel = "#" + channel
		}
		
		command := strings.TrimPrefix(args[2], "!")

		// Initialize channel settings if needed
		if BotConfig.ChannelSettings == nil {
			BotConfig.ChannelSettings = make(map[string]config.ChannelConfig)
		}

		channelCfg, exists := BotConfig.ChannelSettings[channel]
		if !exists {
			channelCfg = config.ChannelConfig{
				EnabledCommands: []string{},
				DisabledCommands: []string{},
				Settings: make(map[string]interface{}),
			}
		}

		// Remove from enabled commands if present
		var newEnabled []string
		for _, cmd := range channelCfg.EnabledCommands {
			if cmd != command {
				newEnabled = append(newEnabled, cmd)
			}
		}
		channelCfg.EnabledCommands = newEnabled

		// Add to disabled commands if not already present
		alreadyDisabled := false
		for _, cmd := range channelCfg.DisabledCommands {
			if cmd == command {
				alreadyDisabled = true
				break
			}
		}
		if !alreadyDisabled {
			channelCfg.DisabledCommands = append(channelCfg.DisabledCommands, command)
		}

		BotConfig.ChannelSettings[channel] = channelCfg

		// Save the updated settings
		if err := config.SaveChannelSettings(BotConfig); err != nil {
			logger.Errorf("Failed to save channel settings: %v", err)
			c.Writef("%s %s :Error saving channel settings: %v", 
				internal.CMD_PRIVMSG, replyTarget, err)
			return
		}

		c.Writef("%s %s :Command '%s' disabled for channel %s", 
			internal.CMD_PRIVMSG, replyTarget, command, channel)

	case "set":
		// Set a custom setting for a channel
		if len(args) < 4 {
			c.Writef("%s %s :Usage: !channel set <channel> <key> <value>", 
				internal.CMD_PRIVMSG, replyTarget)
			return
		}

		channel := args[1]
		if !strings.HasPrefix(channel, "#") {
			channel = "#" + channel
		}
		
		key := args[2]
		value := args[3]

		// Use the helper function to update the setting
		config.UpdateChannelSetting(BotConfig, channel, key, value)

		// Save the updated settings
		if err := config.SaveChannelSettings(BotConfig); err != nil {
			logger.Errorf("Failed to save channel settings: %v", err)
			c.Writef("%s %s :Error saving channel settings: %v", 
				internal.CMD_PRIVMSG, replyTarget, err)
			return
		}

		c.Writef("%s %s :Set '%s=%s' for channel %s", 
			internal.CMD_PRIVMSG, replyTarget, key, value, channel)

	case "save":
		// Explicitly save the channel settings
		if err := config.SaveChannelSettings(BotConfig); err != nil {
			logger.Errorf("Failed to save channel settings: %v", err)
			c.Writef("%s %s :Error saving channel settings: %v", 
				internal.CMD_PRIVMSG, replyTarget, err)
			return
		}
		
		c.Writef("%s %s :Channel settings saved successfully", 
			internal.CMD_PRIVMSG, replyTarget)
			
	default:
		c.Writef("%s %s :Unknown channel subcommand: %s", 
			internal.CMD_PRIVMSG, replyTarget, subcommand)
	}
}

// ignoreUserCmd is a simple wrapper around setLevelCmd that sets a user to the Ignored level
func ignoreUserCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		replyTarget := m.Params[0]
		if replyTarget == c.CurrentNick() {
			replyTarget = m.Prefix.Name
		}
		c.Writef("%s %s :Usage: !ignore <nick>", internal.CMD_PRIVMSG, replyTarget)
		return
	}
	
	// Create new args for setLevelCmd with the "ignored" level
	newArgs := []string{args[0], "ignored"}
	setLevelCmd(c, m, newArgs)
}

// unignoreUserCmd is a simple wrapper around setLevelCmd that sets a user to the Regular level
func unignoreUserCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		replyTarget := m.Params[0]
		if replyTarget == c.CurrentNick() {
			replyTarget = m.Prefix.Name
		}
		c.Writef("%s %s :Usage: !unignore <nick>", internal.CMD_PRIVMSG, replyTarget)
		return
	}
	
	// Create new args for setLevelCmd with the "regular" level
	newArgs := []string{args[0], "regular"}
	setLevelCmd(c, m, newArgs)
}

// rateLimitCmd allows admins to adjust rate limiting settings
func rateLimitCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	if len(args) < 1 {
		c.Writef("%s %s :Usage: !ratelimit <info|set> [parameters]", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :Examples:", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :!ratelimit info - Show current settings", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :!ratelimit set msg-window 10 - Set message window to 10 seconds", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :!ratelimit set msg-max 5 - Set max messages per window to 5", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :!ratelimit set cmd-window 30 - Set command window to 30 seconds", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :!ratelimit set cmd-max 10 - Set max commands per window to 10", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :!ratelimit set warning 3 - Set warnings before auto-ignore to 3", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :!ratelimit reset <nick> - Reset tracking for a specific user", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	hostmask := m.Prefix.String()
	if !userlevels.HasPermission(hostmask, userlevels.Admin) {
		c.Writef("%s %s :You need admin privileges to configure rate limiting.", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "info":
		// Get the current tracker configuration
		c.Writef("%s %s :Current rate limiting settings:", internal.CMD_PRIVMSG, replyTarget)
		c.Writef("%s %s :Message window: %v seconds", internal.CMD_PRIVMSG, replyTarget, 
			security.GlobalMessageTracker.GetMessageWindow().Seconds())
		c.Writef("%s %s :Max messages per window: %d", internal.CMD_PRIVMSG, replyTarget, 
			security.GlobalMessageTracker.GetMaxMessagesPerWindow())
		c.Writef("%s %s :Command window: %v seconds", internal.CMD_PRIVMSG, replyTarget, 
			security.GlobalMessageTracker.GetCommandWindow().Seconds())
		c.Writef("%s %s :Max commands per window: %d", internal.CMD_PRIVMSG, replyTarget, 
			security.GlobalMessageTracker.GetMaxCommandsPerWindow())
		c.Writef("%s %s :Warning threshold before auto-ignore: %d", internal.CMD_PRIVMSG, replyTarget, 
			security.GlobalMessageTracker.GetWarningThreshold())

	case "set":
		if len(args) < 3 {
			c.Writef("%s %s :Usage: !ratelimit set <parameter> <value>", internal.CMD_PRIVMSG, replyTarget)
			return
		}

		param := args[1]
		valueStr := args[2]
		value, err := strconv.Atoi(valueStr)
		if err != nil {
			c.Writef("%s %s :Invalid value: %s must be a number", internal.CMD_PRIVMSG, replyTarget, valueStr)
			return
		}

		switch param {
		case "msg-window":
			if value < 1 {
				c.Writef("%s %s :Value must be at least 1 second", internal.CMD_PRIVMSG, replyTarget)
				return
			}
			security.GlobalMessageTracker.SetMessageWindow(time.Duration(value) * time.Second)
			c.Writef("%s %s :Message window set to %d seconds", internal.CMD_PRIVMSG, replyTarget, value)

		case "msg-max":
			if value < 1 {
				c.Writef("%s %s :Value must be at least 1", internal.CMD_PRIVMSG, replyTarget)
				return
			}
			security.GlobalMessageTracker.SetMaxMessagesPerWindow(value)
			c.Writef("%s %s :Max messages per window set to %d", internal.CMD_PRIVMSG, replyTarget, value)

		case "cmd-window":
			if value < 1 {
				c.Writef("%s %s :Value must be at least 1 second", internal.CMD_PRIVMSG, replyTarget)
				return
			}
			security.GlobalMessageTracker.SetCommandWindow(time.Duration(value) * time.Second)
			c.Writef("%s %s :Command window set to %d seconds", internal.CMD_PRIVMSG, replyTarget, value)

		case "cmd-max":
			if value < 1 {
				c.Writef("%s %s :Value must be at least 1", internal.CMD_PRIVMSG, replyTarget)
				return
			}
			security.GlobalMessageTracker.SetMaxCommandsPerWindow(value)
			c.Writef("%s %s :Max commands per window set to %d", internal.CMD_PRIVMSG, replyTarget, value)

		case "warning":
			if value < 1 {
				c.Writef("%s %s :Value must be at least 1", internal.CMD_PRIVMSG, replyTarget)
				return
			}
			security.GlobalMessageTracker.SetWarningThreshold(value)
			c.Writef("%s %s :Warning threshold set to %d", internal.CMD_PRIVMSG, replyTarget, value)

		default:
			c.Writef("%s %s :Unknown parameter: %s", internal.CMD_PRIVMSG, replyTarget, param)
		}

	case "reset":
		if len(args) < 2 {
			c.Writef("%s %s :Usage: !ratelimit reset <nick>", internal.CMD_PRIVMSG, replyTarget)
			return
		}

		target := args[1]
		
		// Find hostmasks that match this nick
		allHostmasks := userlevels.GetAllHostmasks()
		resetCount := 0
		
		for hostmask := range allHostmasks {
			if strings.HasPrefix(hostmask, target+"!") {
				security.GlobalMessageTracker.ResetUser(hostmask)
				resetCount++
			}
		}

		if resetCount == 0 {
			c.Writef("%s %s :No hostmasks found for user %s", internal.CMD_PRIVMSG, replyTarget, target)
		} else {
			c.Writef("%s %s :Reset rate limiting data for %d hostmasks matching %s", 
				internal.CMD_PRIVMSG, replyTarget, resetCount, target)
		}

	default:
		c.Writef("%s %s :Unknown subcommand: %s", internal.CMD_PRIVMSG, replyTarget, subcommand)
	}
}