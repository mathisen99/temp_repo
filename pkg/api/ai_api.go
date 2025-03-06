package api

import (
	"ircbot/internal/ai"
)

// IsAIAvailable checks if AI functionality is available to plugins
func IsAIAvailable() bool {
	return ai.IsInitialized()
}

// ProcessWithAI sends text to the AI system for processing and returns a response
func ProcessWithAI(message string, user string) (string, error) {
	return ai.ProcessMessage(message, "", "", user)
}

// ProcessWithAISimple is a simplified version that doesn't require user information
func ProcessWithAISimple(message string) (string, error) {
	return ai.ProcessMessage(message, "", "", "")
}

// SummarizeWithAI uses AI to summarize a longer text
func SummarizeWithAI(content string, maxLength int) (string, error) {
	return ai.GenerateSummary(content, maxLength)
}