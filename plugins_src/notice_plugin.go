//go:build notice_plugin
// +build notice_plugin

package main

import (
	"gopkg.in/irc.v4"
	"ircbot/pkg/api"
)

type NoticePlugin struct{}

func (p *NoticePlugin) Name() string {
	return "NoticePlugin"
}

func (p *NoticePlugin) Version() string {
	return "1.0.0"
}

func (p *NoticePlugin) OnLoad() error {
	api.LogSuccess("NoticePlugin version %s loaded!", p.Version())
	return nil
}

func (p *NoticePlugin) OnMessage(c *irc.Client, m *irc.Message) {
}

func (p *NoticePlugin) OnNotice(c *irc.Client, m *irc.Message) {
	api.LogInfo("NoticePlugin: NOTICE received from %s: %s", m.Prefix.Name, m.Trailing())
}

func (p *NoticePlugin) OnUnload() error {
	api.LogSuccess("NoticePlugin version %s unloaded!", p.Version())
	return nil
}

var Plugin NoticePlugin