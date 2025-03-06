//go:build join_plugin
// +build join_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type JoinPlugin struct{}

func (p *JoinPlugin) Name() string {
	return "JoinPlugin"
}

func (p *JoinPlugin) Version() string {
	return "1.0.0"
}

func (p *JoinPlugin) OnLoad() error {
	api.LogSuccess("JoinPlugin version %s loaded!", p.Version())
	return nil
}

func (p *JoinPlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *JoinPlugin) OnJoin(c *irc.Client, m *irc.Message) {
	channel := m.Params[0]
	user := m.Prefix.Name
	reply := fmt.Sprintf("Welcome %s to %s!", user, channel)
	c.Writef("PRIVMSG %s :%s", channel, reply)
}

func (p *JoinPlugin) OnUnload() error {
	api.LogSuccess("JoinPlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin JoinPlugin