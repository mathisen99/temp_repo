package userlevels

import (
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"os"
	"strings"
	"sync"
)

// UserLevel represents the permission level for a user.
type UserLevel int

const (
	Ignored UserLevel = iota // Completely ignored by the bot
	BadBoy
	Regular
	Admin
	Owner
)

var levelNames = map[UserLevel]string{
	Ignored: "Ignored",
	BadBoy:  "BadBoy",
	Regular: "Regular",
	Admin:   "Admin",
	Owner:   "Owner",
}

// LevelName returns the string name of a UserLevel
func LevelName(level UserLevel) string {
	name, ok := levelNames[level]
	if !ok {
		return "Unknown"
	}
	return name
}

// UserHostmask represents a user's hostmask and their permission level
type UserHostmask struct {
	Hostmask string
	Level    UserLevel
}

var (
	users = make(map[string]UserLevel)
	hostmaskUsers = make(map[string]UserLevel)
	mu            sync.RWMutex
)

// SetUserLevel assigns the given level to the specified user (legacy method)
func SetUserLevel(nick string, level UserLevel) {
	mu.Lock()
	defer mu.Unlock()
	users[nick] = level

	wildcardHostmask := nick + "!*@*"
	hostmaskUsers[wildcardHostmask] = level
}

// SetUserLevelByHostmask assigns the given level to the specified hostmask
func SetUserLevelByHostmask(hostmask string, level UserLevel) {
	mu.Lock()
	defer mu.Unlock()
	hostmaskUsers[hostmask] = level

	parts := strings.Split(hostmask, "!")
	if len(parts) > 0 {
		nick := parts[0]
		users[nick] = level
	}
}

// GetUserLevel returns the user's level by nickname (for backward compatibility)
func GetUserLevel(nick string) UserLevel {
	mu.RLock()
	defer mu.RUnlock()

	for storedNick, level := range users {
		if strings.EqualFold(storedNick, nick) {
			return level
		}
	}
	return Regular
}

// GetUserLevelByHostmask returns the user's level by their full hostmask
func GetUserLevelByHostmask(hostmask string) UserLevel {
	mu.RLock()
	defer mu.RUnlock()

	if level, ok := hostmaskUsers[hostmask]; ok {
		return level
	}

	settings, err := config.LoadSettings()
	if err == nil && settings.OwnerVerified && settings.OwnerHostmask != "" {
		if hostmask == settings.OwnerHostmask {
			return Owner
		}
	}

	parts := strings.Split(hostmask, "!")
	if len(parts) > 0 {
		nick := parts[0]
		if level, ok := users[nick]; ok {
			return level
		}
	}

	return Regular
}

// IsVerifiedOwner checks if a user with the given hostmask is the verified owner
func IsVerifiedOwner(hostmask string) bool {
	settings, err := config.LoadSettings()
	if err != nil || !settings.OwnerVerified || settings.OwnerHostmask == "" {
		mu.RLock()
		defer mu.RUnlock()

		parts := strings.Split(hostmask, "!")
		if len(parts) < 1 {
			return false
		}
		nick := parts[0]

		found := false
		for storedNick, level := range users {
			if strings.EqualFold(storedNick, nick) {
				found = true
				if level == Owner {
					return true
				}
			}
		}
		if !found {
			return false
		}
		return false
	}

	return hostmask == settings.OwnerHostmask
}

// HasPermission checks whether a user has at least the required permission level.
func HasPermission(hostmask string, required UserLevel) bool {
	userLevel := GetUserLevelByHostmask(hostmask)
	
	// Users with Ignored level have no permissions whatsoever
	if userLevel == Ignored {
		return false
	}

	if required == Owner {
		return IsVerifiedOwner(hostmask)
	}

	return userLevel >= required
}

// GetAllHostmasks returns a list of all registered hostmasks and their levels
func GetAllHostmasks() map[string]UserLevel {
	mu.RLock()
	defer mu.RUnlock()

	result := make(map[string]UserLevel)

	for hostmask, level := range hostmaskUsers {
		result[hostmask] = level
	}

	settings, err := config.LoadSettings()
	if err == nil && settings.OwnerVerified && settings.OwnerHostmask != "" {
		result[settings.OwnerHostmask] = Owner
	}

	return result
}

// LoadHostmasks loads hostmasks from settings
func LoadHostmasks() {
	settings, err := config.LoadSettings()
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Errorf("Failed to load settings: %v", err)
		}
		return
	}

	if !settings.HostmaskLevels {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	hostmaskUsers = make(map[string]UserLevel)

	for _, entry := range settings.Hostmasks {
		hostmaskUsers[entry.Hostmask] = UserLevel(entry.Level)

		parts := strings.Split(entry.Hostmask, "!")
		if len(parts) > 0 {
			nick := parts[0]
			users[nick] = UserLevel(entry.Level)
		}
	}

	if len(hostmaskUsers) > 0 {
		logger.Infof("Loaded %d hostmasks from settings", len(hostmaskUsers))
		for hostmask, level := range hostmaskUsers {
			logger.Infof("Loaded user level: %s = %s", hostmask, LevelName(level))
		}
	}
}

// SaveHostmasks saves the current hostmask levels to settings
func SaveHostmasks() error {
	mu.RLock()
	tmpHostmaskUsers := make(map[string]UserLevel)
	for hostmask, level := range hostmaskUsers {
		tmpHostmaskUsers[hostmask] = level
	}
	mu.RUnlock()

	var hostmasks []config.HostmaskEntry
	for hostmask, level := range tmpHostmaskUsers {
		hostmasks = append(hostmasks, config.HostmaskEntry{
			Hostmask: hostmask,
			Level:    int(level),
		})
	}

	settings, err := config.LoadSettings()
	if err != nil {
		settings = &config.Settings{
			OwnerVerified:  false,
			Hostmasks:      hostmasks,
			HostmaskLevels: true,
		}
	} else {
		settings.Hostmasks = hostmasks
		settings.HostmaskLevels = true
	}

	return config.SaveSettings(settings)
}
