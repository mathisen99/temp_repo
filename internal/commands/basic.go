package commands

import (
	"strings"
	
	"gopkg.in/irc.v4"
	"ircbot/internal/logger"
)

// Basic IRC Commands
// Channel Operator Commands
func opCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !op <nick>", m.Params[0])
		return
	}
	target := args[0]
	c.Write("MODE " + m.Params[0] + " +o " + target)
}

func deopCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !deop <nick>", m.Params[0])
		return
	}
	target := args[0]
	c.Write("MODE " + m.Params[0] + " -o " + target)
}

func voiceCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !voice <nick>", m.Params[0])
		return
	}
	target := args[0]
	c.Write("MODE " + m.Params[0] + " +v " + target)
}

func devoiceCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !devoice <nick>", m.Params[0])
		return
	}
	target := args[0]
	c.Write("MODE " + m.Params[0] + " -v " + target)
}

func kickCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !kick <nick> [reason]", m.Params[0])
		return
	}

	target := args[0]
	reason := "Kicked by " + m.Prefix.Name
	if len(args) > 1 {
		reason = strings.Join(args[1:], " ")
	}

	c.Writef("KICK %s %s :%s", m.Params[0], target, reason)
}

func banCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !ban <nick|hostmask>", m.Params[0])
		return
	}

	target := args[0]
	// If it's a nick and not a hostmask, add wildcards
	if !strings.Contains(target, "!") && !strings.Contains(target, "@") {
		target = target + "!*@*"
	}

	c.Write("MODE " + m.Params[0] + " +b " + target)
}

func unbanCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !unban <nick|hostmask>", m.Params[0])
		return
	}

	target := args[0]
	// If it's a nick and not a hostmask, add wildcards
	if !strings.Contains(target, "!") && !strings.Contains(target, "@") {
		target = target + "!*@*"
	}

	c.Write("MODE " + m.Params[0] + " -b " + target)
}

func muteCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !mute <nick|hostmask>", m.Params[0])
		return
	}

	target := args[0]
	// If it's a nick and not a hostmask, add wildcards
	if !strings.Contains(target, "!") && !strings.Contains(target, "@") {
		target = target + "!*@*"
	}

	c.Write("MODE " + m.Params[0] + " +q " + target)
}

func unmuteCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !unmute <nick|hostmask>", m.Params[0])
		return
	}

	target := args[0]
	// If it's a nick and not a hostmask, add wildcards
	if !strings.Contains(target, "!") && !strings.Contains(target, "@") {
		target = target + "!*@*"
	}

	c.Write("MODE " + m.Params[0] + " -q " + target)
}

func inviteCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !invite <nick> [channel]", m.Params[0])
		return
	}

	target := args[0]
	channel := m.Params[0]

	// If channel is a user (private message), we need another argument
	if !strings.HasPrefix(channel, "#") {
		if len(args) < 2 {
			c.Writef("PRIVMSG %s :Usage in PM: !invite <nick> <channel>", m.Prefix.Name)
			return
		}
		channel = args[1]
	} else if len(args) > 1 {
		// If a channel was specified as second argument
		channel = args[1]
	}

	c.Writef("INVITE %s %s", target, channel)
}

func topicCmd(c *irc.Client, m *irc.Message, args []string) {
	channel := m.Params[0]

	// If this is a PM, we need a channel argument
	if !strings.HasPrefix(channel, "#") {
		if len(args) < 2 {
			c.Writef("PRIVMSG %s :Usage in PM: !topic <channel> <topic>", m.Prefix.Name)
			return
		}
		channel = args[0]
		args = args[1:]
	}

	if len(args) == 0 {
		// Just query the topic
		c.Writef("TOPIC %s", channel)
		return
	}

	// Set the topic
	topic := strings.Join(args, " ")
	c.Writef("TOPIC %s :%s", channel, topic)
}

// Channel Join/Part Commands
func joinCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !join <channel> [key]", m.Params[0])
		return
	}

	channel := args[0]

	if len(args) > 1 {
		key := args[1]
		c.Writef("JOIN %s %s", channel, key)
	} else {
		c.Writef("JOIN %s", channel)
	}
}

func partCmd(c *irc.Client, m *irc.Message, args []string) {
	channel := m.Params[0]
	reason := "Leaving"

	// If this is a PM, we need a channel argument
	if !strings.HasPrefix(channel, "#") {
		if len(args) < 1 {
			c.Writef("PRIVMSG %s :Usage in PM: !part <channel> [reason]", m.Prefix.Name)
			return
		}
		channel = args[0]
		if len(args) > 1 {
			reason = strings.Join(args[1:], " ")
		}
	} else if len(args) > 0 {
		reason = strings.Join(args, " ")
	}

	c.Writef("PART %s :%s", channel, reason)
}

// Bot Control Commands
func nickCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !nick <new_nickname>", m.Params[0])
		return
	}

	newNick := args[0]
	c.Writef("NICK %s", newNick)
}

// Message Commands
func sayCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		usageMsg := "Usage: !say <message>"
		c.Writef("PRIVMSG %s :%s", m.Params[0], usageMsg)
		// Log the bot's message if it's a channel
		if strings.HasPrefix(m.Params[0], "#") {
			logger.LogBotChannelMessage(m.Params[0], c.CurrentNick(), usageMsg)
		}
		return
	}

	message := strings.Join(args, " ")
	c.Writef("PRIVMSG %s :%s", m.Params[0], message)
	
	// Log the bot's message if it's a channel
	if strings.HasPrefix(m.Params[0], "#") {
		logger.LogBotChannelMessage(m.Params[0], c.CurrentNick(), message)
	}
}

func msgCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 2 {
		usageMsg := "Usage: !msg <target> <message>"
		c.Writef("PRIVMSG %s :%s", m.Params[0], usageMsg)
		// Log the bot's message if it's a channel
		if strings.HasPrefix(m.Params[0], "#") {
			logger.LogBotChannelMessage(m.Params[0], c.CurrentNick(), usageMsg)
		}
		return
	}

	target := args[0]
	message := strings.Join(args[1:], " ")
	c.Writef("PRIVMSG %s :%s", target, message)
	
	// Log the bot's message if the target is a channel
	if strings.HasPrefix(target, "#") {
		logger.LogBotChannelMessage(target, c.CurrentNick(), message)
	}
}

func noticeCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 2 {
		c.Writef("PRIVMSG %s :Usage: !notice <target> <message>", m.Params[0])
		return
	}

	target := args[0]
	message := strings.Join(args[1:], " ")
	c.Writef("NOTICE %s :%s", target, message)
}

func actionCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		usageMsg := "Usage: !action <message>"
		c.Writef("PRIVMSG %s :%s", m.Params[0], usageMsg)
		// Log the bot's message if it's a channel
		if strings.HasPrefix(m.Params[0], "#") {
			logger.LogBotChannelMessage(m.Params[0], c.CurrentNick(), usageMsg)
		}
		return
	}

	action := strings.Join(args, " ")
	// CTCP ACTION format
	c.Writef("PRIVMSG %s :\x01ACTION %s\x01", m.Params[0], action)
	
	// Log the bot's action if it's a channel
	if strings.HasPrefix(m.Params[0], "#") {
		logger.LogChannelAction(m.Params[0], c.CurrentNick(), action)
	}
}

// Information Commands
func whoisCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !whois <nick>", m.Params[0])
		return
	}

	target := args[0]
	c.Writef("WHOIS %s", target)
}

func modeCmd(c *irc.Client, m *irc.Message, args []string) {
	if len(args) < 1 {
		c.Writef("PRIVMSG %s :Usage: !mode <target> [modes]", m.Params[0])
		return
	}

	target := args[0]

	if len(args) == 1 {
		// Just query the modes
		c.Writef("MODE %s", target)
		return
	}

	// Set the modes
	modes := strings.Join(args[1:], " ")
	c.Writef("MODE %s %s", target, modes)
}
