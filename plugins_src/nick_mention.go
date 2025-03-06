//go:build nick_mention
// +build nick_mention

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type NickPlugin struct{}

func (p *NickPlugin) Name() string {
	return "NickPlugin"
}

func (p *NickPlugin) Version() string {
	return "1.0.0"
}

func (p *NickPlugin) OnLoad() error {
	api.LogSuccess("NickPlugin version %s loaded!", p.Version())
	return nil
}

func (p *NickPlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *NickPlugin) OnNickMention(c *irc.Client, m *irc.Message) {
	target := m.Params[0]
	reply := fmt.Sprintf("You mentioned me, %s?", m.Prefix.Name)
	c.Writef("PRIVMSG %s :%s", target, reply)
}

// OnUnload is called when the plugin is unloaded
func (p *NickPlugin) OnUnload() error {
	api.LogSuccess("NickPlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin NickPlugin