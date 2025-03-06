package tools

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
)

// PasteArgs represents the arguments for the paste tool
type PasteArgs struct {
	// Common fields
	Ephemeral bool   `json:"ephemeral,omitempty"`
	Title     string `json:"title,omitempty"`
	
	// For image pastes
	ImageURL  string `json:"imageUrl,omitempty"`
	
	// For text pastes
	Content   string `json:"content,omitempty"`
}

// UploadResponse structure for JSON response parsing
type UploadResponse struct {
	ID string `json:"id"`
}

// PasteTool provides image pasting capabilities
type PasteTool struct {
	BaseTool
	client     *http.Client
	pasteURL   string
	pasteToken string
	userAgents []string
}

// NewPasteTool creates a new paste tool
func NewPasteTool() *PasteTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"imageUrl": {
				Type:        jsonschema.String,
				Description: "URL of the image to download and paste (use this OR content, not both)",
			},
			"content": {
				Type:        jsonschema.String,
				Description: "Text content to paste (use this OR imageUrl, not both)",
			},
			"title": {
				Type:        jsonschema.String,
				Description: "Title for the paste (optional)",
			},
			"ephemeral": {
				Type:        jsonschema.Boolean,
				Description: "If true, the content will be deleted after 24 hours (default: false)",
			},
		},
		// No required fields - either imageUrl or content must be provided
	}

	// Set up client with bypasses cert verification as in the example
	client := &http.Client{
		Timeout: 30 * time.Second,
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

	// Try multiple ways to get the paste token - check both environment variables
	pasteToken := os.Getenv("VALID_PASTE_TOKEN")
	if pasteToken == "" {
		pasteToken = os.Getenv("PASTE_TOKEN")
		if pasteToken == "" {
			logger.Warnf("No paste token found in environment variables VALID_PASTE_TOKEN or PASTE_TOKEN")
		}
	}

	return &PasteTool{
		BaseTool: BaseTool{
			ToolName:        "paste",
			ToolDescription: "Upload content (text or images) to the paste site",
			ToolParameters:  params,
		},
		client:     client,
		pasteURL:   "https://paste.mathizen.net/paste",
		pasteToken: pasteToken,
		userAgents: userAgents,
	}
}

// downloadImage downloads an image from a URL
func (t *PasteTool) downloadImage(imageURL string) ([]byte, string, error) {
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
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("image server returned status: %s", resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("URL does not point to a valid image (content-type: %s)", contentType)
	}

	// Read the image data
	imageData, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // Limit to 10MB
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %v", err)
	}
	
	// Resize the image if it's too large (to avoid 413 Request Entity Too Large errors)
	if len(imageData) > 1000000 {
		logger.Infof("Image size is %d bytes, attempting to resize", len(imageData))
		resizedData, newContentType, err := t.resizeImage(imageData, contentType)
		if err == nil {
			imageData = resizedData
			contentType = newContentType
		} else {
			logger.Warnf("Failed to resize image during download: %v", err)
		}
	}

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

	return imageData, extension, nil
}

// uploadToPaste uploads image data to the paste site
func (t *PasteTool) uploadToPaste(imageData []byte, extension string, ephemeral bool) (string, error) {
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

	// Create a form field for the file
	filename := fmt.Sprintf("image_%d%s", time.Now().Unix(), extension)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %v", err)
	}

	// Check if image is too large, and resize again if needed
	if len(imageData) > 2000000 { // 2MB limit to be safe
		logger.Warnf("Image is still large (%d bytes), attempting another resize", len(imageData))
		// Force a more aggressive resize by specifying a lower max dimension
		bounds := bytes.NewReader(imageData)
		img, _, err := image.Decode(bounds)
		if err == nil {
			// More aggressive resize to 600px max dimension
			maxDim := 600
			bounds := img.Bounds()
			width, height := bounds.Dx(), bounds.Dy()
			var newWidth, newHeight int
			
			if width > height {
				newWidth = maxDim
				newHeight = int(float64(height) * float64(maxDim) / float64(width))
			} else {
				newHeight = maxDim
				newWidth = int(float64(width) * float64(maxDim) / float64(height))
			}
			
			// Resize and encode
			resizedImg := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
			buf := new(bytes.Buffer)
			if strings.HasSuffix(extension, ".jpg") || strings.HasSuffix(extension, ".jpeg") {
				jpeg.Encode(buf, resizedImg, &jpeg.Options{Quality: 75}) // Lower quality
			} else {
				png.Encode(buf, resizedImg)
			}
			
			if buf.Len() > 0 && buf.Len() < len(imageData) {
				imageData = buf.Bytes()
				logger.Infof("Second resize: Image now %d bytes (%.1f%% of original)", 
					len(imageData), 100*float64(len(imageData))/2000000)
			}
		}
	}

	// Copy image data into form field
	_, err = io.Copy(part, bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to copy file content: %v", err)
	}

	// Add additional form fields
	_ = writer.WriteField("type", "image") // Set type to image
	if ephemeral {
		_ = writer.WriteField("ephemeral", "true") // Make image ephemeral
	} else {
		_ = writer.WriteField("ephemeral", "false") // Keep image permanently
	}

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
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to upload image, status code: %d, response: %s", resp.StatusCode, body)
	}

	// Parse response JSON
	var uploadResp UploadResponse
	err = json.NewDecoder(resp.Body).Decode(&uploadResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %v", err)
	}

	// Construct full view URL using the paste ID
	pasteURL := fmt.Sprintf("https://paste.mathizen.net/view?id=%s", uploadResp.ID)
	return pasteURL, nil
}

// uploadTextToPaste uploads text content to the paste site
func (t *PasteTool) uploadTextToPaste(content string, title string, ephemeral bool) (string, error) {
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
			return "", fmt.Errorf("paste token not available in environment, text pasting is disabled")
		}
		
		// Log that we had to get the token directly
		logger.Infof("Using paste token from environment (tool token was empty)")
	}

	// Set default title if empty
	if title == "" {
		title = "IRC Bot Paste"
	}

	// Create request body
	requestBody, err := json.Marshal(map[string]interface{}{
		"content":   content,
		"type":      "text",
		"ephemeral": ephemeral,
		"title":     title,
	})
	if err != nil {
		return "", fmt.Errorf("error marshaling request data: %v", err)
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", t.pasteURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", pasteToken)

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to upload text, status code: %d, response: %s", resp.StatusCode, body)
	}

	// Parse response JSON
	var uploadResp UploadResponse
	err = json.NewDecoder(resp.Body).Decode(&uploadResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %v", err)
	}

	// Construct full view URL using the paste ID
	pasteURL := fmt.Sprintf("https://paste.mathizen.net/view?id=%s", uploadResp.ID)
	return pasteURL, nil
}

// resizeImage takes image data and resizes it to a maximum size
func (t *PasteTool) resizeImage(imageData []byte, contentType string) ([]byte, string, error) {
	// No need to resize if less than 1MB
	if len(imageData) < 1000000 {
		return imageData, contentType, nil
	}

	// Decode the image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		logger.Errorf("Failed to decode image: %v", err)
		return imageData, contentType, err
	}

	// Get the current image size
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions, preserving aspect ratio
	// Target max dimension is 800px
	var newWidth, newHeight int
	maxDimension := 800
	if width > height {
		if width > maxDimension {
			newWidth = maxDimension
			newHeight = int(float64(height) * float64(maxDimension) / float64(width))
		} else {
			newWidth = width
			newHeight = height
		}
	} else {
		if height > maxDimension {
			newHeight = maxDimension
			newWidth = int(float64(width) * float64(maxDimension) / float64(height))
		} else {
			newWidth = width
			newHeight = height
		}
	}

	// Resize using Lanczos resampling (high quality)
	resizedImg := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	// Encode the resized image
	buf := new(bytes.Buffer)
	var newContentType string

	if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
		err = jpeg.Encode(buf, resizedImg, &jpeg.Options{Quality: 85})
		newContentType = "image/jpeg"
	} else {
		// Default to PNG if not JPEG
		err = png.Encode(buf, resizedImg)
		newContentType = "image/png"
	}
	
	if err != nil {
		logger.Errorf("Failed to encode resized image: %v", err)
		return imageData, contentType, err
	}

	resizedImageData := buf.Bytes()
	logger.Infof("Image resized from %d bytes to %d bytes (%.1f%% smaller)",
		len(imageData), len(resizedImageData), 100*(1-float64(len(resizedImageData))/float64(len(imageData))))

	return resizedImageData, newContentType, nil
}

func (t *PasteTool) Execute(args string) (string, error) {
	var params PasteArgs
	err := json.Unmarshal([]byte(args), &params)
	if err != nil {
		logger.Errorf("Failed to parse paste args: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	// Check which mode we're in (image or text)
	hasImageURL := params.ImageURL != ""
	hasContent := params.Content != ""

	// Handle error cases
	if !hasImageURL && !hasContent {
		return "", fmt.Errorf("either imageUrl or content is required")
	}
	if hasImageURL && hasContent {
		return "", fmt.Errorf("cannot provide both imageUrl and content, use one or the other")
	}

	// Set default title if not provided
	if params.Title == "" {
		params.Title = "IRC Bot Paste"
	}

	var pasteURL string
	var contentType string

	// Image paste mode
	if hasImageURL {
		imageURL := params.ImageURL
		
		// Check if the URL already starts with http:// or https://
		if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
			imageURL = "https://" + imageURL
		}

		// Download the image
		imageData, extension, err := t.downloadImage(imageURL)
		if err != nil {
			logger.Errorf("Failed to download image: %v", err)
			return "", fmt.Errorf("failed to download image: %v", err)
		}

		logger.Infof("Successfully downloaded image from %s (%d bytes)", imageURL, len(imageData))

		// Upload to paste site
		pasteURL, err = t.uploadToPaste(imageData, extension, params.Ephemeral)
		if err != nil {
			logger.Errorf("Failed to upload to paste site: %v", err)
			return "", fmt.Errorf("failed to upload to paste site: %v", err)
		}
		
		contentType = "Image"
	} else {
		// Text paste mode
		// Upload the text content
		var err error
		pasteURL, err = t.uploadTextToPaste(params.Content, params.Title, params.Ephemeral)
		if err != nil {
			logger.Errorf("Failed to upload text to paste site: %v", err)
			return "", fmt.Errorf("failed to upload text to paste site: %v", err)
		}
		
		contentType = "Text"
	}

	logger.Infof("Successfully uploaded %s to paste site: %s", strings.ToLower(contentType), pasteURL)

	// Format the result
	permanence := "permanent"
	if params.Ephemeral {
		permanence = "ephemeral (will be deleted after 24 hours)"
	}

	return fmt.Sprintf("%s uploaded to paste site (%s): %s", contentType, permanence, pasteURL), nil
}