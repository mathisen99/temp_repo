//go:build part_plugin
// +build part_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type PartPlugin struct{}

func (p *PartPlugin) Name() string {
	return "PartPlugin"
}

func (p *PartPlugin) Version() string {
	return "1.0.0"
}

func (p *PartPlugin) OnLoad() error {
	api.LogSuccess("PartPlugin version %s loaded!", p.Version())
	return nil
}

func (p *PartPlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *PartPlugin) OnPart(c *irc.Client, m *irc.Message) {
	channel := m.Params[0]
	user := m.Prefix.Name
	reply := fmt.Sprintf("Goodbye %s from %s!", user, channel)
	c.Writef("PRIVMSG %s :%s", channel, reply)
}

func (p *PartPlugin) OnUnload() error {
	api.LogSuccess("PartPlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin PartPlugin