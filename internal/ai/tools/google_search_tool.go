package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
)

const (
	googleSearchURL = "https://www.googleapis.com/customsearch/v1"
	maxResults      = 5
	maxContentChars = 10000 // Maximum characters to extract from each webpage
)

// GoogleSearchArgs represents the arguments for the searchWeb tool
type GoogleSearchArgs struct {
	Query       string `json:"query"`
	ResultCount int    `json:"resultCount,omitempty"`
	Simple      bool   `json:"simple,omitempty"` // If true, returns just search results without fetching content
}

// GoogleSearchResponse represents the JSON response from the Google Custom Search API
type GoogleSearchResponse struct {
	Items []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		Snippet     string `json:"snippet"`
		DisplayLink string `json:"displayLink"`
	} `json:"items"`
}

// WebsiteContent stores the extracted content from a search result
type WebsiteContent struct {
	URL     string
	Title   string
	Content string
	Error   error
}

// GoogleSearchTool provides web search capabilities with content analysis
type GoogleSearchTool struct {
	BaseTool
	searchEngineID string
	userAgents     []string
	openaiAPIKey   string
	openaiClient   *openai.Client
}

// NewGoogleSearchTool creates a new web search tool
func NewGoogleSearchTool() *GoogleSearchTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"query": {
				Type:        jsonschema.String,
				Description: "The search query to look up on the web",
			},
			"resultCount": {
				Type:        jsonschema.Integer,
				Description: "Number of search results to analyze (default: 3, max: 5)",
			},
			"simple": {
				Type:        jsonschema.Boolean,
				Description: "If true, returns just search results without fetching and analyzing content (default: false)",
			},
		},
		Required: []string{"query"},
	}

	// Get the search engine ID from environment
	searchEngineID := os.Getenv("GOOGLE_SEARCH_ENGINE_ID")
	if searchEngineID == "" {
		logger.Warnf("GOOGLE_SEARCH_ENGINE_ID not set, using default value")
		searchEngineID = "" // Will be checked during execution
	}

	// Get OpenAI API key for content summarization
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")

	// List of modern browser user agents to rotate through
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:124.0) Gecko/20100101 Firefox/124.0",
	}

	// Create OpenAI client for summarizations
	var openaiClient *openai.Client
	if openaiAPIKey != "" {
		openaiClient = openai.NewClient(openaiAPIKey)
	}

	return &GoogleSearchTool{
		BaseTool: BaseTool{
			ToolName:        "searchWeb",
			ToolDescription: "Search the web for information about a topic, fetch content from relevant websites, and provide a comprehensive answer",
			ToolParameters:  params,
		},
		searchEngineID: searchEngineID,
		userAgents:     userAgents,
		openaiAPIKey:   openaiAPIKey,
		openaiClient:   openaiClient,
	}
}

// Execute processes the search request and returns the results
func (t *GoogleSearchTool) Execute(args string) (string, error) {
	var params GoogleSearchArgs

	// Parse and validate arguments
	err := json.Unmarshal([]byte(args), &params)
	if err != nil {
		logger.Errorf("Failed to parse search args: %v", err)
		return "", fmt.Errorf("invalid arguments: %v", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Get the API key
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	if googleAPIKey == "" {
		return "", fmt.Errorf("GOOGLE_API_KEY not set")
	}

	// Verify search engine ID
	if t.searchEngineID == "" {
		return "", fmt.Errorf("GOOGLE_SEARCH_ENGINE_ID not set")
	}

	// Set default values
	resultCount := params.ResultCount
	if resultCount <= 0 {
		resultCount = 3
	} else if resultCount > maxResults {
		resultCount = maxResults
	}

	// Optimize the search query
	logger.Debugf("[SearchWeb] Original user query: %s", params.Query)
	optimizedQuery, err := t.optimizeSearchQuery(params.Query)
	if err != nil {
		logger.Warnf("[SearchWeb] Failed to optimize search query: %v. Using original query.", err)
		optimizedQuery = params.Query
	}

	logger.Infof("[SearchWeb] Performing Google search for: %s", optimizedQuery)
	logger.Debugf("[SearchWeb] Using search engine ID: %s", t.searchEngineID)

	// Perform the search
	searchResults, err := t.performGoogleSearch(optimizedQuery, googleAPIKey, resultCount)
	if err != nil {
		return "", fmt.Errorf("search error: %v", err)
	}

	if len(searchResults.Items) == 0 {
		return "No search results found for the query.", nil
	}

	// If simple mode is requested or we don't have OpenAI API key, just return the search results
	if params.Simple || t.openaiClient == nil {
		return t.formatSearchResults(searchResults, params.Query, resultCount), nil
	}

	// For comprehensive mode, fetch and analyze content from the websites
	allContent, err := t.fetchAndProcessContent(searchResults, params.Query, resultCount)
	if err != nil {
		logger.Errorf("Error processing search content: %v", err)
		// Fall back to simple results if content processing fails
		return t.formatSearchResults(searchResults, params.Query, resultCount), nil
	}

	return allContent, nil
}

// optimizeSearchQuery transforms the query to make it more effective for search
func (t *GoogleSearchTool) optimizeSearchQuery(query string) (string, error) {
	// Simple query optimization without using AI to avoid dependency cycles

	// Get the current date for context - useful for time-sensitive queries
	currentYear := time.Now().Year()

	// Only add year for queries that might benefit from current context
	timeRelatedTerms := []string{
		"current", "latest", "recent", "new", "today", "now",
		"this year", "this month", "this week", "trends", "news",
	}

	needsYear := false
	queryLower := strings.ToLower(query)
	for _, term := range timeRelatedTerms {
		if strings.Contains(queryLower, term) {
			needsYear = true
			break
		}
	}

	// Format search query for optimal results
	optimizedQuery := query

	// Don't modify queries that are already well-formed
	if !strings.Contains(query, "site:") && !strings.Contains(query, "filetype:") {
		// Add year for time-sensitive queries that don't already include a year
		if needsYear && !strings.Contains(queryLower, fmt.Sprintf("%d", currentYear)) {
			optimizedQuery = fmt.Sprintf("%s %d", query, currentYear)
		}

		// Remove question words and other common terms that don't help search
		questionWords := []string{
			"what is", "what are", "who is", "who are", "where is", "where are",
			"when is", "when are", "why is", "why are", "how to", "how do",
			"can you", "please", "tell me about", "i want to know about",
		}

		for _, word := range questionWords {
			if strings.HasPrefix(strings.ToLower(optimizedQuery), word) {
				optimizedQuery = optimizedQuery[len(word):]
				break
			}
		}

		// Trim spaces and special characters
		optimizedQuery = strings.TrimSpace(optimizedQuery)
		optimizedQuery = strings.Trim(optimizedQuery, "?.,;:")
	}

	logger.Debugf("Original query: %s, Optimized query: %s", query, optimizedQuery)
	return optimizedQuery, nil
}

// performGoogleSearch executes the actual Google Custom Search API request
func (t *GoogleSearchTool) performGoogleSearch(query, apiKey string, resultCount int) (*GoogleSearchResponse, error) {
	// Build the search URL
	searchURL := fmt.Sprintf("%s?key=%s&cx=%s&q=%s&num=%d",
		googleSearchURL,
		apiKey,
		t.searchEngineID,
		url.QueryEscape(query),
		resultCount,
	)

	// Log the search URL (with API key partially redacted for security)
	redactedURL := strings.Replace(searchURL, apiKey, apiKey[:6]+"..."+apiKey[len(apiKey)-4:], 1)
	logger.Debugf("[SearchWeb] Google API URL: %s", redactedURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	logger.Debugf("[SearchWeb] Using HTTP client with 10s timeout")

	// Make the request
	logger.Debugf("[SearchWeb] Sending request to Google Custom Search API")
	resp, err := client.Get(searchURL)
	if err != nil {
		logger.Errorf("[SearchWeb] HTTP request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Errorf("[SearchWeb] Google API error: %s, %s", resp.Status, string(bodyBytes))
		return nil, fmt.Errorf("API returned status code %d", resp.StatusCode)
	}
	logger.Debugf("[SearchWeb] Google API request successful with status: %s", resp.Status)

	// Parse the response
	var searchResponse GoogleSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		logger.Errorf("[SearchWeb] Failed to parse API response: %v", err)
		return nil, err
	}

	// Log found results
	if len(searchResponse.Items) > 0 {
		logger.Infof("[SearchWeb] Found %d search results", len(searchResponse.Items))
		for i, item := range searchResponse.Items {
			if i < resultCount {
				logger.Debugf("[SearchWeb] Result %d: %s - %s", i+1, item.Title, item.Link)
			}
		}
	} else {
		logger.Warnf("[SearchWeb] No results found for query: %s", query)
	}

	return &searchResponse, nil
}

// fetchAndProcessContent fetches content from search results and processes it
func (t *GoogleSearchTool) fetchAndProcessContent(results *GoogleSearchResponse, query string, resultCount int) (string, error) {
	// Limit results to process
	itemsToProcess := results.Items
	if len(itemsToProcess) > resultCount {
		itemsToProcess = itemsToProcess[:resultCount]
	}

	// Fetch content from all URLs concurrently
	logger.Infof("[SearchWeb] Starting to fetch content from %d websites concurrently", len(itemsToProcess))
	var wg sync.WaitGroup
	contentChan := make(chan WebsiteContent, len(itemsToProcess))

	for i := range itemsToProcess {
		wg.Add(1)
		// Create local copies of the variables to use in the goroutine
		title := itemsToProcess[i].Title
		link := itemsToProcess[i].Link

		logger.Debugf("[SearchWeb] Queuing fetch for result %d: %s - %s", i+1, title, link)

		go func(index int, title, link string) {
			defer wg.Done()

			logger.Debugf("[SearchWeb] Started fetching content from result %d: %s", index+1, link)

			// Fetch the website content
			content, err := t.fetchWebsiteContent(link, maxContentChars)
			if err != nil {
				logger.Warnf("[SearchWeb] Error fetching content from %s: %v", link, err)
				contentChan <- WebsiteContent{URL: link, Title: title, Error: err}
				return
			}

			contentLength := len(content)
			logger.Debugf("[SearchWeb] Successfully fetched content from %s (%d characters)", link, contentLength)

			contentChan <- WebsiteContent{
				URL:     link,
				Title:   title,
				Content: content,
			}
		}(i, title, link)
	}

	// Close channel when all goroutines are done
	go func() {
		wg.Wait()
		close(contentChan)
	}()

	// Collect results
	logger.Debugf("[SearchWeb] Waiting for all content fetch operations to complete")
	var contents []WebsiteContent
	var errorCount int
	for content := range contentChan {
		if content.Error == nil && content.Content != "" {
			logger.Debugf("[SearchWeb] Adding content from %s to results list (%d characters)",
				content.URL, len(content.Content))
			contents = append(contents, content)
		} else {
			errorCount++
			logger.Warnf("[SearchWeb] Skipping content from %s due to error or empty content", content.URL)
		}
	}
	logger.Infof("[SearchWeb] Content fetch complete: %d successful, %d failed", len(contents), errorCount)

	// If no content could be fetched, return an error
	if len(contents) == 0 {
		logger.Errorf("[SearchWeb] Failed to fetch usable content from any search result")
		return "", fmt.Errorf("could not fetch content from any of the search results")
	}

	// Summarize each piece of content in relation to the query
	logger.Infof("[SearchWeb] Starting to summarize content from %d websites", len(contents))
	var summaries []string
	var summaryCount int
	for i, content := range contents {
		logger.Debugf("[SearchWeb] Summarizing content %d/%d: %s", i+1, len(contents), content.URL)
		summary, err := t.summarizeContent(content, query)
		if err != nil {
			logger.Warnf("[SearchWeb] Error summarizing content from %s: %v", content.URL, err)
			continue
		}

		summaryCount++
		logger.Debugf("[SearchWeb] Successfully summarized content from %s (%d characters)",
			content.URL, len(summary))

		// Add the summary with its source
		summaries = append(summaries, fmt.Sprintf("From %s (%s):\n%s", content.Title, content.URL, summary))
	}

	// If no summaries could be generated, return an error
	if len(summaries) == 0 {
		logger.Errorf("[SearchWeb] Failed to generate any summaries from the search results")
		return "", fmt.Errorf("could not summarize content from any of the search results")
	}

	logger.Infof("[SearchWeb] Successfully summarized %d/%d websites", summaryCount, len(contents))

	// Create a final comprehensive answer
	logger.Infof("[SearchWeb] Generating final comprehensive answer from %d summaries", len(summaries))
	finalAnswer, err := t.createFinalAnswer(summaries, query)
	if err != nil {
		logger.Errorf("[SearchWeb] Error creating final answer: %v", err)
		logger.Warnf("[SearchWeb] Falling back to returning raw summaries without synthesis")
		// Return the individual summaries if we can't create a final answer
		return fmt.Sprintf("# Search Results for: %s\n\n%s", query, strings.Join(summaries, "\n\n")), nil
	}

	logger.Infof("[SearchWeb] Successfully generated final answer (%d characters)", len(finalAnswer))

	// Format the final result with sources
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Answer for: %s\n\n", query))
	sb.WriteString(finalAnswer)
	sb.WriteString("\n\n## Sources\n")

	for i, content := range contents {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, content.Title, content.URL))
	}

	finalResponse := sb.String()
	logger.Debugf("[SearchWeb] Final response length: %d characters", len(finalResponse))
	logger.Infof("[SearchWeb] Search and answer generation complete for query: %s", query)

	return finalResponse, nil
}

// formatSearchResults formats the search results into a readable format
func (t *GoogleSearchTool) formatSearchResults(results *GoogleSearchResponse, originalQuery string, resultCount int) string {
	logger.Debugf("[SearchWeb:Format] Formatting %d search results for query: %s",
		len(results.Items), originalQuery)

	var sb strings.Builder

	// Add header
	sb.WriteString(fmt.Sprintf("# Search Results for: %s\n\n", originalQuery))

	// Write results
	resultsFormatted := 0
	for i, item := range results.Items {
		if i >= resultCount {
			break
		}

		logger.Debugf("[SearchWeb:Format] Adding result %d: %s - %s", i+1, item.Title, item.Link)
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, item.Title))
		sb.WriteString(fmt.Sprintf("URL: %s\n", item.Link))
		sb.WriteString(fmt.Sprintf("Source: %s\n", item.DisplayLink))
		sb.WriteString(fmt.Sprintf("Description: %s\n\n", item.Snippet))
		resultsFormatted++
	}

	// Add footer
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	sb.WriteString(fmt.Sprintf("Search performed at: %s\n", currentTime))

	output := sb.String()
	logger.Infof("[SearchWeb:Format] Successfully formatted %d search results (%d characters)",
		resultsFormatted, len(output))

	return output
}

// fetchWebsiteContent fetches and extracts the text content from a website
func (t *GoogleSearchTool) fetchWebsiteContent(websiteURL string, maxChars int) (string, error) {
	logger.Debugf("[SearchWeb:Fetch] Starting content fetch from URL: %s (max chars: %d)", websiteURL, maxChars)

	// Select a random user agent
	userAgent := t.userAgents[time.Now().UnixNano()%int64(len(t.userAgents))]
	logger.Debugf("[SearchWeb:Fetch] Using User-Agent: %s", userAgent[:20]+"...")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 5 redirects
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}

			if len(via) > 0 {
				logger.Debugf("[SearchWeb:Fetch] Following redirect %d: %s → %s",
					len(via), via[0].URL.String(), req.URL.String())
			}

			// Copy headers to the redirected request
			for key, val := range via[0].Header {
				if _, ok := req.Header[key]; !ok {
					req.Header[key] = val
				}
			}
			return nil
		},
	}
	logger.Debugf("[SearchWeb:Fetch] Created HTTP client with 15s timeout")

	// Create request
	req, err := http.NewRequest("GET", websiteURL, nil)
	if err != nil {
		logger.Errorf("[SearchWeb:Fetch] Failed to create HTTP request: %v", err)
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}
	logger.Debugf("[SearchWeb:Fetch] Created HTTP request for URL: %s", websiteURL)

	// Set headers to mimic a real browser
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
	logger.Debugf("[SearchWeb:Fetch] HTTP headers set (mimicking browser request)")

	// Perform the request
	logger.Debugf("[SearchWeb:Fetch] Sending HTTP request to: %s", websiteURL)
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("[SearchWeb:Fetch] HTTP request failed: %v", err)
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()
	requestDuration := time.Since(startTime)
	logger.Debugf("[SearchWeb:Fetch] Request completed in %d ms with status: %s",
		requestDuration.Milliseconds(), resp.Status)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		logger.Warnf("[SearchWeb:Fetch] Website returned non-200 status: %s", resp.Status)
		return "", fmt.Errorf("website returned status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	logger.Debugf("[SearchWeb:Fetch] Content-Type: %s", contentType)

	// Read response body (limiting to 5MB)
	startRead := time.Now()
	logger.Debugf("[SearchWeb:Fetch] Reading response body (limit: 5MB)")
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		logger.Errorf("[SearchWeb:Fetch] Failed to read response body: %v", err)
		return "", fmt.Errorf("failed to read response body: %v", err)
	}
	readDuration := time.Since(startRead)
	logger.Debugf("[SearchWeb:Fetch] Read %d bytes in %d ms", len(bodyBytes), readDuration.Milliseconds())

	// Parse HTML with goquery
	logger.Debugf("[SearchWeb:Fetch] Parsing HTML content with goquery")
	startParse := time.Now()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		logger.Errorf("[SearchWeb:Fetch] Failed to parse HTML: %v", err)
		return "", fmt.Errorf("failed to parse HTML: %v", err)
	}
	parseDuration := time.Since(startParse)
	logger.Debugf("[SearchWeb:Fetch] HTML parsing completed in %d ms", parseDuration.Milliseconds())

	// Extract text content and clean it
	logger.Debugf("[SearchWeb:Fetch] Extracting clean text from HTML")
	startExtract := time.Now()
	textContent := t.extractCleanText(doc)
	extractDuration := time.Since(startExtract)
	logger.Debugf("[SearchWeb:Fetch] Text extraction completed in %d ms, extracted %d characters",
		extractDuration.Milliseconds(), len(textContent))

	// Truncate if necessary
	originalLength := len(textContent)
	if len(textContent) > maxChars {
		logger.Debugf("[SearchWeb:Fetch] Truncating content from %d to %d characters",
			len(textContent), maxChars)
		textContent = textContent[:maxChars] + "..."
	}

	logger.Infof("[SearchWeb:Fetch] Successfully fetched content from %s (%d/%d chars used)",
		websiteURL, len(textContent), originalLength)

	return textContent, nil
}

// extractCleanText extracts and cleans the text content from HTML
func (t *GoogleSearchTool) extractCleanText(doc *goquery.Document) string {
	// Remove script and style elements
	logger.Debugf("[SearchWeb:Extract] Removing non-content elements (scripts, styles, etc.)")
	doc.Find("script, style, noscript, iframe, header, footer, nav").Remove()

	// Find the main content
	logger.Debugf("[SearchWeb:Extract] Searching for main content area using selectors")
	var mainContent string
	mainSelectors := []string{"main", "article", "#content", ".content", "#main", ".main", ".post", ".entry", "body"}

	selectorFound := ""
	for _, selector := range mainSelectors {
		found := false
		elements := doc.Find(selector)
		elementCount := elements.Length()

		if elementCount > 0 {
			logger.Debugf("[SearchWeb:Extract] Found %d elements matching selector: %s", elementCount, selector)
		}

		elements.Each(func(i int, s *goquery.Selection) {
			if !found {
				mainContent = s.Text()
				found = true
				selectorFound = selector
			}
		})
		if found && len(mainContent) > 200 {
			logger.Debugf("[SearchWeb:Extract] Using content from selector '%s' (%d chars)",
				selectorFound, len(mainContent))
			break
		}
	}

	// If no main content found or it's too short, use the body
	if len(mainContent) < 200 {
		logger.Debugf("[SearchWeb:Extract] Main content not found or too short, using body content")
		mainContent = doc.Find("body").Text()
	}

	// Clean the content
	originalLength := len(mainContent)
	logger.Debugf("[SearchWeb:Extract] Cleaning content text (%d characters)", originalLength)
	mainContent = strings.TrimSpace(mainContent)

	// Replace consecutive whitespace with a single space
	whitespacePattern := regexp.MustCompile(`\s+`)
	mainContent = whitespacePattern.ReplaceAllString(mainContent, " ")

	logger.Debugf("[SearchWeb:Extract] Content cleaned: %d chars → %d chars",
		originalLength, len(mainContent))

	return mainContent
}

// summarizeContent uses OpenAI to summarize the website content
func (t *GoogleSearchTool) summarizeContent(content WebsiteContent, query string) (string, error) {
	logger.Debugf("[SearchWeb:Summarize] Starting summarization for content from: %s", content.URL)

	if t.openaiClient == nil {
		logger.Errorf("[SearchWeb:Summarize] OpenAI client not initialized")
		return "", fmt.Errorf("OpenAI client not initialized")
	}

	// Create context with timeout
	logger.Debugf("[SearchWeb:Summarize] Creating context with 30s timeout")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Truncate content if necessary to fit within token limits
	contentText := content.Content
	originalLength := len(contentText)
	if len(contentText) > 12000 {
		logger.Debugf("[SearchWeb:Summarize] Truncating content from %d to 12000 characters for token limit",
			len(contentText))
		contentText = contentText[:12000] + "..."
	}

	// Current date for reference
	currentDate := time.Now().Format("2006-01-02")

	logger.Debugf("[SearchWeb:Summarize] Building prompt with query: %s", query)
	prompt := fmt.Sprintf("The current date is %s. Extract the most important information from this website content to answer this question: %s\n\nWebsite: %s\nTitle: %s\n\nContent:\n%s",
		currentDate,
		query,
		content.URL,
		content.Title,
		contentText,
	)
	promptLength := len(prompt)
	logger.Debugf("[SearchWeb:Summarize] Prompt created (%d characters)", promptLength)

	logger.Debugf("[SearchWeb:Summarize] Sending summarization request to OpenAI API using model: %s",
		openai.GPT4o)
	startTime := time.Now()
	resp, err := t.openaiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a helpful assistant that extracts relevant information from website content to answer questions. Focus only on information directly related to the question.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			Temperature: 0.3,
			MaxTokens:   1000,
		},
	)

	if err != nil {
		logger.Errorf("[SearchWeb:Summarize] OpenAI API error: %v", err)
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}
	apiDuration := time.Since(startTime)
	logger.Debugf("[SearchWeb:Summarize] OpenAI API request completed in %d ms", apiDuration.Milliseconds())

	summary := strings.TrimSpace(resp.Choices[0].Message.Content)
	logger.Infof("[SearchWeb:Summarize] Successfully summarized content from %s (%d chars → %d chars)",
		content.URL, originalLength, len(summary))

	return summary, nil
}

// createFinalAnswer generates a comprehensive answer from all the summaries
func (t *GoogleSearchTool) createFinalAnswer(summaries []string, query string) (string, error) {
	logger.Debugf("[SearchWeb:Synthesize] Starting final answer synthesis from %d summaries", len(summaries))

	if t.openaiClient == nil {
		logger.Errorf("[SearchWeb:Synthesize] OpenAI client not initialized")
		return "", fmt.Errorf("OpenAI client not initialized")
	}

	// Create context with timeout
	logger.Debugf("[SearchWeb:Synthesize] Creating context with 30s timeout")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Join summaries and truncate if necessary
	totalSummaryChars := 0
	for _, summary := range summaries {
		totalSummaryChars += len(summary)
	}

	logger.Debugf("[SearchWeb:Synthesize] Joining %d summaries (total %d characters)",
		len(summaries), totalSummaryChars)
	summariesText := strings.Join(summaries, "\n\n")

	if len(summariesText) > 30000 {
		logger.Warnf("[SearchWeb:Synthesize] Summaries exceed token limit, truncating from %d to 30000 characters",
			len(summariesText))
		summariesText = summariesText[:30000] + "..."
	}

	// Current date for reference
	currentDate := time.Now().Format("2006-01-02")

	logger.Debugf("[SearchWeb:Synthesize] Building synthesis prompt with query: %s", query)
	prompt := fmt.Sprintf("The current date is %s. Based on the following information from different sources, provide a comprehensive and accurate answer to this question: %s\n\nInformation from sources:\n\n%s",
		currentDate,
		query,
		summariesText,
	)
	logger.Debugf("[SearchWeb:Synthesize] Final prompt created (%d characters)", len(prompt))

	logger.Debugf("[SearchWeb:Synthesize] Sending synthesis request to OpenAI API using model: %s",
		openai.GPT4o)
	startTime := time.Now()
	resp, err := t.openaiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a helpful assistant that synthesizes information from multiple sources to provide accurate, comprehensive answers. Mention when information is conflicting or uncertain. Do not make up information not present in the sources.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			Temperature: 0.3,
			MaxTokens:   2000,
		},
	)

	if err != nil {
		logger.Errorf("[SearchWeb:Synthesize] OpenAI API error: %v", err)
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}
	apiDuration := time.Since(startTime)
	logger.Debugf("[SearchWeb:Synthesize] OpenAI API request completed in %d ms", apiDuration.Milliseconds())

	answer := strings.TrimSpace(resp.Choices[0].Message.Content)
	logger.Infof("[SearchWeb:Synthesize] Successfully generated final answer (%d chars) from %d summaries",
		len(answer), len(summaries))

	return answer, nil
}
