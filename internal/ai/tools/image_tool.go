// Package tools provides a framework for creating and managing AI tools
// that can be exposed to LLM models like GPT-4.
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
)

// ImageAnalysisArgs represents the arguments for the analyzeImage tool
type ImageAnalysisArgs struct {
	ImageURL    string  `json:"imageUrl"`
	MaxTokens   int     `json:"maxTokens,omitempty"`
	Question    string  `json:"question,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
}

// ImageTool provides image analysis capabilities through OpenAI's Vision API
type ImageTool struct {
	BaseTool
	client      *http.Client
	visionModel string
}

// NewImageTool creates a new image analysis tool
func NewImageTool() *ImageTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"imageUrl": {
				Type:        jsonschema.String,
				Description: "URL of the image to analyze",
			},
			"question": {
				Type:        jsonschema.String,
				Description: "Specific question to ask about the image (default: 'What's in this image?')",
			},
			"maxTokens": {
				Type:        jsonschema.Integer,
				Description: "Maximum number of tokens for the response (default: 300)",
			},
			"temperature": {
				Type:        jsonschema.Number,
				Description: "Temperature setting for model response (default: 0.7)",
			},
		},
		Required: []string{"imageUrl"},
	}

	client := CreateHTTPClient(30 * time.Second)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Allow up to 10 redirects
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		// Update headers on redirect
		for key, val := range via[0].Header {
			if _, ok := req.Header[key]; !ok {
				req.Header[key] = val
			}
		}
		return nil
	}

	return &ImageTool{
		BaseTool: BaseTool{
			ToolName:        "analyzeImage",
			ToolDescription: "Analyze and describe the content of an image from a URL",
			ToolParameters:  params,
		},
		client:      client,
		visionModel: "gpt-4o",
	}
}

// fetchImageAsBase64 downloads an image and encodes it as base64
func (t *ImageTool) fetchImageAsBase64(imageURL string) (string, error) {
	// Create request with appropriate headers
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set user agent and other headers to appear as a normal browser
	req.Header.Set("User-Agent", GetRandomUserAgent())
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Cache-Control", "max-age=0")

	logger.Infof("Fetching image from URL: %s", imageURL)

	// Make the request
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("image server returned status: %s", resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("URL does not point to a valid image (content-type: %s)", contentType)
	}

	// Get the image data (limit to 10MB to prevent memory issues)
	imageData, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %v", err)
	}

	// Convert to base64 and return as data URL
	base64Str := base64.StdEncoding.EncodeToString(imageData)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Str), nil
}

// Execute processes the tool call with the provided arguments
func (t *ImageTool) Execute(args string) (string, error) {
	// Check if API key is available
	apiKey := GetEnvToken("OPENAI_API_KEY")
	if apiKey == "" {
		return LogAndReturnError("OpenAI API key not available", fmt.Errorf("image analysis is disabled"))
	}

	var params ImageAnalysisArgs
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return LogAndReturnError("Failed to parse image analysis args", err)
	}

	// Validate input
	imageURL := params.ImageURL
	if imageURL == "" {
		return "", fmt.Errorf("imageUrl is required")
	}

	// Set default values
	maxTokens := params.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 300
	}

	temperature := params.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}

	question := params.Question
	if question == "" {
		question = "What's in this image?"
	}

	// Add protocol if missing
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") && 
	   !strings.HasPrefix(imageURL, "data:image/") {
		imageURL = "https://" + imageURL
	}

	// Create OpenAI client
	client := openai.NewClient(apiKey)

	// Process image URL or data URL
	var imageContent string
	if strings.HasPrefix(imageURL, "data:image/") {
		imageContent = imageURL
	} else {
		// Fetch and convert to base64
		base64Image, err := t.fetchImageAsBase64(imageURL)
		if err != nil {
			return LogAndReturnError("Failed to process image", err)
		}
		imageContent = base64Image
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create API request
	request := openai.ChatCompletionRequest{
		Model:       t.visionModel,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: question,
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: imageContent,
						},
					},
				},
			},
		},
	}

	// Call the API
	resp, err := client.CreateChatCompletion(ctx, request)
	if err != nil {
		return LogAndReturnError("OpenAI API error", err)
	}

	// Check for valid response
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from image analysis model")
	}

	analysisResult := resp.Choices[0].Message.Content
	logger.Infof("Image analysis completed for URL: %s", imageURL)

	return CleanString(analysisResult), nil
}