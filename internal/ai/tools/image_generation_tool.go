package tools

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
)

// FluxCreateImageResponse represents the response from the Flux API when creating an image request.
type FluxCreateImageResponse struct {
	ID string `json:"id"`
}

// FluxGetResultResponse represents the response from the Flux API when polling for the result.
type FluxGetResultResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Result struct {
		Sample string `json:"sample"`
	} `json:"result"`
}

// ImageGenerationArgs represents the arguments for the generateImage tool
type ImageGenerationArgs struct {
	Prompt          string `json:"prompt"`
	Size            string `json:"size,omitempty"`
	Quality         string `json:"quality,omitempty"`
	Style           string `json:"style,omitempty"`
	Literal         bool   `json:"literal,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Enhance         bool   `json:"enhance,omitempty"`
	
	// Flux-specific options
	FluxWidth          int    `json:"flux_width,omitempty"`
	FluxHeight         int    `json:"flux_height,omitempty"`
	FluxSeed           int    `json:"flux_seed,omitempty"`
	FluxSafetyLevel    int    `json:"flux_safety_level,omitempty"` 
	FluxOutputFormat   string `json:"flux_output_format,omitempty"`
	FluxPromptUpsampling bool  `json:"flux_prompt_upsampling,omitempty"`
	ImagePrompt       string `json:"image_prompt,omitempty"` // Base64 encoded image for Flux Redux
	
	// User info for safety level determination
	UserLevel        string `json:"user_level,omitempty"`
}

// paste site response is already defined in paste_tool.go

// ImageGenerationTool provides image generation capabilities using multiple providers
type ImageGenerationTool struct {
	BaseTool
	openaiApiKey string
	bflApiKey    string
	pasteToken   string
	pasteURL     string
	client       *http.Client
	userAgents   []string

	// Constants for Flux rate limiting
	maxFluxImages    int
	fluxTimeFrame    int64 // in seconds
	fluxImageCount   int
	fluxFirstImgTime int64
}

// NewImageGenerationTool creates a new image generation tool
func NewImageGenerationTool() *ImageGenerationTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"prompt": {
				Type:        jsonschema.String,
				Description: "Text description of the image to generate",
			},
			"provider": {
				Type:        jsonschema.String,
				Description: "Image generation provider to use (flux or dalle, default: flux)",
				Enum:        []string{"flux", "dalle"},
			},
			// DALL-E specific parameters
			"size": {
				Type:        jsonschema.String,
				Description: "Size of the image for DALL-E (1024x1024, 1024x1792, or 1792x1024, default: 1024x1024)",
				Enum:        []string{"1024x1024", "1024x1792", "1792x1024"},
			},
			"quality": {
				Type:        jsonschema.String,
				Description: "Quality of the image for DALL-E (standard or hd, default: standard)",
				Enum:        []string{"standard", "hd"},
			},
			"style": {
				Type:        jsonschema.String,
				Description: "Style of the image for DALL-E (vivid or natural, default: vivid)",
				Enum:        []string{"vivid", "natural"},
			},
			"literal": {
				Type:        jsonschema.Boolean,
				Description: "For DALL-E: If true, uses prompt as-is without DALL-E's automatic enhancements (default: false)",
			},
			// Common parameters
			"enhance": {
				Type:        jsonschema.Boolean,
				Description: "For Flux: If true, enhance the prompt before generating the image (default: false)",
			},
			// User info
			"user_level": {
				Type:        jsonschema.String,
				Description: "User level for adjusting safety levels (user, admin, etc.)",
				Enum:        []string{"user", "admin", "owner"},
			},
			// Flux-specific parameters
			"flux_width": {
				Type:        jsonschema.Integer,
				Description: "Width of the Flux image in pixels (256-1440, must be multiple of 32, default: 1024)",
			},
			"flux_height": {
				Type:        jsonschema.Integer,
				Description: "Height of the Flux image in pixels (256-1440, must be multiple of 32, default: 768)",
			},
			"flux_seed": {
				Type:        jsonschema.Integer,
				Description: "Optional seed for reproducible Flux generations",
			},
			"flux_safety_level": {
				Type:        jsonschema.Integer,
				Description: "Tolerance level for moderation (0-6, 0=strictest, 6=least strict, default: 2)",
			},
			"flux_output_format": {
				Type:        jsonschema.String,
				Description: "Output format for Flux (jpeg or png, default: jpeg)",
				Enum:        []string{"jpeg", "png"},
			},
			"flux_prompt_upsampling": {
				Type:        jsonschema.Boolean,
				Description: "Whether to perform Flux prompt upsampling (default: false)",
			},
			"image_prompt": {
				Type:        jsonschema.String,
				Description: "Base64 encoded image for Flux Redux (optional)",
			},
		},
		Required: []string{"prompt"},
	}

	// Get API keys from environment
	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	bflApiKey := os.Getenv("BFL_API_KEY")

	// Try multiple ways to get the paste token - check both environment variables
	pasteToken := os.Getenv("VALID_PASTE_TOKEN")
	if pasteToken == "" {
		pasteToken = os.Getenv("PASTE_TOKEN")
		if pasteToken == "" {
			logger.Warnf("No paste token found in environment variables VALID_PASTE_TOKEN or PASTE_TOKEN")
		}
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// List of modern browser user agents to rotate through
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:124.0) Gecko/20100101 Firefox/124.0",
	}

	return &ImageGenerationTool{
		BaseTool: BaseTool{
			ToolName:        "generateImage",
			ToolDescription: "Generate an image using Flux or DALL-E 3 based on a text prompt and upload it to paste site",
			ToolParameters:  params,
		},
		openaiApiKey:     openaiApiKey,
		bflApiKey:        bflApiKey,
		pasteToken:       pasteToken,
		pasteURL:         "https://paste.mathizen.net/paste",
		client:           client,
		userAgents:       userAgents,
		maxFluxImages:    5,
		fluxTimeFrame:    6 * 60 * 60, // 6 hours in seconds
		fluxImageCount:   0,
		fluxFirstImgTime: 0,
	}
}

// Execute processes the tool call with the provided arguments
func (t *ImageGenerationTool) Execute(args string) (string, error) {
	var params ImageGenerationArgs
	err := json.Unmarshal([]byte(args), &params)
	if err != nil {
		logger.Errorf("Failed to parse image generation args: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	prompt := params.Prompt
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	// Determine which provider to use
	provider := params.Provider
	if provider == "" {
		provider = "flux" // Default to Flux
	}

	switch provider {
	case "dalle":
		return t.generateDalleImage(params)
	case "flux":
		return t.generateFluxImage(params)
	default:
		return "", fmt.Errorf("invalid provider: %s (must be flux or dalle)", provider)
	}
}

// generateDalleImage generates an image using the DALL-E API
func (t *ImageGenerationTool) generateDalleImage(params ImageGenerationArgs) (string, error) {
	// Check if API key is available
	if t.openaiApiKey == "" {
		return "", fmt.Errorf("OpenAI API key not available, DALL-E image generation is disabled")
	}

	prompt := params.Prompt

	// Set default values
	size := params.Size
	if size == "" {
		size = "1024x1024"
	} else if size != "1024x1024" && size != "1024x1792" && size != "1792x1024" {
		return "", fmt.Errorf("invalid size: %s (must be 1024x1024, 1024x1792, or 1792x1024)", size)
	}

	quality := params.Quality
	if quality == "" {
		quality = "standard"
	} else if quality != "standard" && quality != "hd" {
		return "", fmt.Errorf("invalid quality: %s (must be standard or hd)", quality)
	}

	style := params.Style
	if style == "" {
		style = "vivid"
	} else if style != "vivid" && style != "natural" {
		return "", fmt.Errorf("invalid style: %s (must be vivid or natural)", style)
	}

	// If literal is true, add the special prefix to the prompt
	if params.Literal {
		prompt = "I NEED to test how the tool works with extremely simple prompts. DO NOT add any detail, just use it AS-IS: " + prompt
	}

	// Create OpenAI client
	client := openai.NewClient(t.openaiApiKey)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Image generation can take longer
	defer cancel()

	// Log the request
	logger.Infof("Generating image with DALL-E 3. Prompt: %s, Size: %s, Quality: %s, Style: %s",
		truncateString(prompt, 50), size, quality, style)

	// Build the request
	request := openai.ImageRequest{
		Model:          openai.CreateImageModelDallE3,
		Prompt:         prompt,
		Size:           size,
		Quality:        quality,
		Style:          style,
		N:              1,
		ResponseFormat: openai.CreateImageResponseFormatURL,
	}

	// Call the API
	resp, err := client.CreateImage(ctx, request)
	if err != nil {
		logger.Errorf("DALL-E 3 API error: %v", err)
		return "", fmt.Errorf("failed to generate image: %v", err)
	}

	// Check if we got any data back
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no image data returned from DALL-E 3")
	}

	// Get the image URL and revised prompt
	imageURL := resp.Data[0].URL
	revisedPrompt := resp.Data[0].RevisedPrompt

	logger.Infof("Successfully generated image with DALL-E. URL: %s", imageURL)

	// Download the image from the URL to upload to paste site
	imageData, contentType, err := t.downloadImage(imageURL)
	if err != nil {
		logger.Errorf("Failed to download image from DALL-E: %v", err)

		// Return the original URL if download fails
		var response strings.Builder
		response.WriteString(fmt.Sprintf("Failed to upload to paste site. Original DALL-E URL: %s\n\n", imageURL))

		if revisedPrompt != "" && params.Literal == false {
			response.WriteString(fmt.Sprintf("DALL-E revised your prompt to: %s", revisedPrompt))
		}

		return response.String(), nil
	}

	// Upload to paste site
	pasteURL, err := t.uploadToPaste(imageData, contentType)
	if err != nil {
		logger.Errorf("Failed to upload to paste site: %v", err)

		// Return the original URL if upload fails
		var response strings.Builder
		response.WriteString(fmt.Sprintf("Failed to upload to paste site. Original DALL-E URL: %s\n\n", imageURL))

		if revisedPrompt != "" && params.Literal == false {
			response.WriteString(fmt.Sprintf("DALL-E revised your prompt to: %s", revisedPrompt))
		}

		return response.String(), nil
	}

	logger.Infof("Successfully uploaded DALL-E image to paste site: %s", pasteURL)

	// Format the final response
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Generated image (DALL-E): %s\n\n", pasteURL))

	if revisedPrompt != "" && params.Literal == false {
		response.WriteString(fmt.Sprintf("DALL-E revised your prompt to: %s", revisedPrompt))
	}

	return response.String(), nil
}

// generateFluxImage generates an image using the Flux API
func (t *ImageGenerationTool) generateFluxImage(params ImageGenerationArgs) (string, error) {
	// Check if API key is available
	if t.bflApiKey == "" {
		return "", fmt.Errorf("BFL API key not available, Flux image generation is disabled")
	}

	// Check rate limiting
	if err := t.checkFluxRateLimit(); err != nil {
		return "Max Flux image limit reached! Try again later or use DALL-E.", err
	}

	prompt := params.Prompt
	
	// Log with appropriate details
	var logMessage strings.Builder
	logMessage.WriteString(fmt.Sprintf("Creating image with Flux. Prompt: %s", truncateString(prompt, 50)))
	
	if params.FluxWidth > 0 || params.FluxHeight > 0 {
		logMessage.WriteString(fmt.Sprintf(", Size: %dx%d", 
			valueOrDefault(params.FluxWidth, 1024), 
			valueOrDefault(params.FluxHeight, 768)))
	}
	
	if params.FluxSeed > 0 {
		logMessage.WriteString(fmt.Sprintf(", Seed: %d", params.FluxSeed))
	}
	
	if params.UserLevel == "admin" || params.UserLevel == "owner" {
		logMessage.WriteString(", Safety: minimal (admin override)")
	} else if params.FluxSafetyLevel > 0 {
		logMessage.WriteString(fmt.Sprintf(", Safety level: %d", params.FluxSafetyLevel))
	}
	
	logger.Infof(logMessage.String())

	// Enhance the prompt if requested
	if params.Enhance {
		logger.Infof("Enhancing Flux prompt...")
		enhancedPrompt, err := t.enhanceFluxPrompt(prompt)
		if err != nil {
			logger.Errorf("Error enhancing Flux prompt: %v", err)
			// Continue with original prompt if enhancement fails
		} else {
			prompt = enhancedPrompt
			logger.Infof("Enhanced Flux prompt: %s", truncateString(prompt, 50))
		}
	}

	// Generate the image
	imageURL, err := t.callFluxAPI(prompt, params)
	if err != nil {
		logger.Errorf("Flux API error: %v", err)
		return "", fmt.Errorf("failed to generate Flux image: %v", err)
	}

	// Download the image
	imageData, contentType, err := t.downloadImage(imageURL)
	if err != nil {
		logger.Errorf("Failed to download Flux image: %v", err)
		return fmt.Sprintf("Failed to download Flux image. Original URL: %s", imageURL), nil
	}

	// Upload to paste site
	pasteURL, err := t.uploadToPaste(imageData, contentType)
	if err != nil {
		logger.Errorf("Failed to upload Flux image to paste site: %v", err)
		return fmt.Sprintf("Failed to upload to paste site. Original Flux URL: %s", imageURL), nil
	}

	logger.Infof("Successfully uploaded Flux image to paste site: %s", pasteURL)

	// Create detailed response with image info
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Generated image (Flux): %s\n", pasteURL))
	
	// Add generation details
	if params.FluxWidth > 0 || params.FluxHeight > 0 {
		response.WriteString(fmt.Sprintf("Size: %dx%d", 
			valueOrDefault(params.FluxWidth, 1024), 
			valueOrDefault(params.FluxHeight, 768)))
	}
	
	if params.FluxSeed > 0 {
		response.WriteString(fmt.Sprintf(", Seed: %d", params.FluxSeed))
	}
	
	// Only show these details to admins/owners
	if params.UserLevel == "admin" || params.UserLevel == "owner" {
		if params.UserLevel == "admin" || params.UserLevel == "owner" {
			response.WriteString(fmt.Sprintf("\nSafety level: %d", 
				valueOrDefaultInt(params.FluxSafetyLevel, 6)))
		}
	}
	
	return response.String(), nil
}

// callFluxAPI sends the prompt to the Flux API and retrieves the generated image URL
func (t *ImageGenerationTool) callFluxAPI(prompt string, params ImageGenerationArgs) (string, error) {
	// Set default values for Flux parameters
	width := 1024
	if params.FluxWidth >= 256 && params.FluxWidth <= 1440 && params.FluxWidth%32 == 0 {
		width = params.FluxWidth
	}
	
	height := 768
	if params.FluxHeight >= 256 && params.FluxHeight <= 1440 && params.FluxHeight%32 == 0 {
		height = params.FluxHeight
	}
	
	promptUpsampling := false
	if params.FluxPromptUpsampling {
		promptUpsampling = true
	}
	
	// Determine safety level based on user level
	safetyTolerance := 2 // Default safety level
	if params.FluxSafetyLevel >= 0 && params.FluxSafetyLevel <= 6 {
		safetyTolerance = params.FluxSafetyLevel
	} else if params.UserLevel == "admin" || params.UserLevel == "owner" {
		// For admins and owners, use minimal safety filtering
		safetyTolerance = 6
	}
	
	// Build the request body
	requestBody := map[string]interface{}{
		"prompt":            prompt,
		"width":             width,
		"height":            height,
		"prompt_upsampling": promptUpsampling,
		"safety_tolerance":  safetyTolerance,
	}
	
	// Add optional parameters if provided
	if params.FluxSeed > 0 {
		requestBody["seed"] = params.FluxSeed
	}
	
	if params.FluxOutputFormat == "png" {
		requestBody["output_format"] = "png"
	} else {
		requestBody["output_format"] = "jpeg"
	}
	
	if params.ImagePrompt != "" {
		requestBody["image_prompt"] = params.ImagePrompt
	}
	
	reqBodyJSON, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", "https://api.bfl.ml/v1/flux-pro-1.1", io.NopCloser(bytes.NewReader(reqBodyJSON)))
	if err != nil {
		return "", err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("x-key", t.bflApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf("Error closing response body: %v", err)
		}
	}(resp.Body)

	respBody, _ := io.ReadAll(resp.Body)

	var createResp FluxCreateImageResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return "", fmt.Errorf("failed to parse Flux API response: %v", err)
	}

	if createResp.ID == "" {
		return "", fmt.Errorf("failed to get request ID from Flux API response")
	}

	logger.Infof("Polling for Flux image generation result (ID: %s)...", createResp.ID)
	for {
		time.Sleep(2 * time.Second)
		result, err := t.pollFluxResult(createResp.ID)
		if err != nil {
			return "", err
		}

		switch result.Status {
		case "Ready":
			if result.Result.Sample != "" {
				return result.Result.Sample, nil
			}
			return "", fmt.Errorf("Flux result is ready but sample is empty")
		case "Pending":
			continue
		case "Request Moderated":
			return "", fmt.Errorf("your request was moderated by Flux and could not be processed")
		case "Error":
			return "", fmt.Errorf("Flux generation failed with status: %s", result.Status)
		default:
			return "", fmt.Errorf("unhandled Flux status: %s", result.Status)
		}
	}
}

// pollFluxResult retrieves the image generation result from the Flux API
func (t *ImageGenerationTool) pollFluxResult(requestID string) (*FluxGetResultResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.bfl.ml/v1/get_result?id=%s", requestID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("x-key", t.bflApiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf("Error closing response body: %v", err)
		}
	}(resp.Body)

	respBody, _ := io.ReadAll(resp.Body)

	var result FluxGetResultResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Flux poll response: %v", err)
	}

	return &result, nil
}

// enhanceFluxPrompt improves the user's original prompt using the OpenAI API
func (t *ImageGenerationTool) enhanceFluxPrompt(originalPrompt string) (string, error) {
	// Check if OpenAI API key is available
	if t.openaiApiKey == "" {
		return originalPrompt, fmt.Errorf("OpenAI API key not available for prompt enhancement")
	}

	client := openai.NewClient(t.openaiApiKey)

	systemMessage := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
		Content: `You are a prompt enhancer for a cutting-edge image generation model called Flux. Your task is to take a given user prompt and improve it to ensure the generated image is detailed, visually rich, and matches the desired scene. 

You will enhance the prompt by:
1. Using parentheses () to increase the emphasis on key concepts.
2. Using square brackets [] to reduce emphasis on less important elements.
3. Applying curly braces {} to introduce randomness by offering alternative elements within the prompt.
4. Assigning weights using the : syntax to prioritize certain words or phrases.
5. Clarifying and expanding the prompt by adding details such as lighting, perspective, and color.
6. Keeping the user's intent intact while improving the clarity, creativity, and visual specificity of the prompt.
8. Providing creative enhancements that align with the desired style and theme.

You will only respond with the enhanced prompt, ensuring the modified version is optimized for the Flux Pro model.`,
	}

	userMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: originalPrompt,
	}

	messages := []openai.ChatCompletionMessage{
		systemMessage,
		userMessage,
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:     "gpt-4o",
			Messages:  messages,
			MaxTokens: 500,
		},
	)

	if err != nil {
		return originalPrompt, fmt.Errorf("error enhancing prompt: %v", err)
	}

	enhancedPrompt := strings.TrimSpace(resp.Choices[0].Message.Content)
	return enhancedPrompt, nil
}

// checkFluxRateLimit checks if the image generation rate limit has been reached
func (t *ImageGenerationTool) checkFluxRateLimit() error {
	// Get the current time
	currentTime := time.Now().Unix()

	// If no images have been created yet, start the time frame
	if t.fluxImageCount == 0 {
		t.fluxFirstImgTime = currentTime
		t.fluxImageCount++
		return nil
	}

	// Check if the current time exceeds the time frame
	if currentTime-t.fluxFirstImgTime >= t.fluxTimeFrame {
		// Reset counters since the time window has expired
		t.fluxFirstImgTime = currentTime
		t.fluxImageCount = 1 // Start counting with this new image
		return nil
	}

	// If the image count has reached the maximum allowed within the time frame
	if t.fluxImageCount >= t.maxFluxImages {
		return fmt.Errorf("Flux image generation rate limit reached (%d images per %d hours)",
			t.maxFluxImages, t.fluxTimeFrame/3600)
	}

	// Increment the image count for this new image
	t.fluxImageCount++
	logger.Infof("Flux image count: %d/%d", t.fluxImageCount, t.maxFluxImages)
	return nil
}

// downloadImage downloads an image from a URL
func (t *ImageGenerationTool) downloadImage(imageURL string) ([]byte, string, error) {
	// Select a random user agent
	userAgent := t.userAgents[time.Now().UnixNano()%int64(len(t.userAgents))]

	// Create request with appropriate headers
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set user agent and other headers to appear as a normal browser
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "max-age=0")

	// Log the request being made
	logger.Infof("Downloading image from URL: %s", imageURL)

	// Make the request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("image server returned status: %s", resp.Status)
	}

	// Get content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("URL does not point to a valid image (content-type: %s)", contentType)
	}

	// Read the image data
	imageData, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // Limit to 10MB
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %v", err)
	}

	return imageData, contentType, nil
}

// uploadToPaste uploads image data to the paste site
func (t *ImageGenerationTool) uploadToPaste(imageData []byte, contentType string) (string, error) {
	// Check if token is available - attempt to get from environment if missing
	pasteToken := t.pasteToken
	if pasteToken == "" {
		// Try both environment variables as a fallback
		pasteToken = os.Getenv("VALID_PASTE_TOKEN")
		if pasteToken == "" {
			pasteToken = os.Getenv("PASTE_TOKEN")
		}

		// If still no token, return error
		if pasteToken == "" {
			return "", fmt.Errorf("paste token not available in environment, image pasting is disabled")
		}

		// Log that we had to get the token directly
		logger.Infof("Using paste token from environment (tool token was empty)")
	}

	// Create a buffer to store multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Determine file extension from content type
	extension := ".jpg" // Default
	switch contentType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	case "image/gif":
		extension = ".gif"
	case "image/webp":
		extension = ".webp"
	case "image/svg+xml":
		extension = ".svg"
	}

	// Create a form field for the file
	filename := fmt.Sprintf("img_%d%s", time.Now().Unix(), extension)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %v", err)
	}

	// Copy image data into form field
	_, err = io.Copy(part, bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to copy file content: %v", err)
	}

	// Add additional form fields
	_ = writer.WriteField("type", "image")      // Set type to image
	_ = writer.WriteField("ephemeral", "false") // Always keep images permanently

	// Close the writer to finalize the multipart form
	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close writer: %v", err)
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", t.pasteURL, &requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", pasteToken) // No "Bearer " prefix needed
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to upload image, status code: %d, response: %s", resp.StatusCode, body)
	}

	// Parse response JSON to get the paste ID
	var uploadResponse struct {
		ID string `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&uploadResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %v", err)
	}

	// Construct full view URL using the paste ID
	pasteURL := fmt.Sprintf("https://paste.mathizen.net/view?id=%s", uploadResponse.ID)
	return pasteURL, nil
}

// truncateString truncates a string to the specified length and adds "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// valueOrDefault returns the provided value if it's non-zero, otherwise returns the default value
func valueOrDefault(value int, defaultValue int) int {
	if value > 0 {
		return value
	}
	return defaultValue
}

// valueOrDefaultInt returns the provided value if it's within range, otherwise returns the default value
func valueOrDefaultInt(value int, defaultValue int) int {
	if value >= 0 {
		return value
	}
	return defaultValue
}
