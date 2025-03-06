// Package api provides a public interface for IRC bot plugins.
// It allows plugins to use bot functionality without directly importing internal packages.
package api

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/irc.v4"
	"ircbot/internal/logger"
)

// Log functions that plugins can use
func LogInfo(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func LogError(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

func LogSuccess(format string, args ...interface{}) {
	logger.Successf(format, args...)
}

func LogWarn(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

func LogDebug(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

// IRC Command constants
const (
	CMD_PRIVMSG = "PRIVMSG"
	CMD_NOTICE  = "NOTICE"
	CMD_JOIN    = "JOIN"
	CMD_PART    = "PART"
	CMD_QUIT    = "QUIT"
	CMD_KICK    = "KICK"
	CMD_NICK    = "NICK"
	CMD_TOPIC   = "TOPIC"
	CMD_INVITE  = "INVITE"
	CMD_MODE    = "MODE"
)

// IRC Numeric reply constants
const (
	RPL_WELCOME           = "001"
	RPL_YOURHOST          = "002"
	RPL_CREATED           = "003"
	RPL_MYINFO            = "004"
	RPL_ISUPPORT          = "005"
	RPL_UMODEIS           = "221"
	RPL_LUSERCLIENT       = "251"
	RPL_LUSEROP           = "252"
	RPL_LUSERUNKNOWN      = "253"
	RPL_LUSERCHANNELS     = "254"
	RPL_LUSERME           = "255"
	RPL_AWAY              = "301"
	RPL_WHOISUSER         = "311"
	RPL_WHOISSERVER       = "312"
	RPL_WHOISOPERATOR     = "313"
	RPL_WHOISIDLE         = "317"
	RPL_ENDOFWHOIS        = "318"
	RPL_WHOISCHANNELS     = "319"
	RPL_ENDOFWHO          = "315"
	RPL_CHANNELMODEIS     = "324"
	RPL_NOTOPIC           = "331"
	RPL_TOPIC             = "332"
	RPL_TOPICWHOTIME      = "333"
	RPL_NAMREPLY          = "353"
	RPL_ENDOFNAMES        = "366"
	RPL_MOTD              = "372"
	RPL_MOTDSTART         = "375"
	RPL_ENDOFMOTD         = "376"
	ERR_NOSUCHNICK        = "401"
	ERR_NOSUCHCHANNEL     = "403"
	ERR_CANNOTSENDTOCHAN  = "404"
	ERR_UNKNOWNCOMMAND    = "421"
	ERR_NICKNAMEINUSE     = "433"
	ERR_NOTONCHANNEL      = "442"
	ERR_NEEDMOREPARAMS    = "461"
	ERR_CHANOPRIVSNEEDED  = "482"
)

// IRC Colors
const (
	ColorWhite       = "\x0300"
	ColorBlack       = "\x0301"
	ColorBlue        = "\x0302"
	ColorGreen       = "\x0303"
	ColorRed         = "\x0304"
	ColorBrown       = "\x0305"
	ColorPurple      = "\x0306"
	ColorOrange      = "\x0307"
	ColorYellow      = "\x0308"
	ColorLightGreen  = "\x0309"
	ColorCyan        = "\x0310"
	ColorLightCyan   = "\x0311"
	ColorLightBlue   = "\x0312"
	ColorPink        = "\x0313"
	ColorGray        = "\x0314"
	ColorLightGray   = "\x0315"
	Bold             = "\x02"
	Italic           = "\x1D"
	Underline        = "\x1F"
	Reset            = "\x0F"
)

// ColorText returns colored text for IRC
func ColorText(text string, color string) string {
	return color + text + Reset
}

// BoldText returns bold text for IRC
func BoldText(text string) string {
	return Bold + text + Reset
}

// ItalicText returns italic text for IRC
func ItalicText(text string) string {
	return Italic + text + Reset
}

// UnderlineText returns underlined text for IRC
func UnderlineText(text string) string {
	return Underline + text + Reset
}

// FormatIRCTimestamp formats a time as HH:MM:SS for IRC display
func FormatIRCTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}

// IsChannelName returns true if the string appears to be a valid IRC channel name
func IsChannelName(name string) bool {
	if len(name) == 0 {
		return false
	}
	
	// Channels typically start with # or &
	return name[0] == '#' || name[0] == '&'
}

// GetReplyTarget returns the appropriate target for a reply
// If the message was sent to the bot directly, reply to the sender
// Otherwise reply to the channel
func GetReplyTarget(client *irc.Client, message *irc.Message) string {
	if message.Params[0] == client.CurrentNick() {
		return message.Prefix.Name
	}
	return message.Params[0]
}

// ParseHostmask parses an IRC hostmask into its components (nick, user, host)
func ParseHostmask(hostmask string) (nick, user, host string) {
	nickEnd := strings.Index(hostmask, "!")
	if nickEnd == -1 {
		return hostmask, "", ""
	}
	
	nick = hostmask[:nickEnd]
	
	userEnd := strings.Index(hostmask[nickEnd+1:], "@")
	if userEnd == -1 {
		return nick, hostmask[nickEnd+1:], ""
	}
	
	userEnd += nickEnd + 1
	user = hostmask[nickEnd+1:userEnd]
	host = hostmask[userEnd+1:]
	
	return nick, user, host
}

// SendMessage sends an IRC message to the specified target
func SendMessage(client *irc.Client, target, message string) error {
	return client.Writef("%s %s :%s", CMD_PRIVMSG, target, message)
}

// SendNotice sends an IRC notice to the specified target
func SendNotice(client *irc.Client, target, message string) error {
	return client.Writef("%s %s :%s", CMD_NOTICE, target, message)
}

// MessageIsCommand returns true if the message appears to be a bot command
// and returns the command and arguments
func MessageIsCommand(message *irc.Message, prefix string) (isCommand bool, command string, args []string) {
	if message.Command != CMD_PRIVMSG {
		return false, "", nil
	}
	
	text := message.Trailing()
	if !strings.HasPrefix(text, prefix) {
		return false, "", nil
	}
	
	// Remove the prefix
	text = strings.TrimPrefix(text, prefix)
	
	// Split the text into command and arguments
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false, "", nil
	}
	
	return true, fields[0], fields[1:]
}

// JoinChannel sends a JOIN command to the specified channel
func JoinChannel(client *irc.Client, channel string) error {
	return client.Writef("%s %s", CMD_JOIN, channel)
}

// PartChannel sends a PART command to leave the specified channel
func PartChannel(client *irc.Client, channel, reason string) error {
	if reason == "" {
		return client.Writef("%s %s", CMD_PART, channel)
	}
	return client.Writef("%s %s :%s", CMD_PART, channel, reason)
}

// KickUser kicks a user from a channel
func KickUser(client *irc.Client, channel, user, reason string) error {
	if reason == "" {
		return client.Writef("%s %s %s", CMD_KICK, channel, user)
	}
	return client.Writef("%s %s %s :%s", CMD_KICK, channel, user, reason)
}

// SetMode sets a mode on a channel or user
func SetMode(client *irc.Client, target, mode string) error {
	return client.Writef("%s %s %s", CMD_MODE, target, mode)
}

// SetTopic sets the topic for a channel
func SetTopic(client *irc.Client, channel, topic string) error {
	return client.Writef("%s %s :%s", CMD_TOPIC, channel, topic)
}

// FormatCommandHelp formats help text for a command
func FormatCommandHelp(command, syntax, description string) string {
	return fmt.Sprintf("%s: %s - %s", BoldText(command), syntax, description)
}

// SplitMessageForIRC splits a long message into multiple IRC-friendly parts
// to avoid hitting message length limits
func SplitMessageForIRC(message string, maxLength int) []string {
	if maxLength <= 0 {
		maxLength = 400 // Default IRC message length limit
	}
	
	var parts []string
	for len(message) > maxLength {
		// Find the last space before the limit
		cutPoint := maxLength
		for cutPoint > 0 && message[cutPoint] != ' ' {
			cutPoint--
		}
		
		if cutPoint == 0 {
			// No space found, just cut at maxLength
			cutPoint = maxLength
		}
		
		parts = append(parts, message[:cutPoint])
		message = message[cutPoint:]
		if len(message) > 0 && message[0] == ' ' {
			message = message[1:]
		}
	}
	
	if len(message) > 0 {
		parts = append(parts, message)
	}
	
	return parts
}