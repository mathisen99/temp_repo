# MBot - A Modular IRC Bot

MBot is a powerful, extensible IRC bot written in Go. It features a plugin-based architecture, comprehensive permission system, AI integration capabilities, and advanced spam protection.

## Key Features

- **Plugin System**: Dynamic loading/unloading of plugins without restarting
- **Permission Levels**: Five-tier permission system (Owner, Admin, Regular, BadBoy, Ignored)
- **Command Framework**: Extensive command system with permission controls
- **Event Handling**: Comprehensive handlers for all IRC events
- **AI Integration**: Natural language interactions with advanced tooling
- **Anti-Spam Protection**: Configurable rate limiting with progressive warnings
- **Note System**: Personal notes that can be queried and used by AI
- **Hot Reloading**: Update plugins on-the-fly with version tracking
- **Security**: Hostmask-based authentication and verification
- **Channel-Specific Settings**: Customize bot behavior per channel

## Getting Started

### Prerequisites

- Go 1.19 or newer
- A working Go environment

### Installation

1. Clone the repository
   ```
   git clone <repository-url>
   cd Mbot
   ```

2. Build the bot
   ```
   go build -o mbot ./cmd/ircbot
   ```

3. Build included plugins
   ```
   ./build_plugins.sh
   ```

### Configuration

Create a configuration file at `./data/config.toml`:

```toml
# IRC server address (hostname:port)
server = "irc.example.com:6667"

# Bot identity
nick = "MBot"
user = "mbot"
real_name = "MBot IRC Bot"

# Optional NickServ password
password = ""

# Channels to join on startup
channels = ["#channel1", "#channel2"]
```

## Running the Bot

Start the bot:
```
./mbot
```

If it's your first time running the bot, you'll be prompted to set up an owner password. The owner has full control over the bot.

## Command System

MBot responds to commands that start with a `!` prefix. Available commands depend on your permission level:

### For All Users
- `!help` - Show available commands
- `!test` - Test if the bot is responding
- `!ai <question>` - Ask the AI assistant (if configured)
- `!plugins` - List available plugins
- `!say <text>` - Make the bot say something
- `!action <text>` - Make the bot perform an action
- `!note` - Manage personal notes (add/list/search/delete)

### For Administrators
- `!reload` - Reload all plugins
- `!load <plugin>` - Load a specific plugin
- `!unload <plugin>` - Unload a specific plugin
- `!load-online <url>` - Load a plugin from a URL
- `!join <channel>` - Join a channel
- `!part <channel>` - Leave a channel
- `!kick <user> [reason]` - Kick a user
- `!ban <user>` - Ban a user
- `!ratelimit` - Configure spam protection settings
- `!ignore <user>` - Ignore a user completely
- `!unignore <user>` - Stop ignoring a user
- Channel management: `!op`, `!deop`, `!voice`, `!devoice`
- `!channel` - Manage channel-specific settings (see below)
- `!personality` - Set channel-specific AI personality

### For the Owner
- `!die` - Shut down the bot
- `!restart` - Restart the bot
- `!nick <new_nick>` - Change the bot's nickname
- `!setlevel <user> <level>` - Set a user's permission level

## Anti-Spam Protection

MBot includes a comprehensive spam protection system:

- **Rate Limiting**: Automatically limits message and command rates
- **Progressive Warnings**: Users receive warnings before being ignored
- **Auto-Ignore**: Persistent spammers are automatically set to Ignored level
- **Admin Controls**: Configure all settings with the `!ratelimit` command

### Rate Limit Command Usage

- `!ratelimit info` - Show current rate limiting settings
- `!ratelimit set msg-window <seconds>` - Set message tracking window
- `!ratelimit set msg-max <count>` - Set max messages per window
- `!ratelimit set cmd-window <seconds>` - Set command tracking window
- `!ratelimit set cmd-max <count>` - Set max commands per window
- `!ratelimit set warning <count>` - Set warnings before auto-ignore
- `!ratelimit reset <nick>` - Reset tracking for a specific user

## Note System

MBot allows users to create and manage personal notes that can be used in AI interactions:

- `!note add <text>` - Add a personal note
- `!note list` - List all your notes
- `!note search <query>` - Search your notes
- `!note delete <id>` - Delete a note by ID
- `!note show <id>` - Show a specific note

Notes are included when interacting with the AI assistant, allowing for personalized responses that consider your stored information.

## Channel-Specific Settings

MBot supports channel-specific settings, allowing you to customize bot behavior per channel. These settings are managed with the `!channel` command.

### Channel Command Usage

- `!channel list` - List all channels with custom settings
- `!channel info <channel>` - View settings for a specific channel
- `!channel enable <channel> <command>` - Enable a command in a channel
- `!channel disable <channel> <command>` - Disable a command in a channel
- `!channel set <channel> <key> <value>` - Set a custom setting
- `!channel save` - Save all channel settings

### Examples

#### Disable specific commands in a channel
```
!channel disable #general help
!channel disable #support test
```

#### Enable only specific commands in a channel
First disable all commands by setting an empty enabled list, then enable only what you want:
```
!channel set #announce enabled_commands []
!channel enable #announce say
```

#### Disable nickname mention responses
Prevent the bot from responding when mentioned in a channel:
```
!channel set #busy use_ai_for_mentions false
```

#### Disable URL logging
Turn off URL logging for channels where it's not needed:
```
!channel set #links log_urls false
```

#### Set custom channel-specific settings
These can be used by plugins or custom commands:
```
!channel set #welcome greeting "Welcome to our community channel!"
!channel set #support auto_help true
!channel set #moderation max_warnings 3
```

### Viewing Channel Settings
To check what settings are enabled for a channel:
```
!channel info #mychannel
```

## Built-in Plugins

MBot includes several plugins:
- **EchoPlugin** - Echoes text (`!echo <text>`)
- **JoinPlugin** - Sends welcome messages when users join
- **PartPlugin** - Responds when users leave
- **KickPlugin** - Responds to kick events
- **NickPlugin** - Responds when the bot is mentioned
- **TopicPlugin** - Responds to topic changes
- **InvitePlugin** - Handles channel invitations
- **NoticePlugin** - Processes IRC notices
- **QuitPlugin** - Responds when users quit
- **AIAssistantPlugin** - Advanced AI assistant functionality

## AI Features and Configuration

MBot has extensive AI integration for natural language interactions. The following features are available when properly configured:

### Core AI Capabilities

- **Chat Interactions**: Respond to questions in natural language
- **Channel Context**: Include recent channel history for relevant responses
- **Personality Settings**: Configure different AI personalities per channel
- **User Notes**: Include personal notes to provide context for responses

### AI Tools

MBot's AI comes with several advanced tools:

1. **Google Search**: Search the web for current information
2. **Website Content Analysis**: Extract and analyze website content
3. **Channel Log Search**: Search through IRC chat logs
4. **Image Generation**: Create images via AI
5. **Error Log Search**: Search bot error logs
6. **Note Management**: Access user notes
7. **Paste Service**: Share code snippets and long text
8. **Code Runner**: Execute code snippets (with safety limitations)

### Search Web Tool Configuration

To enable the Google search feature, set the following environment variables:

```bash
# Required for the Google Search tool
GOOGLE_API_KEY=your_google_api_key
GOOGLE_SEARCH_ENGINE_ID=your_search_engine_cx_id
```

You can obtain these from:
1. Google API Key: [Google Cloud Console](https://console.cloud.google.com/)
2. Search Engine ID: [Google Programmable Search Engine](https://programmablesearchengine.google.com/)

When configured, the AI assistant can search the web autonomously to answer questions about current events or topics it doesn't have information about.

## For Developers

For information on developing plugins and using the MBot API, please see [PLUGINS.md](PLUGINS.md).

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [irc.v4](https://github.com/go-irc/irc) - IRC client library
- [godotenv](https://github.com/joho/godotenv) - .env file loader
- [toml](https://github.com/BurntSushi/toml) - TOML parser
- [fatih/color](https://github.com/fatih/color) - Terminal color output# temp_repo
