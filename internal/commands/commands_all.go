package commands

import (
	"ircbot/internal/userlevels"
)

func init() {
	// Channel Operator Commands - Admin level
	RegisterCommand("op", "Give channel operator status to a user", userlevels.Admin, opCmd)
	RegisterCommand("deop", "Remove channel operator status from a user", userlevels.Admin, deopCmd)
	RegisterCommand("voice", "Give voice status to a user in a channel", userlevels.Admin, voiceCmd)
	RegisterCommand("devoice", "Remove voice status from a user in a channel", userlevels.Admin, devoiceCmd)
	RegisterCommand("kick", "Kick a user from the channel", userlevels.Admin, kickCmd)
	RegisterCommand("ban", "Ban a user or hostmask from the channel", userlevels.Admin, banCmd)
	RegisterCommand("unban", "Remove a ban on a user or hostmask", userlevels.Admin, unbanCmd)
	RegisterCommand("mute", "Mute a user or hostmask in the channel", userlevels.Admin, muteCmd)
	RegisterCommand("unmute", "Unmute a user or hostmask in the channel", userlevels.Admin, unmuteCmd)

	// Channel Commands - Admin level
	RegisterCommand("invite", "Invite a user to a channel", userlevels.Admin, inviteCmd)
	RegisterCommand("topic", "View or change the channel topic", userlevels.Admin, topicCmd)
	RegisterCommand("join", "Make the bot join a channel", userlevels.Admin, joinCmd)
	RegisterCommand("part", "Make the bot leave a channel", userlevels.Admin, partCmd)

	// Bot Control Commands - Owner level
	RegisterCommand("nick", "Change the bot's nickname", userlevels.Owner, nickCmd)

	// Message Commands - Regular level and Admin level
	RegisterCommand("say", "Make the bot say something in the current channel", userlevels.Regular, sayCmd)
	RegisterCommand("msg", "Send a private message to a user or channel", userlevels.Admin, msgCmd)
	RegisterCommand("notice", "Send a notice to a user or channel", userlevels.Admin, noticeCmd)
	RegisterCommand("action", "Make the bot perform an action (/me) in the channel", userlevels.Regular, actionCmd)

	// Information Commands - Regular level
	RegisterCommand("whois", "Get information about a user", userlevels.Regular, whoisCmd)
	RegisterCommand("mode", "View or set modes for a user or channel", userlevels.Admin, modeCmd)

	// Regular user group commands
	RegisterCommand("help", "Show available commands", userlevels.Regular, helpCmd)
	RegisterCommand("test", "Test command", userlevels.Regular, testCommand)
	RegisterCommand("ai", "Ask a question to the AI assistant", userlevels.Regular, aiCmd)
	RegisterCommand("personality", "Set a channel-specific personality for the AI", userlevels.Admin, personalityCmd)
	RegisterCommand("note", "Manage personal notes for AI interactions", userlevels.Regular, noteCommand)

	// Admin user group commands
	RegisterCommand("reload", "Reload plugins", userlevels.Admin, reloadPluginsCmd)
	RegisterCommand("plugins", "List all available plugins", userlevels.Regular, listPlugins)
	RegisterCommand("load", "Load a plugin. Usage: !load <pluginName>", userlevels.Admin, loadPluginCmd)
	RegisterCommand("unload", "Unload a plugin. Usage: !unload <pluginName>", userlevels.Admin, unloadPluginCmd)
	RegisterCommand("load-online", "Download and load a plugin from URL. Usage: !load-online <URL>", userlevels.Admin, loadOnlinePluginCmd)
	RegisterCommand("channel", "Manage channel-specific settings", userlevels.Admin, channelCmd)

	// Owner user group commands
	RegisterCommand("setlevel", "Set a user's level. Usage: !setlevel <user> <level>", userlevels.Owner, setLevelCmd)
	RegisterCommand("ignore", "Ignore a user completely. Usage: !ignore <nick>", userlevels.Admin, ignoreUserCmd)
	RegisterCommand("unignore", "Stop ignoring a user. Usage: !unignore <nick>", userlevels.Admin, unignoreUserCmd)
	RegisterCommand("ratelimit", "Configure anti-spam rate limiting. Usage: !ratelimit <info|set|reset> [args]", userlevels.Admin, rateLimitCmd)
	RegisterCommand("restart", "Restart the bot (useful for applying plugin changes)", userlevels.Owner, restartCmd)
	RegisterCommand("die", "Shut down the bot. Usage: !die [optional message]", userlevels.Owner, dieCmd)
}
