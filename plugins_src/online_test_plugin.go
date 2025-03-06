//go:build online_test_plugin
// +build online_test_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type OnlineTestPlugin struct{}

func (p *OnlineTestPlugin) Name() string {
	return "OnlineTestPlugin"
}

func (p *OnlineTestPlugin) Version() string {
	return "1.0.0"
}

func (p *OnlineTestPlugin) OnLoad() error {
	api.LogSuccess("OnlineTestPlugin version %s loaded successfully from URL!", p.Version())
	return nil
}

func (p *OnlineTestPlugin) OnMessage(c *irc.Client, m *irc.Message) {
	if m.Command == api.CMD_PRIVMSG && m.Trailing() == "!online-test" {
		replyTarget := m.Params[0]
		if replyTarget == c.CurrentNick() {
			replyTarget = m.Prefix.Name
		}

		reply := fmt.Sprintf("Hello %s! I'm a plugin that was loaded from a URL (version %s)!", 
			m.Prefix.Name, p.Version())
		c.Writef("PRIVMSG %s :%s", replyTarget, reply)
	}
}

func (p *OnlineTestPlugin) OnUnload() error {
	api.LogSuccess("OnlineTestPlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin OnlineTestPlugin