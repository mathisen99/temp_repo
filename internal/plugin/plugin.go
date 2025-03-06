package plugin

import "gopkg.in/irc.v4"

// Plugin defines the basic plugin interface.
type Plugin interface {
	Name() string
	Version() string
	OnLoad() error
	OnMessage(c *irc.Client, m *irc.Message)
}

// CommandHandler interface for plugins that want to register commands
type CommandHandler interface {
	GetCommands() []string
	HandleCommand(c *irc.Client, m *irc.Message, cmd string, args []string)
}

type NickMentionHandler interface {
	OnNickMention(c *irc.Client, m *irc.Message)
}

type KickHandler interface {
	OnKick(c *irc.Client, m *irc.Message)
}

type TopicChangeHandler interface {
	OnTopicChange(c *irc.Client, m *irc.Message)
}

type JoinHandler interface {
	OnJoin(c *irc.Client, m *irc.Message)
}

type PartHandler interface {
	OnPart(c *irc.Client, m *irc.Message)
}

type QuitHandler interface {
	OnQuit(c *irc.Client, m *irc.Message)
}

type NickHandler interface {
	OnNickChange(c *irc.Client, m *irc.Message)
}

type InviteHandler interface {
	OnInvite(c *irc.Client, m *irc.Message)
}

type NoticeHandler interface {
	OnNotice(c *irc.Client, m *irc.Message)
}

type PrivMsgHandler interface {
	OnPrivMsg(c *irc.Client, m *irc.Message)
}

type ErrorHandler interface {
	OnError(c *irc.Client, m *irc.Message)
}

type ModeHandler interface {
	OnMode(c *irc.Client, m *irc.Message)
}
