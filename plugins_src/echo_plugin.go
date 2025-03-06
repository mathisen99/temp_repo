//go:build echo_plugin
// +build echo_plugin

package main

import (
	"strings"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type EchoPlugin struct{}

func (p *EchoPlugin) Name() string {
	return "EchoPlugin"
}

func (p *EchoPlugin) Version() string {
	return "1.0.0"
}

func (p *EchoPlugin) OnLoad() error {
	api.LogSuccess("EchoPlugin version %s loaded!", p.Version())
	return nil
}

func (p *EchoPlugin) OnMessage(c *irc.Client, m *irc.Message) {
	if m.Command == api.CMD_PRIVMSG {
		msg := m.Trailing()
		if strings.HasPrefix(msg, "!echo ") {
			echoText := strings.TrimPrefix(msg, "!echo ")
			target := m.Params[0]
			c.Writef("PRIVMSG %s :%s", target, echoText)
		}
	}
}

func (p *EchoPlugin) OnUnload() error {
	api.LogSuccess("EchoPlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin EchoPlugin
