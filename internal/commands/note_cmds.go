package commands

import (
	"encoding/json"
	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/ai/tools"
	"ircbot/internal/logger"
	"regexp"
	"strings"
)

// noteCommand handles the !note command to create, list, search, or delete notes
func noteCommand(c *irc.Client, m *irc.Message, args []string) {
	// If no arguments provided, show usage
	if len(args) == 0 {
		showNoteUsage(c, m)
		return
	}

	// Get user and channel information
	nick := m.Prefix.Name
	replyTarget := m.Params[0]

	// Log detailed information about message source
	logger.Infof("IRC Message details:")
	logger.Infof("  - Prefix: %s", m.Prefix)
	logger.Infof("  - Nick: %s", nick)
	logger.Infof("  - Command: %s", m.Command)
	logger.Infof("  - Params: %v", m.Params)
	logger.Infof("  - ReplyTarget: %s", replyTarget)
	logger.Infof("  - CurrentNick: %s", c.CurrentNick())

	if replyTarget == c.CurrentNick() {
		replyTarget = nick
		logger.Infof("  - Adjusted replyTarget to: %s", replyTarget)
	}

	// Parse the subcommand
	subCmd := strings.ToLower(args[0])

	switch subCmd {
	case "add", "save":
		handleAddNote(c, m, args[1:], nick, replyTarget)

	case "list":
		handleListNotes(c, m, args[1:], nick, replyTarget)

	case "search":
		handleSearchNotes(c, m, args[1:], nick, replyTarget)

	case "delete", "remove":
		handleDeleteNote(c, m, args[1:], nick, replyTarget)

	case "help":
		showNoteUsage(c, m)

	default:
		// If the first argument isn't a subcommand, assume it's part of a note to add
		handleAddNote(c, m, args, nick, replyTarget)
	}
}

// showNoteUsage displays usage information for the note command
func showNoteUsage(c *irc.Client, m *irc.Message) {
	replyTarget := m.Params[0]
	if replyTarget == c.CurrentNick() {
		replyTarget = m.Prefix.Name
	}

	usage := []string{
		"Note command usage:",
		"!note add <text> - Add a new personal note",
		"!note list - List all your notes",
		"!note search <query> - Search for notes containing specific text",
		"!note delete <id> - Delete a note by its ID",
		"!note help - Show this help message",
	}

	// Send usage information
	for _, line := range usage {
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, line)
	}
}

// handleAddNote processes the request to add a new note
func handleAddNote(c *irc.Client, m *irc.Message, args []string, nick, replyTarget string) {
	if len(args) == 0 {
		c.Writef("%s %s :Please provide note content", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	// Join the remaining arguments to form the note content
	noteContent := strings.Join(args, " ")

	// Log information about the note being added
	logger.Infof("Adding note for user '%s' in channel '%s': '%s'", nick, replyTarget, noteContent)

	// Marshall the arguments properly as JSON with explicit type assignments
	argsMap := map[string]interface{}{
		"note":    noteContent,
		"user":    nick,
		"channel": replyTarget,
	}

	argsBytes, err := json.Marshal(argsMap)
	if err != nil {
		logger.Errorf("Error marshalling note args: %v", err)
		c.Writef("%s %s :Error processing note: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// Get note tool from registry
	registry := tools.GetRegistry()

	// Execute the save_note tool
	toolResponse, err := registry.ExecuteTool("save_note", string(argsBytes))
	if err != nil {
		logger.Errorf("Error executing save_note tool: %v", err)
		c.Writef("%s %s :Error saving note: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// Send success message
	c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, toolResponse)
}

// handleListNotes processes the request to list notes
func handleListNotes(c *irc.Client, m *irc.Message, args []string, nick, replyTarget string) {
	// Marshall the arguments properly as JSON with explicit type assignments
	argsMap := map[string]interface{}{
		"user":    nick,
		"channel": replyTarget,
	}

	argsBytes, err := json.Marshal(argsMap)
	if err != nil {
		logger.Errorf("Error marshalling list args: %v", err)
		c.Writef("%s %s :Error processing command: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// Get note tool from registry
	registry := tools.GetRegistry()

	// Execute the list_notes tool
	toolResponse, err := registry.ExecuteTool("list_notes", string(argsBytes))
	if err != nil {
		logger.Errorf("Error executing list_notes tool: %v", err)
		c.Writef("%s %s :Error listing notes: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// If the response is short, send directly
	if len(toolResponse) < 200 {
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, toolResponse)
		return
	}

	// For longer responses, use paste service
	pasteURL, err := PasteService(toolResponse)
	if err != nil {
		// If paste service fails, send a truncated response
		if len(toolResponse) > 400 {
			toolResponse = toolResponse[:397] + "..."
		}
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, toolResponse)
	} else {
		c.Writef("%s %s :Your notes are available at: %s", internal.CMD_PRIVMSG, replyTarget, pasteURL)
	}
}

// handleSearchNotes processes the request to search notes
func handleSearchNotes(c *irc.Client, m *irc.Message, args []string, nick, replyTarget string) {
	if len(args) == 0 {
		c.Writef("%s %s :Please provide a search query", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	// Join the remaining arguments to form the search query
	query := strings.Join(args, " ")

	// Marshall the arguments properly as JSON with explicit type assignments
	argsMap := map[string]interface{}{
		"query":   query,
		"user":    nick,
		"channel": replyTarget,
	}

	argsBytes, err := json.Marshal(argsMap)
	if err != nil {
		logger.Errorf("Error marshalling search args: %v", err)
		c.Writef("%s %s :Error processing search: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// Get note tool from registry
	registry := tools.GetRegistry()

	// Execute the search_notes tool
	toolResponse, err := registry.ExecuteTool("search_notes", string(argsBytes))
	if err != nil {
		logger.Errorf("Error executing search_notes tool: %v", err)
		c.Writef("%s %s :Error searching notes: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// If the response is short, send directly
	if len(toolResponse) < 200 {
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, toolResponse)
		return
	}

	// For longer responses, use paste service
	pasteURL, err := PasteService(toolResponse)
	if err != nil {
		// If paste service fails, send a truncated response
		if len(toolResponse) > 400 {
			toolResponse = toolResponse[:397] + "..."
		}
		c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, toolResponse)
	} else {
		c.Writef("%s %s :Search results are available at: %s", internal.CMD_PRIVMSG, replyTarget, pasteURL)
	}
}

// handleDeleteNote processes the request to delete a note
func handleDeleteNote(c *irc.Client, m *irc.Message, args []string, nick, replyTarget string) {
	if len(args) == 0 {
		c.Writef("%s %s :Please provide a note ID to delete", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	// First, list all notes to show the user what's available
	listArgsMap := map[string]interface{}{
		"user":    nick,
		"channel": replyTarget,
	}

	listArgsBytes, err := json.Marshal(listArgsMap)
	if err != nil {
		logger.Errorf("Error marshalling list args: %v", err)
		c.Writef("%s %s :Error listing notes: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// List all notes
	registry := tools.GetRegistry()
	listResult, err := registry.ExecuteTool("list_notes", string(listArgsBytes))
	if err != nil {
		logger.Errorf("Error listing notes: %v", err)
		c.Writef("%s %s :Error listing notes: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// If no notes found, inform the user
	if strings.Contains(listResult, "No notes found") {
		c.Writef("%s %s :You don't have any notes to delete.", internal.CMD_PRIVMSG, replyTarget)
		return
	}

	// Send the list of notes to the user first
	c.Writef("%s %s :Your notes: %s", internal.CMD_PRIVMSG, replyTarget, listResult)

	// Get the note ID
	noteID := args[0]

	// If the ID doesn't look like an ID, assume it's content and search for the note
	if !strings.HasPrefix(noteID, "17") && len(noteID) < 15 {
		// Try to find the note by content instead
		searchArgsMap := map[string]interface{}{
			"query":   noteID,
			"user":    nick,
			"channel": replyTarget,
		}

		searchArgsBytes, err := json.Marshal(searchArgsMap)
		if err != nil {
			logger.Errorf("Error marshalling search args: %v", err)
			c.Writef("%s %s :Error processing search: %v", internal.CMD_PRIVMSG, replyTarget, err)
			return
		}

		// Search for the note
		searchResult, err := registry.ExecuteTool("search_notes", string(searchArgsBytes))
		if err != nil {
			logger.Errorf("Error searching for note to delete: %v", err)
			c.Writef("%s %s :Error finding note: %v", internal.CMD_PRIVMSG, replyTarget, err)
			return
		}

		// Try to extract ID from search result
		if strings.Contains(searchResult, "ID:") {
			idLine := regexp.MustCompile(`ID:\s*(\d+)`).FindStringSubmatch(searchResult)
			if len(idLine) > 1 {
				extractedID := idLine[1]
				logger.Infof("Found note ID %s matching content: %s", extractedID, noteID)
				noteID = extractedID

				// Tell the user which note will be deleted
				c.Writef("%s %s :Found note with ID %s matching '%s'",
					internal.CMD_PRIVMSG, replyTarget, extractedID, noteID)
			} else {
				c.Writef("%s %s :Couldn't find note ID for content: %s", internal.CMD_PRIVMSG, replyTarget, noteID)
				return
			}
		} else if strings.Contains(searchResult, "No notes found") {
			c.Writef("%s %s :No notes found matching: %s", internal.CMD_PRIVMSG, replyTarget, noteID)
			return
		}
	}

	// Marshall the arguments properly as JSON with explicit type assignments
	argsMap := map[string]interface{}{
		"id":      noteID,
		"user":    nick,
		"channel": replyTarget,
	}

	argsBytes, err := json.Marshal(argsMap)
	if err != nil {
		logger.Errorf("Error marshalling delete args: %v", err)
		c.Writef("%s %s :Error processing delete: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// Get note tool from registry
	registry = tools.GetRegistry()

	// Execute the delete_note tool
	toolResponse, err := registry.ExecuteTool("delete_note", string(argsBytes))
	if err != nil {
		logger.Errorf("Error executing delete_note tool: %v", err)
		c.Writef("%s %s :Error deleting note: %v", internal.CMD_PRIVMSG, replyTarget, err)
		return
	}

	// Send success message
	c.Writef("%s %s :%s", internal.CMD_PRIVMSG, replyTarget, toolResponse)
}
