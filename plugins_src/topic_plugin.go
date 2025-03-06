//go:build topic_plugin
// +build topic_plugin

package main

import (
	"fmt"

	"gopkg.in/irc.v4"
)

type TopicPlugin struct{}

func (p *TopicPlugin) Name() string {
	return "TopicPlugin"
}

func (p *TopicPlugin) Version() string {
	return "1.0.0"
}

func (p *TopicPlugin) OnLoad() error {
	return nil
}

func (p *TopicPlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *TopicPlugin) OnTopicChange(c *irc.Client, m *irc.Message) {
	if m.Command == "333" {
		return
	}

	var channel string
	if m.Command == "TOPIC" {
		channel = m.Params[0]
	} else if m.Command == "332" {
		channel = m.Params[1]
	} else {
		channel = m.Params[0]
	}
	topic := m.Trailing()
	reply := fmt.Sprintf("The topic for %s has been changed to: %s", channel, topic)
	c.Writef("PRIVMSG %s :%s", channel, reply)
}

func (p *TopicPlugin) OnUnload() error {
	return nil
}

var Plugin TopicPlugin
