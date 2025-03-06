package initialization

import (
	"os"

	"github.com/joho/godotenv"
	
	"ircbot/internal"
	"ircbot/internal/ai"
	"ircbot/internal/commands"
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"ircbot/internal/plugin"
	"ircbot/internal/setup"
	"ircbot/internal/userlevels"
)

func Initialize() (*config.Config, string, bool, error) {
	if err := godotenv.Load(); err != nil {
		return nil, "", false, err
	}

	if err := ai.InitializeClient(); err != nil {
		return nil, "", false, err
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = internal.DEFAULT_CONFIG_PATH
	}
	
	logger.Infof("Loading configuration from %s", configPath)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, "", false, err
	}
	
	// Load channel-specific settings
	if err := config.LoadChannelSettings(cfg); err != nil {
		logger.Warnf("Error loading channel settings: %v", err)
	} else {
		logger.Infof("Channel settings loaded successfully")
	}

	initialOwnerNick, isFirstRun := initializeOwnerAndSecurity()
	
	// Share the bot config with the commands package
	commands.BotConfig = cfg
	logger.Infof("Initialized commands.BotConfig: %v", commands.BotConfig != nil)
	
	initializeCommandSystem()
	
	initializePlugins()
	
	return cfg, initialOwnerNick, isFirstRun, nil
}

func initializeOwnerAndSecurity() (string, bool) {
	isFirstRun := config.IsFirstRun()
	
	var initialOwnerNick string
	
	if isFirstRun {
		logger.Infof("First run detected. Security setup required.")
		initialOwnerNick, isFirstRun = setup.SetupOwnerVerification()
	} else {
		settings, err := config.LoadSettings()
		if err != nil {
			logger.Errorf("Error loading settings: %v", err)
			os.Exit(1)
		}
		
		if !settings.OwnerVerified {
			logger.Warnf("Owner verification is incomplete. Running as first run.")
			initialOwnerNick, isFirstRun = setup.SetupOwnerVerification()
		} else {
			logger.Successf("Owner has been verified. Bot security is set up.")
			
			userlevels.LoadHostmasks()
		}
	}
	
	return initialOwnerNick, isFirstRun
}

func initializeCommandSystem() {
	commands.CheckPluginCommands = plugin.HandlePluginCommand
}

func initializePlugins() {
	plugin.GetPluginList = plugin.GetPluginObjects
	
	pluginsPath := os.Getenv("PLUGINS_PATH")
	if pluginsPath == "" {
		pluginsPath = internal.DEFAULT_PLUGINS_PATH
	}
	
	logger.Infof("Loading plugins from %s", pluginsPath)
	pluginsLoaded, err := plugin.LoadPluginsFromDir(pluginsPath)
	if err != nil {
		logger.Warnf("Error loading plugins: %v", err)
	} else {
		logger.Successf("Successfully loaded %d plugins from %s", pluginsLoaded, pluginsPath)
	}
}