//go:build sample_plugin
// +build sample_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type SamplePlugin struct{}

func (p *SamplePlugin) Name() string {
	return "SamplePlugin"
}

func (p *SamplePlugin) Version() string {
	return "1.0.0"
}

func (p *SamplePlugin) OnLoad() error {
	api.LogSuccess("SamplePlugin version %s loaded!", p.Version())
	return nil
}

func (p *SamplePlugin) OnMessage(c *irc.Client, m *irc.Message) {
	if m.Command == api.CMD_PRIVMSG && m.Trailing() == "!sample" {
		reply := fmt.Sprintf("Test command received, %s! - from SamplePlugin v%s",
			m.Prefix.Name, p.Version())
		target := m.Params[0]
		c.Writef("%s %s :%s", api.CMD_PRIVMSG, target, reply)
	}
}

func (p *SamplePlugin) OnUnload() error {
	api.LogSuccess("SamplePlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin SamplePlugin
