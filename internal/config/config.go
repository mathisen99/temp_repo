package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"strings"
)

type ChannelConfig struct {
	EnabledCommands []string          `toml:"enabled_commands"`
	DisabledCommands []string         `toml:"disabled_commands"`
	Settings map[string]interface{}   `toml:"settings"`
}

type Config struct {
	Server   string   `toml:"server"`
	Nick     string   `toml:"nick"`
	User     string   `toml:"user"`
	RealName string   `toml:"real_name"`
	Password string   `toml:"password"`
	Channels []string `toml:"channels"`
	ChannelSettings map[string]ChannelConfig `toml:"channel_settings"`
}

type HostmaskEntry struct {
	Hostmask string `toml:"hostmask"`
	Level    int    `toml:"level"`
}

type Settings struct {
	OwnerVerified  bool            `toml:"owner_verified"`
	OwnerPasshash  string          `toml:"owner_passhash"`
	OwnerHostmask  string          `toml:"owner_hostmask"`
	HostmaskLevels bool            `toml:"hostmask_levels"`
	Hostmasks      []HostmaskEntry `toml:"hostmasks"`
}

// ValidateConfig checks if all required configuration fields are properly set
func ValidateConfig(cfg *Config) error {
	var missingFields []string

	if cfg.Server == "" {
		missingFields = append(missingFields, "server")
	}
	if cfg.Nick == "" {
		missingFields = append(missingFields, "nick")
	}
	if cfg.User == "" {
		missingFields = append(missingFields, "user")
	}
	if cfg.RealName == "" {
		missingFields = append(missingFields, "real_name")
	}

	if cfg.Server != "" && !strings.Contains(cfg.Server, ":") {
		return fmt.Errorf("server address does not contain a port (format should be host:port)")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required configuration fields: %s", strings.Join(missingFields, ", "))
	}

	return nil
}

func LoadConfig(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file %s: %w", path, err)
	}

	if err := ValidateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetSettingsPath returns the path for the settings file
func GetSettingsPath() string {
	settingsPath := os.Getenv("SETTINGS_PATH")
	if settingsPath == "" {
		settingsPath = "./data/settings.toml"
	}
	return settingsPath
}

// IsFirstRun checks if this is the first run
func IsFirstRun() bool {
	settingsPath := GetSettingsPath()

	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return true
	}

	settings, err := LoadSettings()
	if err != nil {
		return true
	}

	return !settings.OwnerVerified
}

// LoadSettings loads the settings from the settings file
func LoadSettings() (*Settings, error) {
	settingsPath := GetSettingsPath()

	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return &Settings{OwnerVerified: false}, nil
	}

	var settings Settings
	if _, err := toml.DecodeFile(settingsPath, &settings); err != nil {
		return nil, fmt.Errorf("failed to decode settings file %s: %w", settingsPath, err)
	}

	return &settings, nil
}

// SaveSettings saves the settings to the settings file
func SaveSettings(settings *Settings) error {
	settingsPath := GetSettingsPath()

	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for settings file: %w", err)
	}

	file, err := os.Create(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to create settings file: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close settings file: %v\n", err)
		}
	}(file)

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(settings); err != nil {
		return fmt.Errorf("failed to encode settings: %w", err)
	}

	return nil
}

// GetChannelConfig returns the config for a specific channel or a default config if not found
func GetChannelConfig(cfg *Config, channel string) ChannelConfig {
	if cfg.ChannelSettings == nil {
		cfg.ChannelSettings = make(map[string]ChannelConfig)
	}
	
	channelCfg, exists := cfg.ChannelSettings[channel]
	if !exists {
		// Return default empty config
		return ChannelConfig{
			EnabledCommands: []string{},
			DisabledCommands: []string{},
			Settings: make(map[string]interface{}),
		}
	}
	
	return channelCfg
}

// IsCommandEnabledForChannel checks if a command is enabled for a specific channel
func IsCommandEnabledForChannel(cfg *Config, channel string, commandName string) bool {
	channelCfg := GetChannelConfig(cfg, channel)
	
	// Check if explicitly disabled
	for _, cmd := range channelCfg.DisabledCommands {
		if cmd == commandName {
			return false
		}
	}
	
	// If we have specific enabled commands and command isn't in it, it's disabled
	if len(channelCfg.EnabledCommands) > 0 {
		for _, cmd := range channelCfg.EnabledCommands {
			if cmd == commandName {
				return true
			}
		}
		return false
	}
	
	// Default to enabled
	return true
}

// GetChannelSetting gets a channel-specific setting with a default fallback
func GetChannelSetting(cfg *Config, channel string, key string, defaultValue interface{}) interface{} {
	channelCfg := GetChannelConfig(cfg, channel)
	
	value, exists := channelCfg.Settings[key]
	if !exists {
		return defaultValue
	}
	
	return value
}

// UpdateChannelSetting updates a channel-specific setting
func UpdateChannelSetting(cfg *Config, channel string, key string, value interface{}) {
	if cfg.ChannelSettings == nil {
		cfg.ChannelSettings = make(map[string]ChannelConfig)
	}
	
	channelCfg, exists := cfg.ChannelSettings[channel]
	if !exists {
		channelCfg = ChannelConfig{
			EnabledCommands: []string{},
			DisabledCommands: []string{},
			Settings: make(map[string]interface{}),
		}
	}
	
	if channelCfg.Settings == nil {
		channelCfg.Settings = make(map[string]interface{})
	}
	
	channelCfg.Settings[key] = value
	cfg.ChannelSettings[channel] = channelCfg
}

// GetChannelSettingsPath returns the path for the channel settings file
func GetChannelSettingsPath() string {
	settingsPath := os.Getenv("CHANNEL_SETTINGS_PATH")
	if settingsPath == "" {
		settingsPath = "./data/channel_settings.toml"
	}
	return settingsPath
}

// SaveChannelSettings saves the channel settings to a dedicated file
func SaveChannelSettings(cfg *Config) error {
	settingsPath := GetChannelSettingsPath()

	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for channel settings file: %w", err)
	}

	file, err := os.Create(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to create channel settings file: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close channel settings file: %v\n", err)
		}
	}(file)

	// Create a temporary structure to save just the channel settings
	channelSettings := struct {
		ChannelSettings map[string]ChannelConfig `toml:"channel_settings"`
	}{
		ChannelSettings: cfg.ChannelSettings,
	}

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(channelSettings); err != nil {
		return fmt.Errorf("failed to encode channel settings: %w", err)
	}

	return nil
}

// LoadChannelSettings loads the channel settings from the settings file
func LoadChannelSettings(cfg *Config) error {
	settingsPath := GetChannelSettingsPath()

	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		// No settings file exists yet
		cfg.ChannelSettings = make(map[string]ChannelConfig)
		return nil
	}

	var channelSettings struct {
		ChannelSettings map[string]ChannelConfig `toml:"channel_settings"`
	}
	
	if _, err := toml.DecodeFile(settingsPath, &channelSettings); err != nil {
		return fmt.Errorf("failed to decode channel settings file %s: %w", settingsPath, err)
	}

	cfg.ChannelSettings = channelSettings.ChannelSettings
	return nil
}
