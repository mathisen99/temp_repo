package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogType string

const (
	ChannelLog LogType = "CHANNEL"
	PrivateLog LogType = "PRIVATE"
)

// chatLogger manages logs for channels and private messages
type chatLogger struct {
	baseDir     string
	logFiles    map[string]*os.File
	logWriters  map[string]*os.File
	mutex       sync.Mutex
	currentDate string
}

var (
	// Singleton instance of the chat logger
	chatLog     *chatLogger
	chatLogOnce sync.Once
)

// getChatLogger returns the singleton chat logger instance
func getChatLogger() *chatLogger {
	chatLogOnce.Do(func() {
		chatLog = &chatLogger{
			baseDir:     filepath.Join("logs"),
			logFiles:    make(map[string]*os.File),
			logWriters:  make(map[string]*os.File),
			currentDate: time.Now().Format("2006-01-02"),
		}
	})
	return chatLog
}

// getLogFilePath returns the path for a log file
func (cl *chatLogger) getLogFilePath(logType LogType, name string, date string) string {
	// Sanitize name to make it safe for file system (remove special characters)
	sanitizedName := sanitizeFilename(name)
	
	// Create directory structure
	dirPath := filepath.Join(cl.baseDir, string(logType), sanitizedName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		Errorf("Failed to create directory for logs: %v", err)
		return ""
	}
	
	// Return the full log file path
	return filepath.Join(dirPath, fmt.Sprintf("%s.log", date))
}

// sanitizeFilename removes characters that are invalid in filenames
func sanitizeFilename(name string) string {
	// Simple sanitization: replace problematic characters
	// For a more comprehensive solution, this could be expanded
	safeMap := map[rune]rune{
		'/':  '-',
		'\\': '-',
		':':  '-',
		'*':  '-',
		'?':  '-',
		'"':  '\'',
		'<':  '(',
		'>':  ')',
		'|':  '-',
	}
	
	result := []rune(name)
	for i, char := range result {
		if replacement, found := safeMap[char]; found {
			result[i] = replacement
		}
	}
	
	return string(result)
}

// getLogWriter returns a file writer for the specified log
// If the date has changed, it will close the old file and open a new one
func (cl *chatLogger) getLogWriter(logType LogType, name string) *os.File {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	
	// Get current date
	currentDate := time.Now().Format("2006-01-02")
	
	// Create a key for this log
	logKey := fmt.Sprintf("%s:%s", logType, name)
	
	// Check if we need to rotate the log file (new day)
	if currentDate != cl.currentDate {
		// Close all open log files
		for key, file := range cl.logFiles {
			file.Close()
			delete(cl.logFiles, key)
			delete(cl.logWriters, key)
		}
		cl.currentDate = currentDate
	}
	
	// Check if we already have a writer
	if writer, exists := cl.logWriters[logKey]; exists {
		return writer
	}
	
	// Create new log file
	logPath := cl.getLogFilePath(logType, name, currentDate)
	if logPath == "" {
		return nil
	}
	
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		Errorf("Failed to open log file %s: %v", logPath, err)
		return nil
	}
	
	// Store file and writer
	cl.logFiles[logKey] = file
	cl.logWriters[logKey] = file
	
	return file
}

// LogChannelMessage logs a message in a channel
func LogChannelMessage(channel, sender, message string) {
	logger := getChatLogger()
	writer := logger.getLogWriter(ChannelLog, channel)
	if writer == nil {
		return
	}
	
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] <%s> %s\n", timestamp, sender, message)
	
	if _, err := writer.WriteString(logEntry); err != nil {
		Errorf("Failed to write channel log: %v", err)
	}
}

// LogChannelAction logs an action in a channel
func LogChannelAction(channel, sender, action string) {
	logger := getChatLogger()
	writer := logger.getLogWriter(ChannelLog, channel)
	if writer == nil {
		return
	}
	
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] * %s %s\n", timestamp, sender, action)
	
	if _, err := writer.WriteString(logEntry); err != nil {
		Errorf("Failed to write channel action log: %v", err)
	}
}

// LogChannelEvent logs a channel event (join, part, quit, etc.)
func LogChannelEvent(channel, event string) {
	logger := getChatLogger()
	writer := logger.getLogWriter(ChannelLog, channel)
	if writer == nil {
		return
	}
	
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s\n", timestamp, event)
	
	if _, err := writer.WriteString(logEntry); err != nil {
		Errorf("Failed to write channel event log: %v", err)
	}
}

// LogBotChannelMessage logs the bot's own messages to the channel log
func LogBotChannelMessage(channel, botNick, message string) {
	logger := getChatLogger()
	writer := logger.getLogWriter(ChannelLog, channel)
	if writer == nil {
		return
	}
	
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] <%s> %s\n", timestamp, botNick, message)
	
	if _, err := writer.WriteString(logEntry); err != nil {
		Errorf("Failed to write bot channel message log: %v", err)
	}
}

// LogPrivateMessage logs a private message
func LogPrivateMessage(nickname, direction, message string) {
	logger := getChatLogger()
	writer := logger.getLogWriter(PrivateLog, nickname)
	if writer == nil {
		return
	}
	
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s: %s\n", timestamp, direction, message)
	
	if _, err := writer.WriteString(logEntry); err != nil {
		Errorf("Failed to write private message log: %v", err)
	}
}

// CloseAllChatLogs closes all open log files
func CloseAllChatLogs() {
	if chatLog == nil {
		return
	}
	
	chatLog.mutex.Lock()
	defer chatLog.mutex.Unlock()
	
	for key, file := range chatLog.logFiles {
		file.Close()
		delete(chatLog.logFiles, key)
		delete(chatLog.logWriters, key)
	}
}