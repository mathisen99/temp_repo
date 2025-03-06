package handlers

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/logger"
	"ircbot/internal/plugin"
	"ircbot/internal/userlevels"
)

// containsNick checks if the bot's nick is mentioned in the text.
// It tries to detect actual nick mentions rather than just substring matches.
func containsNick(nick, text string) bool {
	// Convert both to lowercase for case-insensitive comparison
	lowerText := strings.ToLower(text)
	lowerNick := strings.ToLower(nick)
	
	// Check for exact matches or common addressing patterns
	patterns := []string{
		// Exact nick
		lowerNick + " ",
		" " + lowerNick,
		// Common nick addressing patterns
		lowerNick + ":",
		lowerNick + ",",
		lowerNick + ".",
		lowerNick + "?",
		lowerNick + "!",
		"@" + lowerNick,
	}
	
	// First check if the text is just the nick by itself
	if lowerText == lowerNick {
		return true
	}
	
	// Check for the nick at the beginning of the message
	if strings.HasPrefix(lowerText, lowerNick) {
		// Check if the character after the nick is a space or punctuation
		if len(lowerText) > len(lowerNick) {
			nextChar := lowerText[len(lowerNick)]
			if nextChar == ' ' || strings.ContainsAny(string(nextChar), ",:;.!?") {
				return true
			}
		}
	}
	
	// Check each pattern
	for _, pattern := range patterns {
		if strings.Contains(lowerText, pattern) {
			return true
		}
	}
	
	// Check for the nick surrounded by word boundaries
	// This handles cases like "I saw nick yesterday"
	if strings.Contains(lowerText, " "+lowerNick+" ") {
		return true
	}
	
	// Check for the nick at the end of the message
	if strings.HasSuffix(lowerText, " "+lowerNick) {
		return true
	}
	
	// Fall back to a simple substring check, but only when it's a whole word
	// This helps avoid false positives when the nick is part of another word
	words := strings.Fields(lowerText)
	for _, word := range words {
		if word == lowerNick || 
		   strings.TrimRight(word, ",:;.!?") == lowerNick {
			return true
		}
	}
	
	return false
}

// HandlePing responds to a PING message.
func HandlePing(c *irc.Client, m *irc.Message) {
	logger.Successf(">> PING received: %s", m.Trailing())
	err := c.Writef("%s :%s", internal.CMD_PONG, m.Trailing())
	if err != nil {
		logger.Errorf(">> Error sending PONG: %v", err)
	} else {
		logger.Successf(">> Sent PONG")
	}
}

// HandleMessages processes incoming messages and dispatches them to the appropriate handlers.
func HandleMessages(c *irc.Client, m *irc.Message, password string, channels []string) {
	switch m.Command {
	// Connection Registration
	case internal.RPL_WELCOME: // 001
		logger.Successf(">> Welcome message received: %s", m.Trailing())
		if password != "" {
			if err := c.Writef("%s NickServ :IDENTIFY %s", internal.CMD_PRIVMSG, password); err != nil {
				logger.Errorf(">> Error identifying with NickServ: %v", err)
			} else {
				logger.Successf(">> Identifying with NickServ...")
			}
		}
	case internal.RPL_YOURHOST, internal.RPL_CREATED, internal.RPL_MYINFO, internal.RPL_ISUPPORT:
		logger.Infof(">> Server Info: %s", m.Trailing())

	// Connection Status
	case "250": // No constant for this in RFC
		logger.Infof(">> Connection statistics: %s", m.Trailing())
	case internal.RPL_LUSERCLIENT, internal.RPL_LUSEROP, internal.RPL_LUSERUNKNOWN,
		internal.RPL_LUSERCHANNELS, internal.RPL_LUSERME:
		logger.Infof(">> LUSER Info: %s", m.Trailing())
	case internal.RPL_LOCALUSERS, internal.RPL_GLOBALUSERS:
		logger.Infof(">> Users info: %s", m.Trailing())
	case internal.RPL_USERHOST:
		logger.Whitef(">> ISON response: %s", m.Trailing())

	// MOTD handling
	case internal.RPL_MOTDSTART:
		logger.Bluef(">> MOTD Start: %s", m.Trailing())
	case internal.RPL_MOTD:
		logger.Bluef(">> MOTD: %s", m.Trailing())
	case internal.RPL_ENDOFMOTD:
		userlevels.LoadHostmasks()
		logger.Infof(">> User levels loaded from settings")
		logger.Bluef(">> End of MOTD")

		// Delay channel join to allow proper identification
		time.Sleep(5 * time.Second)
		for _, channel := range channels {
			if err := c.Writef("%s %s", internal.CMD_JOIN, channel); err != nil {
				logger.Errorf(">> Error joining channel %s: %v", channel, err)
			} else {
				logger.Successf(">> Joining channel: %s", channel)
			}
		}

	// PING/PONG
	case internal.CMD_PING:
		HandlePing(c, m)
	case internal.CMD_PONG:
		logger.Successf(">> PONG received from server: %s", m.Trailing())

	// Standard IRC Commands
	case internal.CMD_JOIN:
		channel := m.Params[0]
		nickname := m.Prefix.Name
		logger.Infof(">> %s joined %s", nickname, channel)
		logger.LogChannelEvent(channel, nickname + " has joined the channel")
	case internal.CMD_PART:
		channel := m.Params[0]
		nickname := m.Prefix.Name
		reason := m.Trailing()
		logger.Infof(">> %s left %s: %s", nickname, channel, reason)
		if reason != "" {
			logger.LogChannelEvent(channel, nickname + " has left the channel (" + reason + ")")
		} else {
			logger.LogChannelEvent(channel, nickname + " has left the channel")
		}
	case internal.CMD_KICK:
		channel := m.Params[0]
		kickedUser := m.Params[1]
		kicker := m.Prefix.Name
		reason := m.Trailing()
		logger.Warnf(">> %s was kicked from %s by %s: %s", kickedUser, channel, kicker, reason)
		logger.LogChannelEvent(channel, kickedUser + " was kicked by " + kicker + " (" + reason + ")")
	case internal.CMD_QUIT:
		logger.Infof(">> %s quit: %s", m.Prefix.Name, m.Trailing())
	case internal.CMD_NICK:
		logger.Infof(">> %s changed their nickname to %s", m.Prefix.Name, m.Trailing())
	case internal.CMD_INVITE:
		logger.Infof(">> %s invited %s to %s", m.Prefix.Name, m.Params[0], m.Trailing())
	case internal.CMD_TOPIC:
		channel := m.Params[0]
		nickname := m.Prefix.Name
		topic := m.Trailing()
		logger.Infof(">> %s changed the topic of %s to: %s", nickname, channel, topic)
		logger.LogChannelEvent(channel, nickname + " changed the topic to: " + topic)
	case internal.CMD_NOTICE:
		logger.Infof(">> NOTICE from %s: %s", m.Prefix.Name, m.Trailing())
	case internal.CMD_PRIVMSG:
		if len(m.Params) > 0 && m.Params[0] == c.CurrentNick() {
			// This is a private message to the bot
			nickname := m.Prefix.Name
			message := m.Trailing()
			logger.Whitef(">> Private message from %s: %s", nickname, message)
			// Logging is done inside HandlePrivateMessage
			HandlePrivateMessage(c, m)
		} else {
			// This is a channel message
			nickname := m.Prefix.Name
			message := m.Trailing()
			logger.Infof(">> %s: %s", nickname, message)
			// Logging is done inside HandleChannelMessage 
			HandleChannelMessage(c, m)
		}
	case internal.CMD_ERROR:
		logger.Errorf(">> ERROR: %s", m.Trailing())

	// Mode changes
	case internal.CMD_MODE:
		if len(m.Params) > 1 {
			channel := m.Params[0]
			mode := m.Params[1]
			changer := m.Prefix.Name

			handleModeChange(mode, changer, channel, m)
		} else {
			logger.Infof(">> Mode change: %s", m.Trailing())
		}

	// Numeric Replies
	case internal.RPL_AWAY:
		logger.Whitef(">> %s is away: %s", m.Params[1], m.Trailing())
	case internal.RPL_UNAWAY:
		logger.Whitef(">> User is no longer away: %s", m.Trailing())
	case internal.RPL_TOPIC:
		channel := m.Params[1]
		topic := m.Trailing()
		logger.Bluef(">> Topic for %s: %s", channel, topic)
		logger.LogChannelEvent(channel, "Topic is changed")
	case internal.RPL_TOPICWHOTIME:
		channel := m.Params[1]
		setter := m.Params[2]
		timeStamp := m.Params[3]
		logger.Bluef(">> Topic set by %s at %s", setter, timeStamp)
		logger.LogChannelEvent(channel, "Topic set by " + setter + " at " + timeStamp)
	case internal.RPL_NAMREPLY:
		logger.Infof(">> Users in %s: %s", m.Params[2], m.Trailing())
	case internal.RPL_ENDOFNAMES:
		logger.Infof(">> End of /NAMES list for %s", m.Params[1])
	case internal.RPL_LOGGEDIN:
		logger.Successf(">> You are now logged in as %s", m.Params[2])
	case internal.RPL_HOSTHIDDEN:
		logger.Successf(">> Your hidden host is now: %s", m.Params[1])

	// Error Replies
	case internal.ERR_NICKNAMEINUSE:
		logger.Errorf(">> Nickname %s is already in use. Please choose a different one.", c.CurrentNick())
	case internal.ERR_BANNEDFROMCHAN:
		logger.Errorf(">> You are banned from the channel %s", m.Params[1])
	case internal.ERR_CHANNELISFULL:
		logger.Errorf(">> Channel %s is full", m.Params[1])
	case internal.ERR_INVITEONLYCHAN:
		logger.Errorf(">> Cannot join channel %s (invite-only)", m.Params[1])
	case internal.ERR_BADCHANNELKEY:
		logger.Errorf(">> Cannot join channel %s (bad key)", m.Params[1])
	case internal.ERR_CHANOPRIVSNEEDED:
		logger.Errorf(">> Channel operator privileges needed for channel %s", m.Params[1])
	default:
		logger.Whitef(">> Unhandled message [%s]: %s", m.Command, m.Trailing())
	}

	// Dispatch the message to all loaded plugins
	dispatchToPlugins(c, m)
}

// dispatchToPlugins routes messages to the appropriate plugin handlers
func dispatchToPlugins(c *irc.Client, m *irc.Message) {
	for _, plug := range plugin.GetPluginList() {
		// Always call the generic handler
		plug.OnMessage(c, m)

		// Call specialized handlers based on command type
		switch m.Command {
		case internal.CMD_PRIVMSG:
			if handler, ok := plug.(plugin.PrivMsgHandler); ok {
				handler.OnPrivMsg(c, m)
			}
			if containsNick(c.CurrentNick(), m.Trailing()) {
				if handler, ok := plug.(plugin.NickMentionHandler); ok {
					handler.OnNickMention(c, m)
				}
			}
		case internal.CMD_KICK:
			if handler, ok := plug.(plugin.KickHandler); ok {
				handler.OnKick(c, m)
			}
		case internal.CMD_JOIN:
			if handler, ok := plug.(plugin.JoinHandler); ok {
				handler.OnJoin(c, m)
			}
		case internal.CMD_PART:
			if handler, ok := plug.(plugin.PartHandler); ok {
				handler.OnPart(c, m)
			}
		case internal.CMD_QUIT:
			if handler, ok := plug.(plugin.QuitHandler); ok {
				handler.OnQuit(c, m)
			}
		case internal.CMD_NICK:
			if handler, ok := plug.(plugin.NickHandler); ok {
				handler.OnNickChange(c, m)
			}
		case internal.CMD_INVITE:
			if handler, ok := plug.(plugin.InviteHandler); ok {
				handler.OnInvite(c, m)
			}
		case internal.CMD_TOPIC, internal.RPL_TOPIC, internal.RPL_TOPICWHOTIME:
			if handler, ok := plug.(plugin.TopicChangeHandler); ok {
				handler.OnTopicChange(c, m)
			}
		case internal.CMD_NOTICE:
			if handler, ok := plug.(plugin.NoticeHandler); ok {
				handler.OnNotice(c, m)
			}
		case internal.CMD_ERROR:
			if handler, ok := plug.(plugin.ErrorHandler); ok {
				handler.OnError(c, m)
			}
		case internal.CMD_MODE:
			if handler, ok := plug.(plugin.ModeHandler); ok {
				handler.OnMode(c, m)
			}
		}
	}
}

// handleModeChange processes IRC mode changes with cleaner code organization
func handleModeChange(mode, changer, channel string, m *irc.Message) {
	// Handle user-targeted modes (+o, -o, +v, etc.)
	if len(m.Params) > 2 && (strings.HasPrefix(mode, "+") || strings.HasPrefix(mode, "-")) {
		target := m.Params[2]
		modeChar := mode[1:]
		isAdding := mode[0] == '+'

		switch modeChar {
		case "o":
			action := "given operator status"
			if !isAdding {
				action = "de-opped"
			}
			logger.Infof(">> %s was %s in %s by %s", target, action, channel, changer)
		case "v":
			action := "given voice"
			if !isAdding {
				action = "had voice removed"
			}
			logger.Infof(">> %s was %s in %s by %s", target, action, channel, changer)
		case "h":
			action := "given half-op"
			if !isAdding {
				action = "had half-op removed"
			}
			logger.Infof(">> %s was %s in %s by %s", target, action, channel, changer)
		case "q":
			action := "given owner status"
			if !isAdding {
				action = "had owner status removed"
			}
			logger.Infof(">> %s was %s in %s by %s", target, action, channel, changer)
		case "a":
			action := "given admin status"
			if !isAdding {
				action = "had admin status removed"
			}
			logger.Infof(">> %s was %s in %s by %s", target, action, channel, changer)
		case "b":
			if isAdding {
				logger.Infof(">> %s set a ban on %s in %s", changer, target, channel)
			} else {
				logger.Infof(">> %s lifted the ban on %s in %s", changer, target, channel)
			}
		case "e":
			if isAdding {
				logger.Infof(">> %s set a ban exception on %s in %s", changer, target, channel)
			} else {
				logger.Infof(">> %s removed the ban exception on %s in %s", changer, target, channel)
			}
		case "I":
			if isAdding {
				logger.Infof(">> %s set an invite exception for %s in %s", changer, target, channel)
			} else {
				logger.Infof(">> %s removed the invite exception for %s in %s", changer, target, channel)
			}
		default:
			logger.Infof(">> %s changed mode for %s in %s: %s", changer, target, channel, mode)
		}
		return
	}

	// Handle channel modes (+m, -i, etc.)
	channelModeDescriptions := map[string]string{
		"+i": "set channel %s to invite-only",
		"-i": "removed invite-only mode from channel %s",
		"+m": "set channel %s to moderated",
		"-m": "removed moderated mode from channel %s",
		"+n": "set channel %s to disallow external messages",
		"-n": "allowed external messages in channel %s",
		"+s": "set channel %s to secret",
		"-s": "removed secret mode from channel %s",
		"+p": "set channel %s to private",
		"-p": "removed private mode from channel %s",
		"+t": "set topic-lock on %s",
		"-t": "removed topic-lock from %s",
		"+k": "set a password on channel %s",
		"-k": "removed the password from channel %s",
	}

	if desc, exists := channelModeDescriptions[mode]; exists {
		logger.Infof(">> %s %s", changer, fmt.Sprintf(desc, channel))
	} else if mode == "+l" && len(m.Params) > 2 {
		logger.Infof(">> %s set a user limit of %s on channel %s", changer, m.Params[2], channel)
	} else if mode == "-l" {
		logger.Infof(">> %s removed the user limit from channel %s", changer, channel)
	} else {
		logger.Infof(">> Mode change on %s: %s by %s", channel, mode, changer)
	}
}
