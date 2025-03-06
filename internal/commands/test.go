package commands

import (
	"gopkg.in/irc.v4"
	"ircbot/internal"
)

func testCommand(c *irc.Client, m *irc.Message, args []string) {
	channel := m.Params[0]
	c.Writef("%s %s :%s", internal.CMD_PRIVMSG, channel, "Test command received!")
}