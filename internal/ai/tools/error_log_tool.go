package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
	"ircbot/internal/userlevels"
)

// ErrorLogArgs represents the arguments for the getErrorLog tool
type ErrorLogArgs struct {
	UserHostmask string `json:"userHostmask"`        // Required to check admin permission
	LineCount    int    `json:"lineCount,omitempty"` // Optional, number of lines to retrieve (default: 50)
	Query        string `json:"query,omitempty"`     // Optional search term for filtering logs
}

// ErrorLogTool provides access to the bot's error log for administrators
type ErrorLogTool struct {
	BaseTool
}

// NewErrorLogTool creates a new error log tool
func NewErrorLogTool() *ErrorLogTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"userHostmask": {
				Type:        jsonschema.String,
				Description: "The hostmask of the user requesting the error log (used for permission check)",
			},
			"lineCount": {
				Type:        jsonschema.Integer,
				Description: "Number of lines to retrieve from the error log (default: 50, max: 200)",
			},
			"query": {
				Type:        jsonschema.String,
				Description: "Optional search term to find specific error messages",
			},
		},
		Required: []string{"userHostmask"},
	}

	return &ErrorLogTool{
		BaseTool: BaseTool{
			ToolName:        "getErrorLog",
			ToolDescription: "Get the bot's error logs. This tool is restricted to administrators and owners only. Use this to see recent errors or search for specific error messages.",
			ToolParameters:  params,
		},
	}
}

// Execute processes the tool call with the provided arguments
func (t *ErrorLogTool) Execute(args string) (string, error) {
	logger.AIDebugf("ErrorLogTool.Execute called with args: %s", args)

	var params ErrorLogArgs
	err := json.Unmarshal([]byte(args), &params)
	if err != nil {
		logger.Errorf("Failed to parse error log args: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	logger.AIDebugf("ErrorLogTool params: hostmask=%s, lineCount=%d, query=%s",
		params.UserHostmask, params.LineCount, params.Query)

	// Check admin permission
	if !userlevels.HasPermission(params.UserHostmask, userlevels.Admin) {
		logger.Warnf("Unauthorized access attempt to error logs from %s", params.UserHostmask)
		return "", fmt.Errorf("access denied: error logs are restricted to administrators")
	}

	// Set default line count if not specified
	lineCount := params.LineCount
	if lineCount <= 0 {
		lineCount = 50
	}
	if lineCount > 200 {
		lineCount = 200 // Cap at 200 lines maximum
	}

	// Get the error log
	return getErrorLog(lineCount, params.Query)
}

// getErrorLog retrieves the specified number of lines from the error log file
func getErrorLog(lineCount int, query string) (string, error) {
	// Construct error log file path
	dataDir := filepath.Join("data")
	logPath := filepath.Join(dataDir, "error.log")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "Error log file does not exist.", nil
	}

	// Open log file
	file, err := os.Open(logPath)
	if err != nil {
		logger.Errorf("Failed to open error log file: %v", err)
		return "", fmt.Errorf("failed to access error log: %v", err)
	}
	defer file.Close()

	// Read file to get lines
	var lines []string
	logLines, err := readLastLines(file, lineCount, query)
	if err != nil {
		logger.Errorf("Failed to read error log: %v", err)
		return "", fmt.Errorf("failed to read error log: %v", err)
	}
	lines = logLines

	// Check if we found any lines
	if len(lines) == 0 {
		if query != "" {
			return fmt.Sprintf("No error log entries matching '%s' found.", query), nil
		}
		return "Error log is empty.", nil
	}

	// Format the output
	var result string
	if query != "" {
		result = fmt.Sprintf("Error log entries matching '%s' (showing %d lines):\n\n",
			query, len(lines))
	} else {
		result = fmt.Sprintf("Recent error log entries (showing %d lines):\n\n",
			len(lines))
	}

	result += strings.Join(lines, "\n")
	return result, nil
}

// readLastLines reads the last n lines from a file, optionally filtering by a search query
func readLastLines(file *os.File, n int, query string) ([]string, error) {
	// Start by reading the entire file (with a reasonable size limit)
	const maxBytes = 1024 * 1024 // 1MB max to prevent memory issues

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Determine read size (limited to maxBytes)
	readSize := int64(maxBytes)
	if fileInfo.Size() < readSize {
		readSize = fileInfo.Size()
	}

	// Position cursor to read the last part of the file
	_, err = file.Seek(-readSize, io.SeekEnd)
	if err != nil && err != io.EOF {
		// If we can't seek backward (e.g., file is smaller), start from beginning
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}

	// Read file content
	content := make([]byte, readSize)
	_, err = io.ReadFull(file, content)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	// Convert to string and split into lines
	text := string(content)

	// Handle partial first line by finding the first newline
	if readSize < fileInfo.Size() {
		if idx := strings.Index(text, "\n"); idx >= 0 {
			text = text[idx+1:] // Skip the first partial line
		}
	}

	allLines := strings.Split(text, "\n")

	// Filter lines based on query
	var filteredLines []string
	if query != "" {
		for _, line := range allLines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				filteredLines = append(filteredLines, line)
			}
		}
	} else {
		filteredLines = allLines
	}

	// Ensure we only return up to n lines
	start := 0
	if len(filteredLines) > n {
		start = len(filteredLines) - n
	}

	return filteredLines[start:], nil
}
