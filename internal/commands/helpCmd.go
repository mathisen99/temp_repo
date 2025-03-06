package commands

import (
	"fmt"
	"gopkg.in/irc.v4"
	"ircbot/internal/logger"
	"ircbot/internal/plugin"
	"ircbot/internal/userlevels"
	"sort"
	"strings"
)

func helpCmd(c *irc.Client, m *irc.Message, args []string) {
	hostmask := m.Prefix.String()
	userLevel := userlevels.GetUserLevelByHostmask(hostmask)

	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	var availableCommands []string

	var commandNames []string
	for name := range commandRegistry {
		commandNames = append(commandNames, name)
	}
	sort.Strings(commandNames)

	for _, name := range commandNames {
		cmd := commandRegistry[name]
		if userLevel >= cmd.RequiredLevel {
			availableCommands = append(availableCommands, fmt.Sprintf("!%s (%s) - %s",
				name, userlevels.LevelName(cmd.RequiredLevel), cmd.Description))
		}
	}

	pluginList := plugin.GetPluginList()
	if len(pluginList) > 0 {
		var pluginCommands []string

		for _, p := range pluginList {
			if cmdHandler, ok := p.(plugin.CommandHandler); ok {
				commands := cmdHandler.GetCommands()
				for _, cmd := range commands {
					pluginCommands = append(pluginCommands, fmt.Sprintf("!%s (Plugin: %s)",
						cmd, p.Name()))
				}
			} else {
				if commands, ok := plugin.GetLegacyPluginCommands(p.Name()); ok && len(commands) > 0 {
					for _, cmd := range commands {
						pluginCommands = append(pluginCommands, fmt.Sprintf("!%s (Legacy Plugin: %s)",
							cmd, p.Name()))
					}
				}
			}
		}

		sort.Strings(pluginCommands)
		availableCommands = append(availableCommands, pluginCommands...)
	}

	if len(availableCommands) == 0 {
		err := c.Writef("PRIVMSG %s :No commands available for your permission level (%s)",
			replyTarget, userlevels.LevelName(userLevel))
		if err != nil {
			logger.Errorf("Error sending help message: %v", err)
		}
		return
	}

	err := c.Writef("PRIVMSG %s :Available commands (your level: %s):",
		replyTarget, userlevels.LevelName(userLevel))
	if err != nil {
		logger.Errorf("Error sending help message: %v", err)
		return
	}

	const maxCmdsPerMsg = 5
	for i := 0; i < len(availableCommands); i += maxCmdsPerMsg {
		end := i + maxCmdsPerMsg
		if end > len(availableCommands) {
			end = len(availableCommands)
		}

		chunk := availableCommands[i:end]
		message := strings.Join(chunk, " | ")

		err := c.Writef("PRIVMSG %s :%s", replyTarget, message)
		if err != nil {
			logger.Errorf("Error sending help message: %v", err)
			return
		}
	}
}
