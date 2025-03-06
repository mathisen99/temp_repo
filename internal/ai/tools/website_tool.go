package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
)

// WebsiteInfoArgs represents the arguments for the fetchWebsiteContent tool
type WebsiteInfoArgs struct {
	URL          string `json:"url"`
	MaxCharCount int    `json:"maxCharCount,omitempty"`
	TimeoutSecs  int    `json:"timeoutSecs,omitempty"`
}

// WebsiteTool provides content from websites
type WebsiteTool struct {
	BaseTool
	userAgents []string
}

// NewWebsiteTool creates a new website content fetching tool
func NewWebsiteTool() *WebsiteTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"url": {
				Type:        jsonschema.String,
				Description: "URL of the website to fetch content from",
			},
			"maxCharCount": {
				Type:        jsonschema.Integer,
				Description: "Maximum number of characters to return from the website (default: 20000)",
			},
			"timeoutSecs": {
				Type:        jsonschema.Integer,
				Description: "Timeout in seconds for the HTTP request (default: 10)",
			},
		},
		Required: []string{"url"},
	}

	// List of modern browser user agents to rotate through
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:124.0) Gecko/20100101 Firefox/124.0",
	}

	return &WebsiteTool{
		BaseTool: BaseTool{
			ToolName:        "fetchWebsiteContent",
			ToolDescription: "Fetch and extract text content from a website URL, removing HTML tags and returning a specified maximum number of characters",
			ToolParameters:  params,
		},
		userAgents: userAgents,
	}
}

// cleanHTML removes HTML tags and normalizes whitespace
func (t *WebsiteTool) cleanHTML(html string) string {
	// Remove script tags
	scriptPattern := regexp.MustCompile(`(?s)<script.*?</script>`)
	html = scriptPattern.ReplaceAllString(html, "")

	// Remove style tags
	stylePattern := regexp.MustCompile(`(?s)<style.*?</style>`)
	html = stylePattern.ReplaceAllString(html, "")

	// Remove all HTML tags
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	text := tagPattern.ReplaceAllString(html, " ")

	// Replace multiple whitespace with a single space
	whitespacePattern := regexp.MustCompile(`\s+`)
	text = whitespacePattern.ReplaceAllString(text, " ")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	return strings.TrimSpace(text)
}

// extractTitle extracts the title from HTML content
func (t *WebsiteTool) extractTitle(html string) string {
	titleRegex := regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	matches := titleRegex.FindStringSubmatch(html)

	if len(matches) > 1 {
		// Clean up the title
		title := strings.TrimSpace(matches[1])
		// Decode HTML entities in the title
		title = strings.ReplaceAll(title, "&nbsp;", " ")
		title = strings.ReplaceAll(title, "&amp;", "&")
		title = strings.ReplaceAll(title, "&lt;", "<")
		title = strings.ReplaceAll(title, "&gt;", ">")
		title = strings.ReplaceAll(title, "&quot;", "\"")
		title = strings.ReplaceAll(title, "&#39;", "'")
		return title
	}

	return "No title found"
}

// extractDomain extracts the domain from a URL
func (t *WebsiteTool) extractDomain(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr // Return the original URL if parsing fails
	}
	return u.Hostname()
}

// Execute processes the tool call with the provided arguments
func (t *WebsiteTool) Execute(args string) (string, error) {
	var params WebsiteInfoArgs

	err := json.Unmarshal([]byte(args), &params)
	if err != nil {
		logger.Errorf("Failed to parse website info args: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	url := params.URL
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	// Set default values if not provided
	maxCharCount := params.MaxCharCount
	if maxCharCount <= 0 {
		maxCharCount = 20000
	}

	timeoutSecs := params.TimeoutSecs
	if timeoutSecs <= 0 {
		timeoutSecs = 10
	} else if timeoutSecs > 30 {
		// Cap timeout at 30 seconds for safety
		timeoutSecs = 30
	}

	// Create a client with the specified timeout and redirect handling
	client := &http.Client{
		Timeout: time.Duration(timeoutSecs) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
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
		},
	}

	// Check if the URL has a scheme, add https:// if it doesn't
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	// Select a random user agent
	userAgent := t.userAgents[time.Now().UnixNano()%int64(len(t.userAgents))]

	// Create request with appropriate headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Errorf("Failed to create request: %v", err)
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set user agent and other headers to appear as a normal browser
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	// Log the request being made
	logger.Infof("Fetching content from URL: %s (timeout: %ds)", url, timeoutSecs)

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to fetch website: %v", err)
		return "", fmt.Errorf("failed to fetch website: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Website returned non-OK status: %d %s", resp.StatusCode, resp.Status)
		return "", fmt.Errorf("website returned status: %s", resp.Status)
	}

	// Get Content-Type to extract charset if available
	contentType := resp.Header.Get("Content-Type")
	logger.Infof("Content-Type: %s", contentType)

	// Read response body with a limit to avoid memory issues
	// Limit to 5MB to prevent memory issues with very large pages
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		logger.Errorf("Failed to read response body: %v", err)
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var content string

	// Handle different content types
	if strings.Contains(contentType, "text/html") {
		// For HTML content, clean and extract text
		content = t.cleanHTML(string(bodyBytes))
	} else if strings.Contains(contentType, "application/json") {
		// For JSON, just prettify it
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil {
			content = prettyJSON.String()
		} else {
			// If can't prettify, just use raw JSON
			content = string(bodyBytes)
		}
	} else if strings.Contains(contentType, "text/plain") {
		// For plain text, use as is
		content = string(bodyBytes)
	} else {
		// For other content types, just take the raw content but note the content type
		content = fmt.Sprintf("[Content type: %s]\n\n%s", contentType, string(bodyBytes))
	}

	// Get metadata
	title := t.extractTitle(string(bodyBytes))
	domain := t.extractDomain(url)
	contentLength := len(content)

	// Create summary of metadata
	metadata := fmt.Sprintf("Website: %s\nTitle: %s\nContent Length: %d characters\n\n",
		domain, title, contentLength)

	// Truncate content if necessary, leaving room for metadata
	if len(content)+len(metadata) > maxCharCount {
		maxContentChars := maxCharCount - len(metadata) - 20 // Extra space for truncation message
		if maxContentChars > 0 {
			content = content[:maxContentChars] + "... (content truncated)"
		} else {
			// If metadata alone exceeds maxCharCount, truncate the metadata
			return metadata[:maxCharCount-20] + "... (truncated)", nil
		}
	}

	logger.Debugf("Returning this: Metadata: %s || Content: %s", metadata, content)

	return metadata + content, nil
}
