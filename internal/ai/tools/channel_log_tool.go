package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
)

// ChannelLogArgs represents the arguments for the getChannelLog tool
type ChannelLogArgs struct {
	ChannelName string `json:"channelName"`
	Date        string `json:"date,omitempty"`        // Optional, format: YYYY-MM-DD, defaults to today
	Query       string `json:"query,omitempty"`       // Optional search term for filtering logs
	StartDate   string `json:"startDate,omitempty"`   // Optional start date for date range, only used if query is specified
	EndDate     string `json:"endDate,omitempty"`     // Optional end date for date range, only used if query is specified
}

// ChannelLogTool provides access to channel logs with search capabilities
type ChannelLogTool struct {
	BaseTool
}

// NewChannelLogTool creates a new channel log tool
func NewChannelLogTool() *ChannelLogTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"channelName": {
				Type:        jsonschema.String,
				Description: "IRC channel name to get logs for (including the # symbol)",
			},
			"date": {
				Type:        jsonschema.String,
				Description: "Date in YYYY-MM-DD format. Can use 'yesterday' for the previous day. Used for viewing full logs from a specific day. Ignored if 'query' is provided.",
			},
			"query": {
				Type:        jsonschema.String,
				Description: "Optional search term to find specific messages across logs. If provided, performs a search instead of retrieving full logs.",
			},
			"startDate": {
				Type:        jsonschema.String,
				Description: "Optional start date in YYYY-MM-DD format for search queries. Only used if 'query' is provided. Defaults to 7 days ago.",
			},
			"endDate": {
				Type:        jsonschema.String,
				Description: "Optional end date in YYYY-MM-DD format for search queries. Only used if 'query' is provided. Defaults to today.",
			},
		},
		Required: []string{"channelName"},
	}

	return &ChannelLogTool{
		BaseTool: BaseTool{
			ToolName:        "getChannelLog",
			ToolDescription: "Get IRC channel logs for a specific channel, with optional search capabilities. Use this to see what happened on a specific day or find keywords across multiple days.",
			ToolParameters:  params,
		},
	}
}

// Execute processes the tool call with the provided arguments
func (t *ChannelLogTool) Execute(args string) (string, error) {
	logger.AIDebugf("ChannelLogTool.Execute called with args: %s", args)
	
	var params ChannelLogArgs

	err := json.Unmarshal([]byte(args), &params)
	if err != nil {
		logger.Errorf("Failed to parse channel log args: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	logger.AIDebugf("ChannelLogTool params: channel=%s, date=%s, query=%s", 
		params.ChannelName, params.Date, params.Query)

	// Validate channel name
	channelName := params.ChannelName
	if channelName == "" {
		logger.Errorf("ChannelLogTool: channel name is required")
		return "", fmt.Errorf("channel name is required")
	}
	
	// Make sure channel name starts with #
	if !strings.HasPrefix(channelName, "#") {
		channelName = "#" + channelName
		logger.Debugf("ChannelLogTool: Added # prefix to channel name: %s", channelName)
	}

	// Determine if we're doing a search or full log retrieval
	if params.Query != "" {
		// Search mode - search across multiple days
		return searchChannelLogs(channelName, params.Query, params.StartDate, params.EndDate)
	} else {
		// Full log mode - get logs for a specific day
		return getFullChannelLog(channelName, params.Date)
	}
}

// getFullChannelLog retrieves the full logs for a specific channel and date
func getFullChannelLog(channelName, dateStr string) (string, error) {
	logger.Debugf("getFullChannelLog: Starting retrieval for channel %s, date %s", channelName, dateStr)
	
	// Parse date
	var logDate time.Time
	var err error
	
	if dateStr == "" {
		// Default to today
		logDate = time.Now()
		logger.Debugf("getFullChannelLog: No date provided, using today: %s", logDate.Format("2006-01-02"))
	} else if strings.ToLower(dateStr) == "yesterday" {
		// Special case for yesterday
		logDate = time.Now().AddDate(0, 0, -1)
		logger.Debugf("getFullChannelLog: Using yesterday: %s", logDate.Format("2006-01-02"))
	} else {
		// Parse provided date
		logDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			logger.Errorf("getFullChannelLog: Invalid date format: %v", err)
			return "", fmt.Errorf("invalid date format. Please use YYYY-MM-DD: %v", err)
		}
		
		// Validate the year - prevent using dates from wrong years
		currentYear := time.Now().Year()
		if logDate.Year() < currentYear-1 || logDate.Year() > currentYear {
			logger.Warnf("getFullChannelLog: Date %s has incorrect year %d, using today instead", 
				dateStr, logDate.Year())
			logDate = time.Now() // Reset to today if year is wrong
		}
		
		// If the requested date is in the future, use today instead
		if logDate.After(time.Now()) {
			logger.Warnf("getFullChannelLog: Date %s is in the future, using today instead", dateStr)
			logDate = time.Now()
		}
		
		logger.Debugf("getFullChannelLog: Using specified date: %s", logDate.Format("2006-01-02"))
	}

	// Format the date for file access
	formattedDate := logDate.Format("2006-01-02")

	// Construct log file path
	baseDir := filepath.Join("logs", "CHANNEL")
	var foundChannelName string
	var logFile string
	
	// First check for exact match
	exactLogDir := filepath.Join(baseDir, channelName)
	exactLogFile := filepath.Join(exactLogDir, formattedDate+".log")
	if _, err := os.Stat(exactLogFile); !os.IsNotExist(err) {
		foundChannelName = channelName
		logFile = exactLogFile
	} else {
		// Try similar channel names
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			return fmt.Sprintf("Error reading log directory: %v", err), nil
		}
		
		// Try to find closest match
		for _, entry := range entries {
			if entry.IsDir() {
				// Try exact case-insensitive match first
				if strings.EqualFold(entry.Name(), channelName) {
					foundChannelName = entry.Name()
					logFile = filepath.Join(baseDir, foundChannelName, formattedDate+".log")
					if _, err := os.Stat(logFile); !os.IsNotExist(err) {
						break
					}
				}
				
				// Try without the # prefix if present
				noHashChannel := strings.TrimPrefix(channelName, "#")
				noHashEntry := strings.TrimPrefix(entry.Name(), "#")
				if strings.EqualFold(noHashEntry, noHashChannel) {
					foundChannelName = entry.Name()
					logFile = filepath.Join(baseDir, foundChannelName, formattedDate+".log")
					if _, err := os.Stat(logFile); !os.IsNotExist(err) {
						break
					}
				}
			}
		}
	}
	
	// If we didn't find a matching log file
	if logFile == "" || foundChannelName == "" {
		return fmt.Sprintf("No logs found for channel %s on %s", channelName, formattedDate), nil
	}
	
	// Check if log file exists (final check)
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		logger.Warnf("getFullChannelLog: No logs found at path %s", logFile)
		return fmt.Sprintf("No logs found for channel %s on %s", foundChannelName, formattedDate), nil
	}

	logger.Debugf("getFullChannelLog: Found log file at %s", logFile)

	// Read log file
	file, err := os.Open(logFile)
	if err != nil {
		logger.Errorf("Failed to open log file %s: %v", logFile, err)
		return "", fmt.Errorf("failed to access logs: %v", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		logger.Errorf("Failed to get file info for %s: %v", logFile, err)
		return "", fmt.Errorf("failed to get file info: %v", err)
	}

	// Determine read size (limit to 20K characters)
	maxBytes := int64(20000)
	readSize := maxBytes
	fileSize := fileInfo.Size()
	
	if fileSize <= maxBytes {
		// Small file, read it all
		readSize = fileSize
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			logger.Errorf("Failed to seek to start of file %s: %v", logFile, err)
			return "", fmt.Errorf("failed to read logs: %v", err)
		}
	} else {
		// Larger file, read from the end to get most recent logs
		if _, err := file.Seek(-maxBytes, io.SeekEnd); err != nil {
			logger.Errorf("Failed to seek in file %s: %v", logFile, err)
			return "", fmt.Errorf("failed to read logs: %v", err)
		}
	}

	// Read file content
	content := make([]byte, readSize)
	n, err := io.ReadFull(file, content)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		logger.Errorf("Failed to read log file %s: %v", logFile, err)
		return "", fmt.Errorf("failed to read logs: %v", err)
	}

	// Trim content to actual bytes read
	content = content[:n]
	logContent := string(content)

	// If we started reading in the middle of the file, find the first complete line
	if fileSize > maxBytes {
		if idx := strings.Index(logContent, "\n"); idx >= 0 {
			logContent = logContent[idx+1:]
		}
		
		// Add a note about truncation
		logContent = fmt.Sprintf("[Note: Log file truncated, showing last %d characters of %d total]\n\n", 
			len(logContent), fileSize) + logContent
	}

	// Return log content
	if logContent == "" {
		return fmt.Sprintf("Log file exists but is empty for channel %s on %s", foundChannelName, formattedDate), nil
	}

	if channelName != foundChannelName {
		return fmt.Sprintf("Channel logs for %s (found match for '%s') on %s:\n\n%s", 
			foundChannelName, channelName, formattedDate, logContent), nil
	}
	
	return fmt.Sprintf("Channel logs for %s on %s:\n\n%s", foundChannelName, formattedDate, logContent), nil
}

// searchChannelLogs searches for a query across multiple days of logs
func searchChannelLogs(channelName, query, startDateStr, endDateStr string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("search query is required")
	}
	
	// Parse dates
	now := time.Now()
	startDate := now.AddDate(0, 0, -7) // Default to 7 days ago
	endDate := now                     // Default to today

	var err error
	currentYear := time.Now().Year()
	
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return "", fmt.Errorf("invalid start date format. Please use YYYY-MM-DD: %v", err)
		}
		
		// Validate the year
		if startDate.Year() < currentYear-1 || startDate.Year() > currentYear {
			logger.Warnf("searchChannelLogs: Start date %s has incorrect year %d, using default", 
				startDateStr, startDate.Year())
			startDate = now.AddDate(0, 0, -7) // Reset to default
		}
		
		// If start date is in the future, use today minus 7 days
		if startDate.After(now) {
			logger.Warnf("searchChannelLogs: Start date %s is in the future, using default", startDateStr)
			startDate = now.AddDate(0, 0, -7)
		}
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return "", fmt.Errorf("invalid end date format. Please use YYYY-MM-DD: %v", err)
		}
		
		// Validate the year
		if endDate.Year() < currentYear-1 || endDate.Year() > currentYear {
			logger.Warnf("searchChannelLogs: End date %s has incorrect year %d, using today", 
				endDateStr, endDate.Year())
			endDate = now // Reset to today
		}
		
		// If end date is in the future, use today
		if endDate.After(now) {
			logger.Warnf("searchChannelLogs: End date %s is in the future, using today", endDateStr)
			endDate = now
		}
	}

	// Make sure start date is before end date
	if startDate.After(endDate) {
		return "", fmt.Errorf("start date must be before end date")
	}

	// Construct log directory path
	baseDir := filepath.Join("logs", "CHANNEL")
	var foundChannelName string
	var foundLogDir string
	
	// First check for exact match
	exactLogDir := filepath.Join(baseDir, channelName)
	if _, err := os.Stat(exactLogDir); !os.IsNotExist(err) {
		foundChannelName = channelName
		foundLogDir = exactLogDir
	} else {
		// Try similar channel names
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			return fmt.Sprintf("Error reading log directory: %v", err), nil
		}
		
		// Try to find closest match
		for _, entry := range entries {
			if entry.IsDir() {
				// Try exact case-insensitive match first
				if strings.EqualFold(entry.Name(), channelName) {
					foundChannelName = entry.Name()
					foundLogDir = filepath.Join(baseDir, foundChannelName)
					break
				}
				
				// Try without the # prefix if present
				noHashChannel := strings.TrimPrefix(channelName, "#")
				noHashEntry := strings.TrimPrefix(entry.Name(), "#")
				if strings.EqualFold(noHashEntry, noHashChannel) {
					foundChannelName = entry.Name()
					foundLogDir = filepath.Join(baseDir, foundChannelName)
					break
				}
			}
		}
	}
	
	// If we didn't find a matching log directory
	if foundLogDir == "" || foundChannelName == "" {
		return fmt.Sprintf("No logs found for channel %s", channelName), nil
	}
	
	// Final check
	if _, err := os.Stat(foundLogDir); os.IsNotExist(err) {
		return fmt.Sprintf("No logs found for channel %s", channelName), nil
	}

	// Search through logs from startDate to endDate
	results := []string{}
	maxResults := 50 // Limit the number of results
	resultsCount := 0
	totalResultsCount := 0
	totalBytesRead := 0
	maxBytes := 15000 // Limit total content to ~15K characters

	// For each date in the range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		logFile := filepath.Join(foundLogDir, dateStr+".log")

		// Skip if log file doesn't exist
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			continue
		}

		// Open and read the log file
		file, err := os.Open(logFile)
		if err != nil {
			logger.Errorf("Failed to open log file %s: %v", logFile, err)
			continue
		}

		// Read all lines first to enable more context
		scanner := bufio.NewScanner(file)
		var allLines []string
		
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}
		
		if err := scanner.Err(); err != nil {
			logger.Errorf("Error reading log file %s: %v", logFile, err)
			file.Close()
			continue
		}
		
		file.Close()
		
		// Process the file contents to find matches with extended context
		const contextLines = 15 // Number of lines before/after match to include
		
		for i, line := range allLines {
			// Check if line contains search query
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				// Process match
				totalResultsCount++
				
				// Skip if we've reached result limit
				if resultsCount >= maxResults {
					continue
				}
				
				// Get extensive context around the match
				startIdx := max(0, i-contextLines)
				endIdx := min(len(allLines)-1, i+contextLines)
				
				// Extract context with the matching line
				contextBuffer := allLines[startIdx:endIdx+1]
				
				// Format context with match
				context := formatContextLog(contextBuffer, dateStr)
				
				// Add to results if we have room
				if totalBytesRead+len(context) <= maxBytes {
					results = append(results, context)
					resultsCount++
					totalBytesRead += len(context)
				} else if resultsCount > 0 {
					// If we've already added some results but can't add more, break
					goto DoneSearching // Break out of both loops
				}
			}
		}
	}

DoneSearching:
	// Return results
	if len(results) == 0 {
		if totalResultsCount > 0 {
			return fmt.Sprintf("Found %d matches for '%s' in %s logs, but all exceeded size limit", 
				totalResultsCount, query, foundChannelName), nil
		}
		return fmt.Sprintf("No matches found for '%s' in %s logs between %s and %s", 
			query, foundChannelName, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")), nil
	}

	// Build response
	var response string
	if channelName != foundChannelName {
		response = fmt.Sprintf("Search results for '%s' in %s (found match for '%s') logs (showing %d of %d matches):\n\n", 
			query, foundChannelName, channelName, len(results), totalResultsCount)
	} else {
		response = fmt.Sprintf("Search results for '%s' in %s logs (showing %d of %d matches):\n\n", 
			query, foundChannelName, len(results), totalResultsCount)
	}
	
	response += strings.Join(results, "\n\n")
	
	if totalResultsCount > len(results) {
		response += fmt.Sprintf("\n\n[Note: %d additional matches were found but not shown due to size limits]", 
			totalResultsCount - len(results))
	}
	
	return response, nil
}

// formatContextLog formats the context lines with the given date
func formatContextLog(lines []string, date string) string {
	result := fmt.Sprintf("On %s:\n", date)
	
	for _, line := range lines {
		result += fmt.Sprintf("  %s\n", line)
	}
	
	return result
}

// Helper functions for index bounds
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}