package api

import (
	"ircbot/internal/userlevels"
)

// UserLevel represents the permission level for a user
type UserLevel int

const (
	// Ignored is a user that the bot will completely ignore
	Ignored UserLevel = iota
	// BadBoy is a restricted user with limited privileges
	BadBoy
	// Regular is a normal user with standard privileges
	Regular
	// Admin is a user with administrative privileges
	Admin
	// Owner is the bot owner with full privileges
	Owner
)

// GetUserLevel returns the level name for a user level
func GetUserLevelName(level UserLevel) string {
	switch level {
	case Ignored:
		return "Ignored"
	case BadBoy:
		return "BadBoy"
	case Regular:
		return "Regular"
	case Admin:
		return "Admin"
	case Owner:
		return "Owner"
	default:
		return "Unknown"
	}
}

// CheckUserPermission checks if a user with the given hostmask has at least the required permission level
func CheckUserPermission(hostmask string, required UserLevel) bool {
	return userlevels.HasPermission(hostmask, userlevels.UserLevel(required))
}

// GetUserLevelByHostmask returns the user level for a given hostmask
func GetUserLevelByHostmask(hostmask string) UserLevel {
	return UserLevel(userlevels.GetUserLevelByHostmask(hostmask))
}

// IsUserAdmin checks if a user is an admin
func IsUserAdmin(hostmask string) bool {
	return CheckUserPermission(hostmask, Admin)
}

// IsUserOwner checks if a user is the bot owner
func IsUserOwner(hostmask string) bool {
	return userlevels.IsVerifiedOwner(hostmask)
}

// IsKnownHostmask checks if the hostmask is registered in the system
func IsKnownHostmask(hostmask string) bool {
	// If the hostmask has a level higher than Regular, it's a known hostmask
	return GetUserLevelByHostmask(hostmask) > Regular
}