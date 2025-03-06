//go:build invite_plugin
// +build invite_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type InvitePlugin struct{}

func (p *InvitePlugin) Name() string {
	return "InvitePlugin"
}

func (p *InvitePlugin) Version() string {
	return "1.0.0"
}

func (p *InvitePlugin) OnLoad() error {
	api.LogSuccess("InvitePlugin version %s loaded!", p.Version())
	return nil
}

func (p *InvitePlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *InvitePlugin) OnInvite(c *irc.Client, m *irc.Message) {
	invitedNick := m.Params[0]
	channel := m.Trailing()
	reply := fmt.Sprintf("Invitation: %s, you are invited to join %s!", invitedNick, channel)
	c.Writef("PRIVMSG %s :%s", m.Prefix.Name, reply)
}

func (p *InvitePlugin) OnUnload() error {
	api.LogSuccess("InvitePlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin InvitePlugin