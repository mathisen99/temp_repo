package bot

import (
	"gopkg.in/irc.v4"
	"ircbot/internal/commands"
	"ircbot/internal/config"
	"ircbot/internal/handlers"
	"ircbot/internal/userlevels"
	"net"
)

// SetupClient initializes a new IRC client with the provided connection and configuration.
func SetupClient(conn net.Conn, cfg *config.Config) *irc.Client {
	userlevels.LoadHostmasks()
	
	// Set the config global for command system
	commands.BotConfig = cfg
	
	clientConfig := irc.ClientConfig{
		Nick: cfg.Nick,
		User: cfg.User,
		Name: cfg.RealName,
		Handler: irc.HandlerFunc(func(c *irc.Client, m *irc.Message) {
			handlers.HandleMessages(c, m, cfg.Password, cfg.Channels)
		}),
	}
	return irc.NewClient(conn, clientConfig)
}
