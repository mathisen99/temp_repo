package commands

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// sanitizeForIRC cleans the AI response to make it IRC-friendly
func sanitizeForIRC(response string) []string {
	// Clean markdown formatting
	response = cleanMarkdown(response)

	// Fix URL formatting (remove parentheses around URLs)
	response = cleanURLFormatting(response)

	// Replace newlines with spaces, except for paragraph breaks
	response = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(response, "§PARAGRAPH§")
	response = strings.ReplaceAll(response, "\n", " ")
	response = strings.ReplaceAll(response, "§PARAGRAPH§", " | ")

	// Remove multiple spaces
	response = regexp.MustCompile(`\s+`).ReplaceAllString(response, " ")

	// Trim any leading/trailing whitespace
	response = strings.TrimSpace(response)

	// Split into chunks to avoid flooding
	maxLength := 400 // Safe IRC message length
	var chunks []string

	for len(response) > 0 {
		if len(response) <= maxLength {
			chunks = append(chunks, response)
			break
		}

		// Find a good breaking point
		breakPoint := findBreakPoint(response, maxLength)
		chunks = append(chunks, response[:breakPoint])
		response = response[breakPoint:]
		response = strings.TrimSpace(response)
	}

	return chunks
}

// cleanMarkdown removes markdown formatting
func cleanMarkdown(text string) string {
	// Handle code blocks
	text = regexp.MustCompile("```[a-zA-Z]*\n").ReplaceAllString(text, "[code] ")
	text = regexp.MustCompile("```").ReplaceAllString(text, " [/code] ")

	// Handle inline code
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "'$1'")

	// Handle headers
	text = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`).ReplaceAllString(text, "*** $1 ***")

	// Handle bold and italic
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "*$1*")
	text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "*$1*")
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(text, "$1")

	// Handle bullet lists
	text = regexp.MustCompile(`(?m)^[ \t]*[-*+][ \t]+(.+)$`).ReplaceAllString(text, "• $1")

	// Handle numbered lists
	text = regexp.MustCompile(`(?m)^[ \t]*\d+\.[ \t]+(.+)$`).ReplaceAllString(text, "• $1")

	// Handle blockquotes
	text = regexp.MustCompile(`(?m)^>[ \t]*(.+)$`).ReplaceAllString(text, "\"$1\"")

	// Handle links - converting markdown [text](url) to just "text url"
	text = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(text, "$1 $2")

	// Handle images
	text = regexp.MustCompile(`!\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(text, "[Image: $1]")

	// Handle horizontal rules
	text = regexp.MustCompile(`(?m)^[ \t]*[-*_]{3,}[ \t]*$`).ReplaceAllString(text, "---")

	return text
}

// findBreakPoint finds a good place to split a message
func findBreakPoint(text string, maxLength int) int {
	if len(text) <= maxLength {
		return len(text)
	}

	// Try to break at sentence-ending punctuation
	for i := maxLength; i >= maxLength-50 && i > 0; i-- {
		if i < len(text) && (text[i] == '.' || text[i] == '!' || text[i] == '?') &&
			(i+1 >= len(text) || text[i+1] == ' ') {
			return i + 1
		}
	}

	// Try to break at other punctuation
	for i := maxLength; i >= maxLength-50 && i > 0; i-- {
		if i < len(text) && (text[i] == ',' || text[i] == ';' || text[i] == ':') &&
			(i+1 >= len(text) || text[i+1] == ' ') {
			return i + 1
		}
	}

	// Break at pipe separator (from paragraph breaks)
	for i := maxLength; i >= maxLength-50 && i > 0; i-- {
		if i < len(text) && text[i] == '|' && i > 0 && text[i-1] == ' ' &&
			i+1 < len(text) && text[i+1] == ' ' {
			return i + 2
		}
	}

	// Fall back to breaking at spaces
	for i := maxLength; i >= maxLength-50 && i > 0; i-- {
		if i < len(text) && text[i] == ' ' {
			return i + 1
		}
	}

	// Last resort: just break at maxLength
	return maxLength
}

// PasteService sends the content to the paste service and returns the full view URL
func PasteService(content string) (string, error) {
	token := os.Getenv("VALID_PASTE_TOKEN")
	if token == "" {
		return "", fmt.Errorf("VALID_PASTE_TOKEN is not set")
	}

	// Format the content for better readability in paste service
	formattedContent := formatContentForPaste(content)

	url := "https://paste.mathizen.net/paste"

	requestBody, err := json.Marshal(map[string]interface{}{
		"content":   formattedContent,
		"type":      "text",
		"ephemeral": false,
		"title":     "AI Assistant Response",
	})
	if err != nil {
		return "", fmt.Errorf("error marshaling request data: %v", err)
	}

	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 response: %d, message: %s", resp.StatusCode, responseBody)
	}

	// Parse JSON response
	var result map[string]string
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return "", fmt.Errorf("error parsing JSON: %v", err)
	}

	// Extract paste ID and construct full URL
	pasteID, ok := result["id"]
	if !ok {
		return "", fmt.Errorf("paste ID not found in response")
	}

	pasteURL := fmt.Sprintf("https://paste.mathizen.net/view?id=%s", pasteID)
	return pasteURL, nil
}

// formatContentForPaste ensures the content is properly formatted for the paste service
func formatContentForPaste(content string) string {
	// Preserve original markdown formatting
	// But ensure consistent paragraph spacing
	content = strings.ReplaceAll(content, "\r\n", "\n")

	// Ensure single newlines are preserved (unlike in IRC formatting)
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")

	return content
}

// cleanURLFormatting removes parentheses around URLs and fixes common URL formatting issues
func cleanURLFormatting(text string) string {
	// Common URL patterns
	urlPattern := `https?://[^\s)]+`

	// Remove parentheses that surround URLs: (https://example.com) -> https://example.com
	text = regexp.MustCompile(`\((`+urlPattern+`)\)`).ReplaceAllString(text, "$1")

	// Fix "here (https://example.com)" pattern -> "here https://example.com"
	text = regexp.MustCompile(`([a-zA-Z]) \((`+urlPattern+`)\)`).ReplaceAllString(text, "$1 $2")

	// Fix space before punctuation with URL: "https://example.com ." -> "https://example.com."
	text = regexp.MustCompile(`(`+urlPattern+`) ([.,;:!?])`).ReplaceAllString(text, "$1$2")

	// Fix punctuation at end of URLs: "https://example.com." -> "https://example.com"
	text = regexp.MustCompile(`(`+urlPattern+`)([.,;:!?])(\s|$)`).ReplaceAllString(text, "$1$3")

	return text
}
