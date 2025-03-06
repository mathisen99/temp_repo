package tools

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"ircbot/internal/logger"
)

// GetEnvToken returns the first non-empty environment variable value from the provided keys
func GetEnvToken(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			logger.Debugf("Using token from environment variable: %s", key)
			return value
		}
	}
	return ""
}

// TruncateString shortens a string to the specified maximum length, adding ellipsis if needed
func TruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	
	return string([]rune(s)[:maxLen-3]) + "..."
}

// LogAndReturnError logs an error and returns it with a user-friendly message
func LogAndReturnError(context string, err error) (string, error) {
	errMsg := fmt.Sprintf("%s: %v", context, err)
	logger.Errorf(errMsg)
	return "", fmt.Errorf(errMsg)
}

var multipleNewlines = regexp.MustCompile(`\n{3,}`)

// CleanString normalizes whitespace and line endings in a string
func CleanString(s string) string {
	// Normalize all line endings to \n
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	
	// Replace sequences of 3+ newlines with just 2
	s = multipleNewlines.ReplaceAllString(s, "\n\n")
	
	return strings.TrimSpace(s)
}

// SanitizeFilename removes invalid characters from a filename
func SanitizeFilename(filename string) string {
	// Replace characters that are invalid in filenames
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename
	
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	
	return result
}