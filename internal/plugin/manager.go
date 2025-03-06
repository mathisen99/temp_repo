package plugin

import (
	"fmt"
	"gopkg.in/irc.v4"
	"ircbot/internal/logger"
	"os"
	"path/filepath"
	goPlugin "plugin"
	"strings"
	"sync"
	"time"
)

type pluginInfo struct {
	plugin        Plugin
	version       string
	loadTimestamp time.Time
	filePath      string
}

type manager struct {
	plugins     map[string]pluginInfo
	pluginPaths map[string]string
	mu          sync.Mutex
}

var mgr = &manager{
	plugins:     make(map[string]pluginInfo),
	pluginPaths: make(map[string]string),
}

var knownLegacyCommands = map[string][]string{
	"SamplePlugin": {"test"},
	"EchoPlugin": {"echo"},
}

// LoadPlugin loads a single plugin from the given .so file.
func LoadPlugin(path string) error {
	p, err := goPlugin.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open plugin %s: %w", path, err)
	}

	symPlugin, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("failed to lookup Plugin symbol in %s: %w", path, err)
	}

	plug, ok := symPlugin.(Plugin)
	if !ok {
		return fmt.Errorf("invalid plugin type in %s", path)
	}

	pluginName := plug.Name()
	pluginVersion := plug.Version()

	mgr.mu.Lock()
	existingInfo, exists := mgr.plugins[pluginName]
	
	if exists {
		if existingInfo.version == pluginVersion {
			mgr.mu.Unlock()
			logger.Infof("Plugin %s version %s is already loaded", pluginName, pluginVersion)
			return nil
		}
		
		if unloader, ok := existingInfo.plugin.(Unloader); ok {
			if err := unloader.OnUnload(); err != nil {
				logger.Errorf("OnUnload error for plugin %s: %v", pluginName, err)
			}
		}
		logger.Infof("Unloaded previous version of plugin %s (was %s, loading %s)", 
			pluginName, existingInfo.version, pluginVersion)
		
		delete(mgr.pluginPaths, pluginName)
	}
	mgr.mu.Unlock()

	if err := plug.OnLoad(); err != nil {
		return fmt.Errorf("plugin %s OnLoad error: %w", pluginName, err)
	}

	info := pluginInfo{
		plugin:        plug,
		version:       pluginVersion,
		loadTimestamp: time.Now(),
		filePath:      path,
	}

	mgr.mu.Lock()
	mgr.plugins[pluginName] = info
	mgr.pluginPaths[pluginName] = path
	mgr.mu.Unlock()

	if cmdHandler, ok := plug.(CommandHandler); ok {
		commands := cmdHandler.GetCommands()
		if len(commands) > 0 {
			logger.Infof("Plugin %s version %s registered commands: %s", 
				pluginName, pluginVersion, strings.Join(commands, ", "))
		}
	}

	logger.Infof("Plugin %s version %s loaded from %s", pluginName, pluginVersion, path)
	return nil
}

// LoadPluginsFromDir scans a directory for .so files and loads them.
// Returns the number of successfully loaded plugins and any error encountered during directory reading.
func LoadPluginsFromDir(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read plugin directory %s: %w", dir, err)
	}
	
	loadedCount := 0
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".so" {
			pluginPath := filepath.Join(dir, file.Name())
			err = LoadPlugin(pluginPath)
			if err != nil {
				logger.Errorf("Error loading plugin %s: %v", file.Name(), err)
			} else {
				loadedCount++
			}
		}
	}
	return loadedCount, nil
}

// GetPlugins returns a slice of the currently loaded plugin details.
func GetPlugins() []struct {
	Name      string
	Version   string
	LoadTime  time.Time
	FilePath  string
} {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	
	pluginDetails := make([]struct {
		Name      string
		Version   string
		LoadTime  time.Time
		FilePath  string
	}, 0, len(mgr.plugins))
	
	for name, info := range mgr.plugins {
		pluginDetails = append(pluginDetails, struct {
			Name      string
			Version   string
			LoadTime  time.Time
			FilePath  string
		}{
			Name:      name,
			Version:   info.version,
			LoadTime:  info.loadTimestamp,
			FilePath:  info.filePath,
		})
	}
	return pluginDetails
}

// GetPluginObjects returns a slice of the actual plugin objects.
func GetPluginObjects() []Plugin {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	plugins := make([]Plugin, 0, len(mgr.plugins))
	for _, info := range mgr.plugins {
		plugins = append(plugins, info.plugin)
	}
	return plugins
}

// Unloader is an optional interface for plugins that need a cleanup hook.
type Unloader interface {
	OnUnload() error
}

// UnloadPlugin removes a plugin from the registry.
// If the plugin implements Unloader, its OnUnload method is called.
func UnloadPlugin(name string) error {
	mgr.mu.Lock()
	info, exists := mgr.plugins[name]
	if !exists {
		mgr.mu.Unlock()
		return fmt.Errorf("plugin %s is not loaded", name)
	}
	
	if unloader, ok := info.plugin.(Unloader); ok {
		if err := unloader.OnUnload(); err != nil {
			logger.Errorf("OnUnload error for plugin %s: %v", name, err)
		}
	}
	
	delete(mgr.plugins, name)
	delete(mgr.pluginPaths, name)
	mgr.mu.Unlock()
	
	logger.Infof("Plugin %s version %s unloaded", name, info.version)
	return nil
}

// HandlePluginCommand attempts to handle a command through plugins.
// Returns true if a plugin handled the command, false otherwise.
func HandlePluginCommand(c *irc.Client, m *irc.Message, cmd string, args []string) bool {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	
	for _, info := range mgr.plugins {
		plug := info.plugin
		
		if cmdHandler, ok := plug.(CommandHandler); ok {
			for _, supportedCmd := range cmdHandler.GetCommands() {
				if supportedCmd == cmd {
					cmdHandler.HandleCommand(c, m, cmd, args)
					return true
				}
			}
		}
		
		if cmds, ok := knownLegacyCommands[plug.Name()]; ok {
			for _, supportedCmd := range cmds {
				if supportedCmd == cmd {
					return true
				}
			}
		}
	}
	
	return false
}

// GetLegacyPluginCommands returns the known commands for a legacy plugin if available.
func GetLegacyPluginCommands(pluginName string) ([]string, bool) {
	commands, ok := knownLegacyCommands[pluginName]
	return commands, ok
}

var GetPluginList func() []Plugin = GetPluginObjects

// ReloadPluginsFromDir unloads all currently loaded plugins and reloads plugins from the specified directory.
// Returns the number of successfully loaded plugins and any error encountered.
func ReloadPluginsFromDir(dir string) (int, error) {
	unloadedCount := 0
	mgr.mu.Lock()
	for name, info := range mgr.plugins {
		if unloader, ok := info.plugin.(Unloader); ok {
			if err := unloader.OnUnload(); err != nil {
				logger.Errorf("OnUnload error for plugin %s: %v", name, err)
			}
		}
		logger.Infof("Plugin %s version %s unloaded", name, info.version)
		delete(mgr.plugins, name)
		delete(mgr.pluginPaths, name)
		unloadedCount++
	}
	mgr.mu.Unlock()
	
	logger.Infof("Unloaded %d plugins", unloadedCount)
	
	return LoadPluginsFromDir(dir)
}

// GetPluginVersion returns the version of a loaded plugin.
func GetPluginVersion(name string) (string, bool) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	
	info, exists := mgr.plugins[name]
	if !exists {
		return "", false
	}
	
	return info.version, true
}
