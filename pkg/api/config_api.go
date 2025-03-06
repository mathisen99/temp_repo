package api

import (
	"ircbot/internal/config"
	"path/filepath"
	"strings"
)

// BotConfig provides read-only access to bot configuration
type BotConfig struct {
	ServerAddress string
	BotNick       string
	BotUsername   string
	BotRealName   string
	Channels      []string
}

// GetBotConfig returns the bot's current configuration
func GetBotConfig() (*BotConfig, error) {
	cfg, err := config.LoadConfig("./data/config.toml")
	if err != nil {
		return nil, err
	}
	
	return &BotConfig{
		ServerAddress: cfg.Server,
		BotNick:       cfg.Nick,
		BotUsername:   cfg.User,
		BotRealName:   cfg.RealName,
		Channels:      cfg.Channels,
	}, nil
}

// GetServerName returns just the server hostname without the port
func GetServerName() (string, error) {
	cfg, err := GetBotConfig()
	if err != nil {
		return "", err
	}
	
	// Extract just the hostname from server:port
	parts := strings.Split(cfg.ServerAddress, ":")
	if len(parts) > 0 {
		return parts[0], nil
	}
	
	return cfg.ServerAddress, nil
}

// IsFirstRun returns true if this is the first run of the bot
func IsFirstRun() bool {
	return config.IsFirstRun()
}

// GetPluginDataDir returns a directory path where a plugin can store its data
// Each plugin gets a unique directory based on its name
func GetPluginDataDir(pluginName string) string {
	// Sanitize plugin name to be safe for filesystem
	safeName := strings.ReplaceAll(pluginName, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	
	return filepath.Join("./data/plugins", safeName)
}

// SavePluginConfig saves plugin-specific configuration to a TOML file
// It returns a function that can be used to save the config
func GetPluginConfigHelper(pluginName string) *PluginConfigHelper {
	return &PluginConfigHelper{
		PluginName: pluginName,
		ConfigPath: filepath.Join(GetPluginDataDir(pluginName), "config.toml"),
	}
}

// PluginConfigHelper helps plugins store and retrieve configuration
type PluginConfigHelper struct {
	PluginName string
	ConfigPath string
}