package ai

import (
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"ircbot/internal/logger"
)

func truncateContent(content string) string {
	words := strings.Fields(content)
	if len(words) <= 10 {
		return content
	}
	return strings.Join(words[:10], " ") + "..."
}

func GenerateSummary(content string, maxLength int) (string, error) {
	if !IsInitialized() {
		return "AI summarization not available", nil
	}
	
	if len(content) <= maxLength {
		return content, nil
	}
	
	if !GetConfig().EnableSummarization {
		return truncateContent(content), nil
	}
	
	ctx, cancel := CreateContext()
	defer cancel()
	
	cfg := GetConfig()
	
	resp, err := GetClient().CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model: MapModelName(cfg.Model),
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: fmt.Sprintf("Summarize the following text in %d characters or less:", maxLength),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxResponseTokens,
		},
	)
	
	if err != nil {
		logger.Errorf("Summary generation error: %v", err)
		return truncateContent(content), err
	}
	
	summary := resp.Choices[0].Message.Content
	
	if len(summary) > maxLength {
		summary = summary[:maxLength-3] + "..."
	}
	
	return summary, nil
}