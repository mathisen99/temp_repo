//go:build nick_change_plugin
// +build nick_change_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type NickChangePlugin struct{}

func (p *NickChangePlugin) Name() string {
	return "NickChangePlugin"
}

func (p *NickChangePlugin) Version() string {
	return "1.0.0"
}

func (p *NickChangePlugin) OnLoad() error {
	api.LogSuccess("NickChangePlugin version %s loaded!", p.Version())
	return nil
}

func (p *NickChangePlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *NickChangePlugin) OnNickChange(c *irc.Client, m *irc.Message) {
	oldNick := m.Prefix.Name
	newNick := m.Trailing()
	reply := fmt.Sprintf("Nick change detected: %s is now known as %s", oldNick, newNick)
	if len(m.Params) > 0 {
		target := m.Params[0]
		c.Writef("PRIVMSG %s :%s", target, reply)
	}
}

func (p *NickChangePlugin) OnUnload() error {
	api.LogSuccess("NickChangePlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin NickChangePlugin