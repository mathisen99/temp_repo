//go:build quit_plugin
// +build quit_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type QuitPlugin struct{}

func (p *QuitPlugin) Name() string {
	return "QuitPlugin"
}

func (p *QuitPlugin) Version() string {
	return "1.0.0"
}

func (p *QuitPlugin) OnLoad() error {
	api.LogSuccess("QuitPlugin version %s loaded!", p.Version())
	return nil
}

func (p *QuitPlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *QuitPlugin) OnQuit(c *irc.Client, m *irc.Message) {
	reason := m.Trailing()
	user := m.Prefix.Name
	reply := fmt.Sprintf("User %s quit: %s", user, reason)
	api.LogInfo("QuitPlugin: %s", reply)
}

func (p *QuitPlugin) OnUnload() error {
	api.LogSuccess("QuitPlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin QuitPlugin