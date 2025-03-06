//go:build versioned_sample
// +build versioned_sample

package main

import (
	"fmt"
	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type VersionedSamplePlugin struct{}

// Name returns the plugin's name
func (p *VersionedSamplePlugin) Name() string {
	return "VersionedSample"
}

// Version returns the plugin version
func (p *VersionedSamplePlugin) Version() string {
	return "1.0.2"
}

// OnLoad is called when the plugin is loaded
func (p *VersionedSamplePlugin) OnLoad() error {
	api.LogSuccess("VersionedSample plugin version %s loaded!", p.Version())
	return nil
}

// OnMessage handles all IRC messages
func (p *VersionedSamplePlugin) OnMessage(c *irc.Client, m *irc.Message) {
	if m.Command == api.CMD_PRIVMSG {
		trailing := m.Trailing()

		if trailing == "!version-test" {
			reply := fmt.Sprintf("Hello, %s! I am VersionedSample plugin version onMessage text %s",
				m.Prefix.Name, p.Version())
			target := m.Params[0]
			c.Writef("%s %s :%s", api.CMD_PRIVMSG, target, reply)
		}
	}
}

// GetCommands returns the commands this plugin handles
func (p *VersionedSamplePlugin) GetCommands() []string {
	return []string{"version-test"}
}

// HandleCommand processes commands for this plugin
func (p *VersionedSamplePlugin) HandleCommand(c *irc.Client, m *irc.Message, cmd string, args []string) {
	target := m.Params[0]
	if target == c.CurrentNick() {
		target = m.Prefix.Name
	}

	if cmd == "version-test" {
		reply := fmt.Sprintf("Hello, %s! I am VersionedSample plugin version command handling %s",
			m.Prefix.Name, p.Version())
		c.Writef("%s %s :%s", api.CMD_PRIVMSG, target, reply)
	}
}

// OnUnload is called when the plugin is unloaded
func (p *VersionedSamplePlugin) OnUnload() error {
	api.LogSuccess("VersionedSample plugin version %s unloaded!", p.Version())
	return nil
}

var Plugin VersionedSamplePlugin