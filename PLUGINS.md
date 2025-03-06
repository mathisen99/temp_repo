# MBot Plugin Development Guide

This guide explains how to create plugins for MBot. The plugin system allows you to extend the bot's functionality without modifying the core codebase.

## Plugin System Architecture

MBot uses Go's plugin package to dynamically load shared objects (`.so` files) at runtime. Plugins can:

- Implement command handlers for specific commands
- React to IRC events (joins, parts, messages, etc.)
- Access the bot's API for common operations
- Store and retrieve persistent data
- Get updated without restarting the bot
- Utilize the AI integration capabilities
- Configure channel-specific settings

## Creating a Basic Plugin

### Plugin Structure

Every plugin must implement the basic `Plugin` interface:

```go
type Plugin interface {
    Name() string
    Version() string
    OnLoad() error
    OnMessage(*irc.Client, *irc.Message)
    OnUnload() error
}
```

### Starter Template

Create a new Go file in the `plugins_src` directory:

```go
//go:build my_plugin
// +build my_plugin

package main

import (
    "gopkg.in/irc.v4"
    "ircbot/pkg/api"
)

type MyPlugin struct{}

// Name returns the plugin name
func (p *MyPlugin) Name() string {
    return "MyPlugin"
}

// Version returns the plugin version
func (p *MyPlugin) Version() string {
    return "1.0.0"
}

// OnLoad is called when the plugin is loaded
func (p *MyPlugin) OnLoad() error {
    api.LogSuccess("MyPlugin version %s loaded!", p.Version())
    return nil
}

// OnMessage handles all IRC messages
func (p *MyPlugin) OnMessage(c *irc.Client, m *irc.Message) {
    // Process all IRC messages here
}

// OnUnload is called when the plugin is unloaded
func (p *MyPlugin) OnUnload() error {
    api.LogSuccess("MyPlugin version %s unloaded!", p.Version())
    return nil
}

// Export the plugin
var Plugin MyPlugin
```

### Building Your Plugin

After creating your plugin source file, build it using:

```bash
./build_plugins.sh
```

This script:
1. Extracts the plugin version from your code
2. Builds the plugin with the appropriate tags
3. Creates a versioned filename (e.g., `my_plugin_v1.0.0.so`)
4. Creates a symlink to the latest version (`my_plugin.so`)

Alternatively, build a single plugin manually:

```bash
go build -buildmode=plugin -tags my_plugin -o plugins/my_plugin_v1.0.0.so plugins_src/my_plugin.go
ln -sf my_plugin_v1.0.0.so plugins/my_plugin.so
```

## Event Handling

Plugins can implement various handler interfaces to respond to specific IRC events:

### Command Handler

Process specific commands with permission checking:

```go
// CommandHandler interface
func (p *MyPlugin) GetCommands() []string {
    return []string{"mycmd"}
}

func (p *MyPlugin) HandleCommand(c *irc.Client, m *irc.Message, cmd string, args []string) {
    if cmd == "mycmd" {
        target := api.GetReplyTarget(c, m)
        api.SendMessage(c, target, "Hello from MyPlugin!")
    }
}
```

### Event-Specific Handlers

Implement any of these interfaces to handle specific IRC events:

```go
// NickMentionHandler - When bot's nick is mentioned
func (p *MyPlugin) OnNickMention(c *irc.Client, m *irc.Message) {}

// JoinHandler - When users join channels
func (p *MyPlugin) OnJoin(c *irc.Client, m *irc.Message) {}

// PartHandler - When users leave channels
func (p *MyPlugin) OnPart(c *irc.Client, m *irc.Message) {}

// QuitHandler - When users quit IRC
func (p *MyPlugin) OnQuit(c *irc.Client, m *irc.Message) {}

// KickHandler - When users are kicked
func (p *MyPlugin) OnKick(c *irc.Client, m *irc.Message) {}

// TopicChangeHandler - When channel topics change
func (p *MyPlugin) OnTopicChange(c *irc.Client, m *irc.Message) {}

// NickHandler - When users change nicks
func (p *MyPlugin) OnNickChange(c *irc.Client, m *irc.Message) {}

// InviteHandler - When bot is invited to channels
func (p *MyPlugin) OnInvite(c *irc.Client, m *irc.Message) {}

// NoticeHandler - When bot receives notices
func (p *MyPlugin) OnNotice(c *irc.Client, m *irc.Message) {}

// ModeHandler - When channel/user modes change
func (p *MyPlugin) OnMode(c *irc.Client, m *irc.Message) {}

// ErrorHandler - When IRC error occurs
func (p *MyPlugin) OnError(c *irc.Client, m *irc.Message) {}
```

## MBot Plugin API

The API provides functions for common IRC operations, formatting, user management, and more.

### Message Handling

```go
// Send a message to a channel or user
api.SendMessage(client, "#channel", "Hello, world!")

// Send a notice to a channel or user
api.SendNotice(client, user, "This is a notice")

// Get the appropriate reply target (channel or user)
target := api.GetReplyTarget(client, message)

// Reply to a message context-appropriately
api.ReplyToMessage(client, message, "This is a reply")

// Split long messages for IRC (to avoid truncation)
chunks := api.SplitMessageForIRC(longMessage, 400)
```

### User Permissions

```go
// User permission levels
const (
    Ignored = 0 // Completely ignored user
    BadBoy  = 1 // Restricted user
    Regular = 2 // Normal user
    Admin   = 3 // Administrator
    Owner   = 4 // Bot owner
)

// Check if user has a specific permission level
isAllowed := api.CheckUserPermission(hostmask, api.Admin)

// Check if user is an admin
isAdmin := api.IsUserAdmin(hostmask)

// Check if user is the owner
isOwner := api.IsUserOwner(hostmask)

// Check if user is ignored
isIgnored := api.IsUserIgnored(hostmask)
```

### Text Formatting

```go
// Add colors to IRC messages
api.ColorText("Error!", api.ColorRed)
api.ColorText("Success!", api.ColorGreen)

// Format text styles
api.BoldText("Important!")
api.ItalicText("Emphasized")
api.UnderlineText("Underlined")
```

### Channel Operations

```go
// Join a channel
api.JoinChannel(client, "#channel")

// Leave a channel
api.PartChannel(client, "#channel", "Goodbye!")

// Kick a user
api.KickUser(client, "#channel", "user", "Reason")

// Set mode
api.SetMode(client, "#channel", "+o user")

// Change channel topic
api.ModifyChannelTopic(client, "#channel", "New topic")
```

### Data Storage

Each plugin can have its own data directory for persistent storage:

```go
// Get plugin data directory
dataDir := api.GetPluginDataDir(pluginName)

// Ensure data directory exists
api.EnsurePluginDataDir(pluginName)

// Save data to file
data := []byte(`{"setting": "value"}`)
api.SavePluginData(pluginName, "config.json", data)

// Load data from file
data, err := api.LoadPluginData(pluginName, "config.json")
```

### Channel-Specific Settings

Plugins can access and use channel-specific settings:

```go
// Check if a command is enabled for a channel
isEnabled := api.IsCommandEnabledForChannel("#channel", "mycommand")

// Get a channel-specific setting with default fallback value
greeting := api.GetChannelSetting("#channel", "greeting", "Hello!")

// Access custom settings for a plugin
maxWarnings, ok := api.GetChannelSetting("#channel", "max_warnings", 3).(int)
if ok {
    // Use the setting
}
```

### User Notes API

Access and manage user notes from plugins:

```go
// Add a note for a user
noteID, err := api.AddUserNote(userHostmask, "Important information")

// Get all notes for a user
notes, err := api.GetUserNotes(userHostmask)

// Search user notes
matches, err := api.SearchUserNotes(userHostmask, "search term")

// Delete a user note
err := api.DeleteUserNote(userHostmask, noteID)
```

### Spam Protection API

Access the rate limiting system:

```go
// Check if a user is exceeding message rate limits
isSpamming, count := api.IsUserSpamming(userHostmask)

// Get warning count for a user
warningCount := api.GetUserWarningCount(userHostmask)

// Reset tracking for a user
api.ResetUserTracking(userHostmask)
```

### Utility Functions

```go
// Parse IRC hostmasks (nick!user@host)
nick, user, host := api.ParseHostmask(hostmask)

// Parse command arguments with quote handling
args := api.ParseCommandArgs("arg1 \"arg with spaces\" arg3")

// Check if a string is a valid channel name
isChannel := api.IsChannelName("#channel")
```

### AI Integration

If AI functionality is enabled in the bot:

```go
// Check if AI is available
if api.IsAIAvailable() {
    // Process a message with AI
    response, err := api.ProcessWithAI("What's the weather like?")
    
    // Summarize text with AI
    summary, err := api.SummarizeWithAI(longText, 100)
    
    // Generate an image with AI
    imageUrl, err := api.GenerateImageWithAI("a beautiful sunset over mountains")
}
```

### Logging

```go
// Log at different levels
api.LogInfo("Information message")
api.LogSuccess("Success message")
api.LogWarn("Warning message")
api.LogError("Error message")
api.LogDebug("Debug message")
```

## Plugin Versioning & Updates

MBot supports updating plugins without restarting:

1. Update your plugin code
2. Change the version in the `Version()` method
3. Build the plugin with `./build_plugins.sh`
4. Use `!load <plugin>` to reload the plugin

The plugin system will:
1. Detect the version change
2. Unload the old version
3. Load the new version
4. Preserve plugin state

## Examples

### Echo Plugin

```go
//go:build echo_plugin
// +build echo_plugin

package main

import (
    "strings"
    "gopkg.in/irc.v4"
    "ircbot/pkg/api"
)

type EchoPlugin struct{}

func (p *EchoPlugin) Name() string {
    return "EchoPlugin"
}

func (p *EchoPlugin) Version() string {
    return "1.0.0"
}

func (p *EchoPlugin) GetCommands() []string {
    return []string{"echo"}
}

func (p *EchoPlugin) HandleCommand(c *irc.Client, m *irc.Message, cmd string, args []string) {
    if cmd == "echo" {
        target := api.GetReplyTarget(c, m)
        message := strings.Join(args, " ")
        api.SendMessage(c, target, message)
    }
}

func (p *EchoPlugin) OnLoad() error {
    api.LogSuccess("EchoPlugin loaded!")
    return nil
}

func (p *EchoPlugin) OnMessage(c *irc.Client, m *irc.Message) {
    // Not needed for this plugin as we use HandleCommand
}

func (p *EchoPlugin) OnUnload() error {
    api.LogSuccess("EchoPlugin unloaded!")
    return nil
}

var Plugin EchoPlugin
```

### Join Handler Plugin

```go
//go:build join_plugin
// +build join_plugin

package main

import (
    "gopkg.in/irc.v4"
    "ircbot/pkg/api"
)

type JoinPlugin struct{}

func (p *JoinPlugin) Name() string {
    return "JoinPlugin"
}

func (p *JoinPlugin) Version() string {
    return "1.0.0"
}

func (p *JoinPlugin) OnLoad() error {
    api.LogSuccess("JoinPlugin loaded!")
    return nil
}

func (p *JoinPlugin) OnMessage(c *irc.Client, m *irc.Message) {
    // Not needed for this plugin
}

func (p *JoinPlugin) OnJoin(c *irc.Client, m *irc.Message) {
    // Don't respond to our own joins
    if m.Prefix.Name == c.CurrentNick() {
        return
    }
    
    channel := m.Params[0]
    api.SendMessage(c, channel, "Welcome to "+channel+", "+m.Prefix.Name+"!")
}

func (p *JoinPlugin) OnUnload() error {
    api.LogSuccess("JoinPlugin unloaded!")
    return nil
}

var Plugin JoinPlugin
```

### Channel-Specific Settings Example

```go
//go:build greeter_plugin
// +build greeter_plugin

package main

import (
    "gopkg.in/irc.v4"
    "ircbot/pkg/api"
)

type GreeterPlugin struct{}

func (p *GreeterPlugin) Name() string {
    return "GreeterPlugin"
}

func (p *GreeterPlugin) Version() string {
    return "1.0.0"
}

func (p *GreeterPlugin) OnLoad() error {
    api.LogSuccess("GreeterPlugin loaded!")
    return nil
}

func (p *GreeterPlugin) OnMessage(c *irc.Client, m *irc.Message) {
    // Not needed for this plugin
}

func (p *GreeterPlugin) OnJoin(c *irc.Client, m *irc.Message) {
    // Don't respond to our own joins
    if m.Prefix.Name == c.CurrentNick() {
        return
    }
    
    channel := m.Params[0]
    
    // Check if greeting is enabled for this channel
    enabled := api.GetChannelSetting(channel, "greeting_enabled", true)
    if enabled, ok := enabled.(bool); !ok || !enabled {
        // Greetings are disabled for this channel
        return
    }
    
    // Get channel-specific greeting message
    defaultGreeting := "Welcome to " + channel + ", " + m.Prefix.Name + "!"
    greeting := api.GetChannelSetting(channel, "greeting_message", defaultGreeting)
    
    // Send the greeting
    if greetingMsg, ok := greeting.(string); ok {
        api.SendMessage(c, channel, greetingMsg)
    } else {
        api.SendMessage(c, channel, defaultGreeting)
    }
}

func (p *GreeterPlugin) OnUnload() error {
    api.LogSuccess("GreeterPlugin unloaded!")
    return nil
}

var Plugin GreeterPlugin
```

### AI Assistant Plugin Example

```go
//go:build ai_assistant_plugin
// +build ai_assistant_plugin

package main

import (
    "strings"
    "gopkg.in/irc.v4"
    "ircbot/pkg/api"
)

type AIAssistantPlugin struct{}

func (p *AIAssistantPlugin) Name() string {
    return "AIAssistantPlugin"
}

func (p *AIAssistantPlugin) Version() string {
    return "1.0.0"
}

func (p *AIAssistantPlugin) GetCommands() []string {
    return []string{"ask"}
}

func (p *AIAssistantPlugin) HandleCommand(c *irc.Client, m *irc.Message, cmd string, args []string) {
    if cmd == "ask" {
        target := api.GetReplyTarget(c, m)
        
        if len(args) == 0 {
            api.SendMessage(c, target, "Usage: !ask <question>")
            return
        }
        
        question := strings.Join(args, " ")
        
        // Check if AI is available
        if !api.IsAIAvailable() {
            api.SendMessage(c, target, "AI functionality is not available.")
            return
        }
        
        // Get user's notes to provide context
        userHostmask := m.Prefix.String()
        notes, _ := api.GetUserNotes(userHostmask)
        
        // Process with AI including notes as context
        response, err := api.ProcessWithAI(question, notes)
        if err != nil {
            api.SendMessage(c, target, "Error processing request: "+err.Error())
            return
        }
        
        // Split response into IRC-friendly chunks
        chunks := api.SplitMessageForIRC(response, 400)
        for _, chunk := range chunks {
            api.SendMessage(c, target, chunk)
        }
    }
}

func (p *AIAssistantPlugin) OnLoad() error {
    api.LogSuccess("AIAssistantPlugin loaded!")
    return nil
}

func (p *AIAssistantPlugin) OnMessage(c *irc.Client, m *irc.Message) {
    // Not needed for this plugin as we use HandleCommand
}

func (p *AIAssistantPlugin) OnUnload() error {
    api.LogSuccess("AIAssistantPlugin unloaded!")
    return nil
}

var Plugin AIAssistantPlugin
```

## Best Practices

1. **Version Your Plugins**: Always update the version when making changes.
2. **Handle Errors**: Gracefully handle errors in your plugin.
3. **Clean Up Resources**: Free resources in the `OnUnload()` method.
4. **Check Permissions**: Verify user permissions before executing privileged commands.
5. **Be Responsive**: Keep message handlers fast to avoid blocking the bot.
6. **Document Commands**: Make your commands discoverable via the help system.
7. **Persist Configuration**: Use the data storage API for persistent settings.
8. **Use the API**: Use the provided API functions instead of direct IRC commands.
9. **Respect Channel Settings**: Check channel-specific settings before taking actions.
10. **Use Default Fallbacks**: Always provide sensible defaults when getting channel settings.
11. **Rate Limit Awareness**: Respect and utilize the spam protection system.
12. **Note Integration**: If using AI, consider utilizing the note system for context.

Happy plugin development!