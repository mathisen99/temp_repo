//go:build kick_plugin
// +build kick_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type KickResponderPlugin struct{}

func (p *KickResponderPlugin) Name() string {
	return "KickResponderPlugin"
}

func (p *KickResponderPlugin) Version() string {
	return "1.0.0"
}

func (p *KickResponderPlugin) OnLoad() error {
	api.LogSuccess("KickResponderPlugin version %s loaded!", p.Version())
	return nil
}

func (p *KickResponderPlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *KickResponderPlugin) OnKick(c *irc.Client, m *irc.Message) {
	channel := m.Params[0]
	kickedUser := m.Params[1]
	reply := fmt.Sprintf("Heads up! User %s was kicked from %s.", kickedUser, channel)
	c.Writef("PRIVMSG %s :%s", channel, reply)
}

func (p *KickResponderPlugin) OnUnload() error {
	api.LogSuccess("KickResponderPlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin KickResponderPlugin