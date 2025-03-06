package commands

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/logger"
	"ircbot/internal/plugin"
	"ircbot/internal/userlevels"
)

func listPlugins(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	pluginDir := "./plugins"
	files, err := os.ReadDir(pluginDir)
	if err != nil {
		c.Writef("%s %s :Error listing plugins: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	loaded := plugin.GetPlugins()
	loadedMap := make(map[string]string)
	for _, p := range loaded {
		loadedMap[p.Name] = p.Version
	}

	type pluginStatus struct {
		name    string
		loaded  bool
		version string
	}

	basePluginMap := make(map[string]pluginStatus)

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".so" {
			filename := file.Name()

			baseName := filename

			versionMatch := regexp.MustCompile(`(.+)_v[0-9.]+\.so`).FindStringSubmatch(filename)
			if len(versionMatch) > 1 {
				baseName = versionMatch[1] + ".so"
			}

			pluginName := strings.TrimSuffix(baseName, ".so")

			if _, exists := basePluginMap[pluginName]; exists {
				continue
			}

			version, loaded := loadedMap[pluginName]
			if !loaded {
				version = "not loaded"
			}

			basePluginMap[pluginName] = pluginStatus{
				name:    pluginName,
				loaded:  loaded,
				version: version,
			}
		}
	}

	var plugins []pluginStatus
	for _, status := range basePluginMap {
		plugins = append(plugins, status)
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].name < plugins[j].name
	})

	loadedCount := len(loaded)
	totalCount := len(plugins)
	err = c.Writef("%s %s :Plugins (%d/%d loaded):", internal.CMD_PRIVMSG, replyTarget, loadedCount, totalCount)
	if err != nil {
		logger.Errorf("Error sending plugin list: %v", err)
		return
	}

	const maxPluginsPerMsg = 5
	for i := 0; i < len(plugins); i += maxPluginsPerMsg {
		end := i + maxPluginsPerMsg
		if end > len(plugins) {
			end = len(plugins)
		}

		var chunk []string
		for _, p := range plugins[i:end] {
			status := "[✓]"
			if !p.loaded {
				status = "[ ]"
			}
			if p.loaded {
				chunk = append(chunk, fmt.Sprintf("%s %s (v%s)", status, p.name, p.version))
			} else {
				chunk = append(chunk, fmt.Sprintf("%s %s", status, p.name))
			}
		}

		message := strings.Join(chunk, " | ")
		err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, message)
		if err != nil {
			logger.Errorf("Error sending plugin list: %v", err)
			return
		}
	}
}

func loadPluginCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	if len(args) < 1 {
		c.Writef("%s %s :Usage: !load <pluginName>", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	pluginName := args[0]
	pluginName = strings.TrimSuffix(pluginName, ".so")

	pluginPath := filepath.Join("./plugins", pluginName+".so")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		c.Writef("%s %s :Plugin file %s.so not found. Available plugins:", internal.CMD_PRIVMSG, replyTarget, pluginName)

		files, err := os.ReadDir("./plugins")
		if err == nil {
			var availablePlugins []string
			for _, file := range files {
				if filepath.Ext(file.Name()) == ".so" {
					if !strings.Contains(file.Name(), "_v") {
						availablePlugins = append(availablePlugins, strings.TrimSuffix(file.Name(), ".so"))
					}
				}
			}
			if len(availablePlugins) > 0 {
				c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, strings.Join(availablePlugins, ", "))
			} else {
				c.Writef("%s %s :No plugins found in ./plugins directory", internal.CMD_PRIVMSG, replyTarget)
			}
		}
		return
	}

	plugins := plugin.GetPlugins()
	for _, p := range plugins {
		if p.Name == pluginName {
			version := p.Version
			c.Writef("%s %s :Plugin %s version %s is already loaded", internal.CMD_PRIVMSG, replyTarget, pluginName, version)
			return
		}
	}

	err := plugin.LoadPlugin(pluginPath)
	if err != nil {
		logger.Errorf("Error loading plugin %s: %v", pluginName, err)
		c.Writef("%s %s :Error loading plugin %s: %v", internal.CMD_PRIVMSG, replyTarget, pluginName, err)
	} else {
		version, _ := plugin.GetPluginVersion(pluginName)
		logger.Successf("Plugin %s version %s loaded successfully", pluginName, version)
		c.Writef("%s %s :Plugin %s version %s loaded successfully", internal.CMD_PRIVMSG, replyTarget, pluginName, version)
	}
}

func unloadPluginCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	if len(args) < 1 {
		c.Writef("%s %s :Usage: !unload <pluginName>", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	pluginName := args[0]

	version, loaded := plugin.GetPluginVersion(pluginName)
	if !loaded {
		c.Writef("%s %s :Plugin %s is not currently loaded. Loaded plugins:", internal.CMD_PRIVMSG, replyTarget, pluginName)

		plugins := plugin.GetPlugins()
		var loadedNames []string
		for _, p := range plugins {
			loadedNames = append(loadedNames, fmt.Sprintf("%s (v%s)", p.Name, p.Version))
		}

		if len(loadedNames) > 0 {
			c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, strings.Join(loadedNames, ", "))
		} else {
			c.Writef("%s %s :No plugins currently loaded", internal.CMD_PRIVMSG, replyTarget)
		}
		return
	}

	err := plugin.UnloadPlugin(pluginName)
	if err != nil {
		logger.Errorf("Error unloading plugin %s: %v", pluginName, err)
		c.Writef("%s %s :Error unloading plugin %s: %v", internal.CMD_PRIVMSG, replyTarget, pluginName, err)
	} else {
		logger.Successf("Plugin %s version %s unloaded successfully", pluginName, version)
		c.Writef("%s %s :Plugin %s version %s unloaded successfully", internal.CMD_PRIVMSG, replyTarget, pluginName, version)
	}
}

func reloadPluginsCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	count, err := plugin.ReloadPluginsFromDir("./plugins")
	if err != nil {
		logger.Errorf("Failed to reload plugins: %v", err)
		c.Writef("%s %s :Plugin reload failed: %v", internal.CMD_PRIVMSG, replyTarget, err)
	} else {
		logger.Successf("Plugins reloaded successfully: %d plugins loaded", count)
		c.Writef("%s %s :Plugins reloaded successfully: %d plugins loaded", internal.CMD_PRIVMSG, replyTarget, count)
	}
}

func loadOnlinePluginCmd(c *irc.Client, m *irc.Message, args []string) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	hostmask := m.Prefix.String()
	if !userlevels.HasPermission(hostmask, userlevels.Admin) {
		c.Writef("%s %s :You need admin privileges to use this command.", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	if len(args) < 1 {
		c.Writef("%s %s :Usage: !load-online <plugin URL>", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	pluginURL := args[0]

	if !strings.HasPrefix(pluginURL, "http://") && !strings.HasPrefix(pluginURL, "https://") {
		c.Writef("%s %s :Invalid URL. Must start with http:// or https://", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	urlPath := strings.Split(pluginURL, "/")
	fileName := urlPath[len(urlPath)-1]
	pluginName := strings.TrimSuffix(fileName, ".go")

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("mbot_plugin_%d", time.Now().UnixNano()))
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		logger.Errorf("Error creating temp directory: %v", err)
		c.Writef("%s %s :Error creating temp directory: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			logger.Warnf("Error removing temp directory: %v", err)
		}
	}(tempDir)

	tempFilePath := filepath.Join(tempDir, fileName)

	c.Writef("%s %s :Downloading plugin from %s...", internal.CMD_PRIVMSG, replyTarget, pluginURL)

	resp, err := http.Get(pluginURL)
	if err != nil {
		logger.Errorf("Error downloading plugin: %v", err)
		c.Writef("%s %s :Error downloading plugin: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Error downloading plugin: HTTP %d", resp.StatusCode)
		c.Writef("%s %s :Error downloading plugin: HTTP %d", internal.CMD_PRIVMSG, replyTarget, resp.StatusCode)
		return
	}

	out, err := os.Create(tempFilePath)
	if err != nil {
		logger.Errorf("Error creating temp file: %v", err)
		c.Writef("%s %s :Error creating temp file: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		logger.Errorf("Error writing plugin source: %v", err)
		c.Writef("%s %s :Error writing plugin source: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	fileContent, err := os.ReadFile(tempFilePath)
	if err != nil {
		logger.Errorf("Error reading plugin source: %v", err)
		c.Writef("%s %s :Error reading plugin source: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	hasBuildTag := strings.Contains(string(fileContent), "//go:build")

	if !hasBuildTag {
		buildTagContent := fmt.Sprintf("//go:build %s\n// +build %s\n\n%s",
			pluginName, pluginName, string(fileContent))

		err = os.WriteFile(tempFilePath, []byte(buildTagContent), 0644)
		if err != nil {
			logger.Errorf("Error adding build tags: %v", err)
			c.Writef("%s %s :Error adding build tags: %v", internal.CMD_PRIVMSG, replyTarget, err)
			return
		}
	}

	c.Writef("%s %s :Compiling plugin %s...", internal.CMD_PRIVMSG, replyTarget, pluginName)

	err = os.MkdirAll("./plugins", 0755)
	if err != nil {
		logger.Errorf("Error creating plugins directory: %v", err)
		c.Writef("%s %s :Error creating plugins directory: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	fileContent = bytes.TrimSpace(fileContent)
	version := "1.0.0"
	versionRegex := `func\s+\(\w+\s+\*?\w+\)\s+Version\(\)\s+string\s+{\s*return\s+"([^"]+)"`
	re := regexp.MustCompile(versionRegex)
	matches := re.FindStringSubmatch(string(fileContent))
	if len(matches) > 1 {
		version = matches[1]
	}

	versionedFilename := fmt.Sprintf("%s_v%s.so", pluginName, version)
	outputPath := filepath.Join("./plugins", versionedFilename)

	standardPath := filepath.Join("./plugins", pluginName+".so")

	cmd := exec.Command("go", "build", "-tags", pluginName, "-buildmode=plugin", "-o", outputPath, tempFilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error compiling plugin: %v\n%s", err, output)
		c.Writef("%s %s :Error compiling plugin: %v\n%s", internal.CMD_PRIVMSG, replyTarget, err, limitOutput(string(output), 100))
		return
	}

	if _, err := os.Stat(standardPath); err == nil {
		os.Remove(standardPath)
	}
	err = os.Symlink(versionedFilename, standardPath)
	if err != nil {
		logger.Warnf("Error creating symlink for plugin %s: %v", pluginName, err)
	}

	c.Writef("%s %s :Loading plugin %s version %s...", internal.CMD_PRIVMSG, replyTarget, pluginName, version)

	err = plugin.LoadPlugin(outputPath)
	if err != nil {
		logger.Errorf("Error loading plugin %s: %v", pluginName, err)
		c.Writef("%s %s :Error loading plugin %s: %v", internal.CMD_PRIVMSG, replyTarget, pluginName, err)
		return
	}

	err = os.MkdirAll("./plugins_src", 0755)
	if err != nil {
		logger.Warnf("Error creating plugins_src directory: %v", err)
	} else {
		srcPath := filepath.Join("./plugins_src", fileName)
		err = copyFile(tempFilePath, srcPath)
		if err != nil {
			logger.Warnf("Error saving plugin source: %v", err)
		}
	}

	logger.Successf("Plugin %s version %s loaded successfully from URL %s", pluginName, version, pluginURL)
	c.Writef("%s %s :Plugin %s version %s loaded successfully!", internal.CMD_PRIVMSG, replyTarget, pluginName, version)
}

func limitOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "..."
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
