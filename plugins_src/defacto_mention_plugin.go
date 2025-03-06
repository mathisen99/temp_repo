//go:build defacto_mention_plugin
// +build defacto_mention_plugin

package main

import (
	"strings"
	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

// DefactoMentionPlugin is a plugin that responds when the nick 'de-facto' is mentioned.
type DefactoMentionPlugin struct{}

// Name returns the name of the plugin.
func (p *DefactoMentionPlugin) Name() string {
	return "DefactoMentionPlugin"
}

// Version returns the version of the plugin.
func (p *DefactoMentionPlugin) Version() string {
	return "1.0.0"
}

// OnLoad is called when the plugin is loaded.
func (p *DefactoMentionPlugin) OnLoad() error {
	api.LogInfo("DefactoMentionPlugin loaded successfully.")
	return nil
}

// OnMessage handles incoming IRC messages and responds when 'de-facto' is mentioned.
func (p *DefactoMentionPlugin) OnMessage(c *irc.Client, m *irc.Message) {
	// Only process channel messages
	if len(m.Params) == 0 || !api.IsChannelName(m.Params[0]) {
		return
	}

	text := m.Trailing()
	// Check if 'de-facto' is mentioned in the message
	if strings.Contains(strings.ToLower(text), "de-facto") {
		target := api.GetReplyTarget(c, m)
		api.SendMessage(c, target, "de-facto is the man!")
	}
}

// OnUnload is called when the plugin is unloaded.
func (p *DefactoMentionPlugin) OnUnload() error {
	api.LogInfo("DefactoMentionPlugin unloaded successfully.")
	return nil
}

// Export the plugin
var Plugin DefactoMentionPlugin
