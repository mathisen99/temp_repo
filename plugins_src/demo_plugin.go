//go:build demo_plugin
// +build demo_plugin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

// DemoPlugin demonstrates the capabilities of the MBot Plugin API
type DemoPlugin struct {
	commandPrefix string
	lastUsage     map[string]time.Time
	userData      map[string]UserData
}

// UserData stores information about users
type UserData struct {
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	MessageCount int       `json:"message_count"`
	Nickname     string    `json:"nickname"`
}

// Name returns the plugin name
func (p *DemoPlugin) Name() string {
	return "DemoPlugin"
}

// Version returns the plugin version
func (p *DemoPlugin) Version() string {
	return "1.0.0"
}

// OnLoad initializes the plugin
func (p *DemoPlugin) OnLoad() error {
	api.LogSuccess("DemoPlugin version %s loaded successfully!", p.Version())

	// Initialize plugin data
	p.commandPrefix = "!demo"
	p.lastUsage = make(map[string]time.Time)
	p.userData = make(map[string]UserData)

	// Load saved user data if it exists
	dataDir, err := api.EnsurePluginDataDir(p.Name())
	if err != nil {
		api.LogWarn("Could not create data directory: %v", err)
	}

	filePath := fmt.Sprintf("%s/users.json", dataDir)
	data, err := os.ReadFile(filePath)
	if err == nil {
		if err := json.Unmarshal(data, &p.userData); err != nil {
			api.LogWarn("Error loading user data: %v", err)
		} else {
			api.LogInfo("Loaded data for %d users", len(p.userData))
		}
	}

	// Print bot configuration info
	botCfg, err := api.GetBotConfig()
	if err != nil {
		api.LogError("Could not load bot config: %v", err)
	} else {
		api.LogInfo("Bot is configured to connect to %s with nick %s",
			botCfg.ServerAddress, botCfg.BotNick)
	}

	return nil
}

// OnMessage handles incoming IRC messages
func (p *DemoPlugin) OnMessage(c *irc.Client, m *irc.Message) {
	// Process only PRIVMSG commands
	if m.Command != api.CMD_PRIVMSG {
		return
	}

	// Track user activity - this is the only part we keep in OnMessage
	p.trackUserActivity(m)

	// We're not handling commands here anymore
	// Command handling is now done by HandleCommand method
	// This prevents duplicate responses to commands
}

// handleHelp shows the help message
func (p *DemoPlugin) handleHelp(c *irc.Client, m *irc.Message, args []string) {
	target := api.GetReplyTarget(c, m)

	helpText := []string{
		api.BoldText("DemoPlugin Commands:"),
		api.FormatCommandHelp("help", "!demo help", "Shows this help message"),
		api.FormatCommandHelp("info", "!demo info [user]", "Shows information about a user or the bot"),
		api.FormatCommandHelp("color", "!demo color <color>", "Demonstrates IRC color codes"),
		api.FormatCommandHelp("format", "!demo format", "Shows IRC text formatting examples"),
		api.FormatCommandHelp("stats", "!demo stats", "Shows plugin statistics"),
		api.FormatCommandHelp("ai", "!demo ai <message>", "Processes text with AI (if available)"),
		api.FormatCommandHelp("admin", "!demo admin", "Admin-only command"),
	}

	for _, line := range helpText {
		api.SendMessage(c, target, line)
	}
}

// handleInfo shows information about a user or the bot
func (p *DemoPlugin) handleInfo(c *irc.Client, m *irc.Message, args []string) {
	target := api.GetReplyTarget(c, m)

	var nickname string
	if len(args) > 0 {
		nickname = args[0]
	} else {
		nickname = m.Prefix.Name
	}

	// Get user info
	userData, found := p.userData[strings.ToLower(nickname)]
	if found {
		firstSeen := userData.FirstSeen.Format("2006-01-02 15:04:05")
		lastSeen := userData.LastSeen.Format("2006-01-02 15:04:05")

		api.SendMessage(c, target, fmt.Sprintf("User %s: First seen: %s, Last seen: %s, Messages: %d",
			api.BoldText(nickname), firstSeen, lastSeen, userData.MessageCount))
	} else {
		api.SendMessage(c, target, fmt.Sprintf("No data for user %s", api.BoldText(nickname)))
	}

	// Show hostmask info if it's the current user
	if nickname == m.Prefix.Name {
		hostmask := m.Prefix.String()
		nick, user, host := api.ParseHostmask(hostmask)

		api.SendMessage(c, target, fmt.Sprintf("Your hostmask: %s (Nick: %s, User: %s, Host: %s)",
			hostmask, nick, user, host))

		// Show permission level
		level := api.GetUserLevelByHostmask(hostmask)
		levelName := api.GetUserLevelName(level)

		api.SendMessage(c, target, fmt.Sprintf("Your permission level: %s", api.BoldText(levelName)))
	}
}

// handleColor demonstrates IRC color codes
func (p *DemoPlugin) handleColor(c *irc.Client, m *irc.Message, args []string) {
	target := api.GetReplyTarget(c, m)

	if len(args) == 0 {
		// Show available colors
		colorNames := []string{
			"white", "black", "blue", "green",
			"red", "brown", "purple", "orange",
			"yellow", "lightgreen", "cyan", "lightcyan",
			"lightblue", "pink", "gray", "lightgray",
		}
		api.SendMessage(c, target, fmt.Sprintf("Available colors: %s", strings.Join(colorNames, ", ")))
		return
	}

	// Show example of requested color
	color := args[0]
	switch strings.ToLower(color) {
	case "white":
		api.SendMessage(c, target, api.ColorText("This text is white", api.ColorWhite))
	case "black":
		api.SendMessage(c, target, api.ColorText("This text is black", api.ColorBlack))
	case "blue":
		api.SendMessage(c, target, api.ColorText("This text is blue", api.ColorBlue))
	case "green":
		api.SendMessage(c, target, api.ColorText("This text is green", api.ColorGreen))
	case "red":
		api.SendMessage(c, target, api.ColorText("This text is red", api.ColorRed))
	case "brown":
		api.SendMessage(c, target, api.ColorText("This text is brown", api.ColorBrown))
	case "purple":
		api.SendMessage(c, target, api.ColorText("This text is purple", api.ColorPurple))
	case "orange":
		api.SendMessage(c, target, api.ColorText("This text is orange", api.ColorOrange))
	case "yellow":
		api.SendMessage(c, target, api.ColorText("This text is yellow", api.ColorYellow))
	case "lightgreen":
		api.SendMessage(c, target, api.ColorText("This text is light green", api.ColorLightGreen))
	case "cyan":
		api.SendMessage(c, target, api.ColorText("This text is cyan", api.ColorCyan))
	case "lightcyan":
		api.SendMessage(c, target, api.ColorText("This text is light cyan", api.ColorLightCyan))
	case "lightblue":
		api.SendMessage(c, target, api.ColorText("This text is light blue", api.ColorLightBlue))
	case "pink":
		api.SendMessage(c, target, api.ColorText("This text is pink", api.ColorPink))
	case "gray":
		api.SendMessage(c, target, api.ColorText("This text is gray", api.ColorGray))
	case "lightgray":
		api.SendMessage(c, target, api.ColorText("This text is light gray", api.ColorLightGray))
	default:
		api.SendMessage(c, target, "Unknown color. Try: white, black, blue, green, red, etc.")
	}
}

// handleFormat demonstrates IRC text formatting
func (p *DemoPlugin) handleFormat(c *irc.Client, m *irc.Message, args []string) {
	target := api.GetReplyTarget(c, m)

	formats := []string{
		fmt.Sprintf("Bold: %s", api.BoldText("This text is bold")),
		fmt.Sprintf("Italic: %s", api.ItalicText("This text is italic")),
		fmt.Sprintf("Underline: %s", api.UnderlineText("This text is underlined")),
		fmt.Sprintf("Combined: %s", api.BoldText(api.ColorText("Bold and red", api.ColorRed))),
	}

	for _, line := range formats {
		api.SendMessage(c, target, line)
	}
}

// handleStats shows plugin statistics
func (p *DemoPlugin) handleStats(c *irc.Client, m *irc.Message, args []string) {
	target := api.GetReplyTarget(c, m)

	// Calculate statistics
	totalUsers := len(p.userData)
	totalMessages := 0
	for _, user := range p.userData {
		totalMessages += user.MessageCount
	}

	// Get some bot info
	botCfg, _ := api.GetBotConfig()
	serverName, _ := api.GetServerName()
	isFirstRun := api.IsFirstRun()

	stats := []string{
		api.BoldText("Plugin Statistics:"),
		fmt.Sprintf("- Tracked users: %d", totalUsers),
		fmt.Sprintf("- Total messages: %d", totalMessages),
		fmt.Sprintf("- Bot nick: %s", botCfg.BotNick),
		fmt.Sprintf("- Server: %s", serverName),
		fmt.Sprintf("- First run: %t", isFirstRun),
		fmt.Sprintf("- AI available: %t", api.IsAIAvailable()),
	}

	for _, line := range stats {
		api.SendMessage(c, target, line)
	}
}

// handleAI demonstrates AI integration
func (p *DemoPlugin) handleAI(c *irc.Client, m *irc.Message, args []string) {
	target := api.GetReplyTarget(c, m)
	user := m.Prefix.Name

	if !api.IsAIAvailable() {
		api.SendMessage(c, target, "AI is not available. Check if API key is configured.")
		return
	}

	if len(args) == 0 {
		api.SendMessage(c, target, "Usage: !demo ai <message>")
		return
	}

	// Process message with AI
	message := strings.Join(args, " ")
	response, err := api.ProcessWithAI(message, user)
	if err != nil {
		api.SendMessage(c, target, fmt.Sprintf("AI error: %v", err))
		return
	}

	// Split response into multiple messages if needed
	messages := api.SplitMessageForIRC(response, 400)
	for _, msg := range messages {
		api.SendMessage(c, target, msg)
	}
}

// handleAdmin shows admin-only commands
func (p *DemoPlugin) handleAdmin(c *irc.Client, m *irc.Message, args []string) {
	target := api.GetReplyTarget(c, m)
	hostmask := m.Prefix.String()

	// Check if user is an admin
	if !api.IsUserAdmin(hostmask) {
		api.SendMessage(c, target, "You need administrator privileges to use this command.")
		return
	}

	api.SendMessage(c, target, "Admin commands:")
	api.SendMessage(c, target, "- Save: Saves user data")
	api.SendMessage(c, target, "- Clear: Clears user data")

	if len(args) == 0 {
		return
	}

	switch strings.ToLower(args[0]) {
	case "save":
		p.saveUserData()
		api.SendMessage(c, target, "User data saved successfully.")
	case "clear":
		if api.IsUserOwner(hostmask) {
			p.userData = make(map[string]UserData)
			api.SendMessage(c, target, "User data cleared (owner only command).")
		} else {
			api.SendMessage(c, target, "Only the bot owner can clear all user data.")
		}
	}
}

// trackUserActivity updates user data based on activity
func (p *DemoPlugin) trackUserActivity(m *irc.Message) {
	nickname := m.Prefix.Name
	lowerNick := strings.ToLower(nickname)

	now := time.Now()

	userData, exists := p.userData[lowerNick]
	if !exists {
		userData = UserData{
			FirstSeen:    now,
			LastSeen:     now,
			MessageCount: 1,
			Nickname:     nickname,
		}
	} else {
		userData.LastSeen = now
		userData.MessageCount++
	}

	p.userData[lowerNick] = userData
}

// checkRateLimit checks if a command can be used based on rate limiting
func (p *DemoPlugin) checkRateLimit(hostmask, command string) bool {
	key := hostmask + ":" + command
	now := time.Now()

	// Check if the command was used recently
	lastTime, exists := p.lastUsage[key]
	if exists {
		// Rate limit: 5 seconds between command uses
		if now.Sub(lastTime) < 5*time.Second {
			return false
		}
	}

	// Update last usage time
	p.lastUsage[key] = now
	return true
}

// saveUserData saves user data to a JSON file
func (p *DemoPlugin) saveUserData() {
	dataDir, err := api.EnsurePluginDataDir(p.Name())
	if err != nil {
		api.LogError("Could not create data directory: %v", err)
		return
	}

	// Convert to JSON
	data, err := json.MarshalIndent(p.userData, "", "  ")
	if err != nil {
		api.LogError("Could not marshal user data: %v", err)
		return
	}

	// Save to file
	filePath := fmt.Sprintf("%s/users.json", dataDir)
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		api.LogError("Could not save user data: %v", err)
	} else {
		api.LogSuccess("Saved data for %d users", len(p.userData))
	}
}

// OnUnload is called when the plugin is unloaded
func (p *DemoPlugin) OnUnload() error {
	// Save user data when plugin is unloaded
	p.saveUserData()

	api.LogSuccess("DemoPlugin version %s unloaded!", p.Version())
	return nil
}

// GetCommands returns the supported commands for this plugin
func (p *DemoPlugin) GetCommands() []string {
	return []string{"demo"}
}

// HandleCommand processes commands for this plugin
func (p *DemoPlugin) HandleCommand(c *irc.Client, m *irc.Message, cmd string, args []string) {
	if cmd != "demo" {
		return
	}

	// Handle subcommands directly
	if len(args) > 0 {
		subcmd := args[0]
		subargs := args[1:]

		switch strings.ToLower(subcmd) {
		case "help":
			p.handleHelp(c, m, subargs)
		case "info":
			p.handleInfo(c, m, subargs)
		case "color":
			p.handleColor(c, m, subargs)
		case "format":
			p.handleFormat(c, m, subargs)
		case "stats":
			p.handleStats(c, m, subargs)
		case "ai":
			p.handleAI(c, m, subargs)
		case "admin":
			p.handleAdmin(c, m, subargs)
		default:
			target := api.GetReplyTarget(c, m)
			api.SendMessage(c, target, fmt.Sprintf("Unknown command: %s. Try %s help",
				api.BoldText(subcmd), p.commandPrefix))
		}
	} else {
		// Show help if no subcommand
		p.handleHelp(c, m, nil)
	}
}

// Plugin export
var Plugin DemoPlugin
