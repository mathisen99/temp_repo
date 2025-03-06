package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"sync"
	"time"
)

const (
	memory = 64 * 1024
	iterations = 3
	parallelism = 2
	keyLength = 32
	saltLength = 16
)

// GenerateHash creates a new hash from a passphrase
func GenerateHash(passphrase string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(passphrase),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	saltB64 := base64.StdEncoding.EncodeToString(salt)
	hashB64 := base64.StdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, iterations, parallelism,
		saltB64, hashB64,
	)

	return encodedHash, nil
}

// VerifyHash checks if a passphrase matches a previously generated hash
func VerifyHash(passphrase string, encodedHash string) (bool, error) {
	var version int
	var mem, iter, par uint32
	var saltB64, hashB64 string

	_, err := fmt.Sscanf(
		encodedHash,
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		&version, &mem, &iter, &par, &saltB64, &hashB64,
	)
	if err != nil {
		return false, fmt.Errorf("failed to parse hash: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	storedHash, err := base64.StdEncoding.DecodeString(hashB64)
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	computedHash := argon2.IDKey(
		[]byte(passphrase),
		salt,
		uint32(iter),
		uint32(mem),
		uint8(par),
		uint32(keyLength),
	)

	match := subtle.ConstantTimeCompare(storedHash, computedHash) == 1
	return match, nil
}

// MessageTracker tracks message rates for spam protection
type MessageTracker struct {
	// Maps hostmask to a slice of message timestamps
	userMessages     map[string][]time.Time
	userCommandUsage map[string]map[string][]time.Time // hostmask -> command -> timestamps
	warningCounts    map[string]int                    // Track warnings issued to users
	mutex            sync.RWMutex
	// Configuration
	messageWindow        time.Duration // Time window for rate limiting
	maxMessagesPerWindow int           // Max allowed messages in window
	commandWindow        time.Duration // Time window for command rate limiting
	maxCommandsPerWindow int           // Max allowed command uses in window
	warningThreshold     int           // Warnings before auto-ignore
}

// NewMessageTracker creates a new message tracker with default settings
func NewMessageTracker() *MessageTracker {
	return &MessageTracker{
		userMessages:         make(map[string][]time.Time),
		userCommandUsage:     make(map[string]map[string][]time.Time),
		warningCounts:        make(map[string]int),
		mutex:                sync.RWMutex{},
		messageWindow:        10 * time.Second,
		maxMessagesPerWindow: 5,
		commandWindow:        30 * time.Second,
		maxCommandsPerWindow: 10,
		warningThreshold:     3,
	}
}

// Configure allows setting custom rate limiting parameters
func (m *MessageTracker) Configure(messageWindow time.Duration, maxMessages int, 
	commandWindow time.Duration, maxCommands int, warningThreshold int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.messageWindow = messageWindow
	m.maxMessagesPerWindow = maxMessages
	m.commandWindow = commandWindow
	m.maxCommandsPerWindow = maxCommands
	m.warningThreshold = warningThreshold
}

// TrackMessage records a message and checks if rate limit is exceeded
// Returns true if the user should be ignored
func (m *MessageTracker) TrackMessage(hostmask string) (bool, int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	now := time.Now()
	
	// Initialize if first message from this user
	if _, exists := m.userMessages[hostmask]; !exists {
		m.userMessages[hostmask] = []time.Time{}
	}
	
	// Add current message timestamp
	m.userMessages[hostmask] = append(m.userMessages[hostmask], now)
	
	// Remove messages outside the window
	cutoff := now.Add(-m.messageWindow)
	newMessages := []time.Time{}
	for _, t := range m.userMessages[hostmask] {
		if t.After(cutoff) {
			newMessages = append(newMessages, t)
		}
	}
	m.userMessages[hostmask] = newMessages
	
	// Check if rate limit exceeded
	messageCount := len(m.userMessages[hostmask])
	
	// Return whether the user is spamming and the number of messages in window
	return messageCount > m.maxMessagesPerWindow, messageCount
}

// TrackCommand records a command usage and checks if rate limit is exceeded
// Returns true if the command usage should be blocked
func (m *MessageTracker) TrackCommand(hostmask, commandName string) (bool, int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	now := time.Now()
	
	// Initialize maps if needed
	if _, exists := m.userCommandUsage[hostmask]; !exists {
		m.userCommandUsage[hostmask] = make(map[string][]time.Time)
	}
	if _, exists := m.userCommandUsage[hostmask][commandName]; !exists {
		m.userCommandUsage[hostmask][commandName] = []time.Time{}
	}
	
	// Add current command timestamp
	m.userCommandUsage[hostmask][commandName] = append(m.userCommandUsage[hostmask][commandName], now)
	
	// Remove commands outside the window
	cutoff := now.Add(-m.commandWindow)
	newCommands := []time.Time{}
	for _, t := range m.userCommandUsage[hostmask][commandName] {
		if t.After(cutoff) {
			newCommands = append(newCommands, t)
		}
	}
	m.userCommandUsage[hostmask][commandName] = newCommands
	
	// Check if rate limit exceeded
	commandCount := len(m.userCommandUsage[hostmask][commandName])
	
	// Return whether the command is being spammed and the count
	return commandCount > m.maxCommandsPerWindow, commandCount
}

// AddWarning increments the warning count for a user and returns true if they should be ignored
func (m *MessageTracker) AddWarning(hostmask string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.warningCounts[hostmask]++
	
	return m.warningCounts[hostmask] >= m.warningThreshold
}

// GetWarningCount returns the current warning count for a user
func (m *MessageTracker) GetWarningCount(hostmask string) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return m.warningCounts[hostmask]
}

// GetMessageWindow returns the configured message window duration
func (m *MessageTracker) GetMessageWindow() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.messageWindow
}

// GetMaxMessagesPerWindow returns the configured max messages per window
func (m *MessageTracker) GetMaxMessagesPerWindow() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.maxMessagesPerWindow
}

// GetCommandWindow returns the configured command window duration
func (m *MessageTracker) GetCommandWindow() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.commandWindow
}

// GetMaxCommandsPerWindow returns the configured max commands per window
func (m *MessageTracker) GetMaxCommandsPerWindow() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.maxCommandsPerWindow
}

// GetWarningThreshold returns the configured warning threshold
func (m *MessageTracker) GetWarningThreshold() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.warningThreshold
}

// SetMessageWindow sets the message window duration
func (m *MessageTracker) SetMessageWindow(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.messageWindow = duration
}

// SetMaxMessagesPerWindow sets the max messages per window
func (m *MessageTracker) SetMaxMessagesPerWindow(max int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.maxMessagesPerWindow = max
}

// SetCommandWindow sets the command window duration
func (m *MessageTracker) SetCommandWindow(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.commandWindow = duration
}

// SetMaxCommandsPerWindow sets the max commands per window
func (m *MessageTracker) SetMaxCommandsPerWindow(max int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.maxCommandsPerWindow = max
}

// SetWarningThreshold sets the warning threshold before auto-ignore
func (m *MessageTracker) SetWarningThreshold(threshold int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.warningThreshold = threshold
}

// ResetUser clears all tracking data for a user
func (m *MessageTracker) ResetUser(hostmask string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	delete(m.userMessages, hostmask)
	delete(m.userCommandUsage, hostmask)
	delete(m.warningCounts, hostmask)
}

// GlobalMessageTracker is the singleton instance of MessageTracker
var GlobalMessageTracker = NewMessageTracker()