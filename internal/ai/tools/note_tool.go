package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai/jsonschema"
	"ircbot/internal/logger"
)

// Mutex for thread-safe access to notes
var notesMutex sync.Mutex

// Notes storage file path
var notesFile = "./data/user_notes.json"

// UserNote Structure for storing notes
type UserNote struct {
	ID        string    `json:"id"`        // Unique ID for the note
	User      string    `json:"user"`      // Who saved the note
	Channel   string    `json:"channel"`   // Where the note was saved
	Note      string    `json:"note"`      // The note content
	Timestamp time.Time `json:"timestamp"` // When the note was saved
}

// NotesData is the structure for loading/saving all notes
type NotesData struct {
	Notes []UserNote `json:"notes"`
}

// SaveNoteArgs represents the arguments for saving a note
type SaveNoteArgs struct {
	Note    string `json:"note"`              // The note content to save
	User    string `json:"user,omitempty"`    // User who is saving the note
	Channel string `json:"channel,omitempty"` // Channel where the note is being saved
}

// DeleteNoteArgs represents the arguments for deleting a note
type DeleteNoteArgs struct {
	ID      string `json:"id"`                // The ID of the note to delete
	User    string `json:"user,omitempty"`    // User who is deleting the note
	Channel string `json:"channel,omitempty"` // Channel where the delete is happening
}

// ListNotesArgs represents the arguments for listing notes
type ListNotesArgs struct {
	User    string `json:"user,omitempty"`    // Filter by user (optional)
	Channel string `json:"channel,omitempty"` // Filter by channel (optional)
}

// SearchNotesArgs represents the arguments for searching notes
type SearchNotesArgs struct {
	Query   string `json:"query"`             // Search term
	User    string `json:"user,omitempty"`    // Filter by user (optional)
	Channel string `json:"channel,omitempty"` // Filter by channel (optional)
}

// NoteTools provides note taking capabilities for users
type NoteTool struct {
	BaseTool
}

// NewSaveNoteTool creates a new tool for saving notes
func NewSaveNoteTool() *NoteTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"note": {
				Type:        jsonschema.String,
				Description: "The note content to save",
			},
		},
		Required: []string{"note"},
	}

	return &NoteTool{
		BaseTool: BaseTool{
			ToolName:        "save_note",
			ToolDescription: "Save a user note that will be used as part of the system prompt in future conversations",
			ToolParameters:  params,
		},
	}
}

// NewDeleteNoteTool creates a new tool for deleting notes
func NewDeleteNoteTool() *NoteTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"id": {
				Type:        jsonschema.String,
				Description: "The ID of the note to delete",
			},
		},
		Required: []string{"id"},
	}

	return &NoteTool{
		BaseTool: BaseTool{
			ToolName:        "delete_note",
			ToolDescription: "Delete a user note by its ID",
			ToolParameters:  params,
		},
	}
}

// NewListNotesTool creates a new tool for listing notes
func NewListNotesTool() *NoteTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"user": {
				Type:        jsonschema.String,
				Description: "Filter notes by user (optional)",
			},
			"channel": {
				Type:        jsonschema.String,
				Description: "Filter notes by channel (optional)",
			},
		},
	}

	return &NoteTool{
		BaseTool: BaseTool{
			ToolName:        "list_notes",
			ToolDescription: "List all notes, optionally filtered by user or channel",
			ToolParameters:  params,
		},
	}
}

// NewSearchNotesTool creates a new tool for searching notes
func NewSearchNotesTool() *NoteTool {
	params := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"query": {
				Type:        jsonschema.String,
				Description: "The search term to find in notes",
			},
			"user": {
				Type:        jsonschema.String,
				Description: "Filter notes by user (optional)",
			},
			"channel": {
				Type:        jsonschema.String,
				Description: "Filter notes by channel (optional)",
			},
		},
		Required: []string{"query"},
	}

	return &NoteTool{
		BaseTool: BaseTool{
			ToolName:        "search_notes",
			ToolDescription: "Search for notes containing specific text, optionally filtered by user or channel",
			ToolParameters:  params,
		},
	}
}

// generateNoteID creates a unique ID for a note
func generateNoteID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// loadNotesFromFile loads all notes from the storage file
func loadNotesFromFile() ([]UserNote, error) {
	var notes []UserNote
	var notesData NotesData

	// Check if file exists
	if _, err := os.Stat(notesFile); os.IsNotExist(err) {
		// File doesn't exist, return empty notes
		return notes, nil
	}

	// Read file content
	data, err := os.ReadFile(notesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read notes file: %v", err)
	}

	// Try to parse JSON as array first (old format)
	if err := json.Unmarshal(data, &notes); err != nil {
		// Try as NotesData struct (new format)
		if err := json.Unmarshal(data, &notesData); err != nil {
			return nil, fmt.Errorf("failed to parse notes file: %v", err)
		}
		notes = notesData.Notes
	}

	return notes, nil
}

// saveNotesToFile saves all notes to the storage file
func saveNotesToFile(notes []UserNote) error {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll("./data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Marshal notes to JSON with indentation
	data, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal notes: %v", err)
	}

	// Write to file
	if err := os.WriteFile(notesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write notes file: %v", err)
	}

	return nil
}

// saveNote adds a new note to storage
func (t *NoteTool) saveNote(user, channel, noteContent string) (string, error) {
	notesMutex.Lock()
	defer notesMutex.Unlock()

	// Debug output for the parameters
	logger.Infof("SAVE NOTE DEBUG - Parameters received:")
	logger.Infof("  - User: '%s'", user)
	logger.Infof("  - Channel: '%s'", channel)
	logger.Infof("  - Content: '%s'", noteContent)

	// Validate note content
	noteContent = strings.TrimSpace(noteContent)
	if noteContent == "" {
		return "", fmt.Errorf("note content cannot be empty")
	}

	// Load existing notes
	notes, err := loadNotesFromFile()
	if err != nil {
		return "", err
	}

	// Create new note
	newNote := UserNote{
		ID:        generateNoteID(),
		User:      user,
		Channel:   channel,
		Note:      noteContent,
		Timestamp: time.Now(),
	}

	// Debug the note being saved
	noteBytes, _ := json.MarshalIndent(newNote, "", "  ")
	logger.Infof("New note being saved: %s", string(noteBytes))

	// Add to notes collection
	notes = append(notes, newNote)

	// Save back to file
	if err := saveNotesToFile(notes); err != nil {
		return "", err
	}

	logger.Infof("Note saved for user %s in channel %s: %s", user, channel, noteContent)
	return fmt.Sprintf("Note saved with ID: %s", newNote.ID), nil
}

// deleteNote removes a note by ID
func (t *NoteTool) deleteNote(user, noteID string) (string, error) {
	notesMutex.Lock()
	defer notesMutex.Unlock()

	// Load existing notes
	notes, err := loadNotesFromFile()
	if err != nil {
		return "", err
	}

	// Find and remove note
	found := false
	var filteredNotes []UserNote

	for _, note := range notes {
		if note.ID == noteID {
			// Skip this note (delete it)
			found = true
			// Check if the user owns the note
			if note.User != user {
				return "", fmt.Errorf("you can only delete your own notes")
			}
		} else {
			filteredNotes = append(filteredNotes, note)
		}
	}

	if !found {
		return "", fmt.Errorf("note with ID %s not found", noteID)
	}

	// Save back to file
	if err := saveNotesToFile(filteredNotes); err != nil {
		return "", err
	}

	logger.Infof("Note with ID %s deleted by user %s", noteID, user)
	return fmt.Sprintf("Note with ID %s successfully deleted", noteID), nil
}

// listNotes returns a list of notes, optionally filtered
func (t *NoteTool) listNotes(user, channel string) (string, error) {
	notesMutex.Lock()
	defer notesMutex.Unlock()

	// Load notes
	notes, err := loadNotesFromFile()
	if err != nil {
		return "", err
	}

	// Filter notes
	var filteredNotes []UserNote
	for _, note := range notes {
		// Apply filters if provided
		if (user == "" || note.User == user) && (channel == "" || note.Channel == channel) {
			filteredNotes = append(filteredNotes, note)
		}
	}

	// No notes found
	if len(filteredNotes) == 0 {
		return "No notes found", nil
	}

	// Format output
	var result strings.Builder
	
	// For IRC display, show a simplified response with just one note
	if len(filteredNotes) == 1 {
		note := filteredNotes[0]
		result.WriteString(fmt.Sprintf("%s, här är din anteckning: \"%s\"\n", user, note.Note))
		result.WriteString(fmt.Sprintf("(ID: %s - To delete, use: !note delete %s)", note.ID, note.ID))
		return result.String(), nil
	}
	
	// For multiple notes, show detailed listing
	result.WriteString(fmt.Sprintf("Found %d notes:\n\n", len(filteredNotes)))

	for _, note := range filteredNotes {
		dateStr := note.Timestamp.Format("2006-01-02 15:04")
		result.WriteString(fmt.Sprintf("ID: %s | User: %s | Date: %s\n", note.ID, note.User, dateStr))
		result.WriteString(fmt.Sprintf("Content: %s\n", note.Note))
		result.WriteString(fmt.Sprintf("To delete: !note delete %s\n\n", note.ID))
	}

	return result.String(), nil
}

// searchNotes searches for notes containing specific text
func (t *NoteTool) searchNotes(query, user, channel string) (string, error) {
	notesMutex.Lock()
	defer notesMutex.Unlock()

	// Validate query
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return "", fmt.Errorf("search query cannot be empty")
	}

	// Load notes
	notes, err := loadNotesFromFile()
	if err != nil {
		return "", err
	}

	// Search and filter notes
	var matchedNotes []UserNote
	for _, note := range notes {
		// Check if note matches search criteria
		if strings.Contains(strings.ToLower(note.Note), query) {
			// Apply additional filters if provided
			if (user == "" || note.User == user) && (channel == "" || note.Channel == channel) {
				matchedNotes = append(matchedNotes, note)
			}
		}
	}

	// No matches found
	if len(matchedNotes) == 0 {
		return fmt.Sprintf("No notes found matching query: %s", query), nil
	}

	// Format output
	var result strings.Builder
	
	// For IRC display, show a simplified response with just one note
	if len(matchedNotes) == 1 {
		note := matchedNotes[0]
		result.WriteString(fmt.Sprintf("Found note matching '%s': \"%s\"\n", query, note.Note))
		result.WriteString(fmt.Sprintf("(ID: %s - To delete, use: !note delete %s)", note.ID, note.ID))
		return result.String(), nil
	}
	
	// For multiple notes, show detailed listing
	result.WriteString(fmt.Sprintf("Found %d notes matching '%s':\n\n", len(matchedNotes), query))

	for _, note := range matchedNotes {
		dateStr := note.Timestamp.Format("2006-01-02 15:04")
		result.WriteString(fmt.Sprintf("ID: %s | User: %s | Date: %s\n", note.ID, note.User, dateStr))
		result.WriteString(fmt.Sprintf("Content: %s\n", note.Note))
		result.WriteString(fmt.Sprintf("To delete: !note delete %s\n\n", note.ID))
	}

	return result.String(), nil
}

// Execute runs the appropriate note operation based on tool name
func (t *NoteTool) Execute(args string) (string, error) {
	// Extract current operation from tool name
	operation := t.Name()

	logger.Debugf("Executing note tool operation: %s with args: %s", operation, args)

	switch operation {
	case "save_note":
		var params SaveNoteArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			logger.Errorf("Failed to unmarshal save_note args: %v, args: %s", err, args)
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		
		// Use provided user and channel or fallback to empty
		user := params.User
		channel := params.Channel
		
		// Debug log the parameters
		logger.Infof("SAVE NOTE PARAMETERS - User: '%s', Channel: '%s', Note: '%s'", 
			user, channel, params.Note)
		
		if user == "" {
			logger.Warnf("No user provided in save_note args, notes may not be properly tied to users")
		}
		
		return t.saveNote(user, channel, params.Note)

	case "delete_note":
		var params DeleteNoteArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		
		// Use provided user or fallback to empty
		user := params.User
		if user == "" {
			logger.Warnf("No user provided in delete_note args, permission check may fail")
		}
		
		return t.deleteNote(user, params.ID)

	case "list_notes":
		var params ListNotesArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		
		// Use provided user/channel filters
		return t.listNotes(params.User, params.Channel)

	case "search_notes":
		var params SearchNotesArgs
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		
		// Use provided user/channel filters
		return t.searchNotes(params.Query, params.User, params.Channel)

	default:
		return "", fmt.Errorf("unknown note operation: %s", operation)
	}
}

// GetUserNotes retrieves all notes for a specific user
// This function can be used by the AI system to include user notes in the context
func GetUserNotes(user string) string {
	notesMutex.Lock()
	defer notesMutex.Unlock()

	// Load notes
	notes, err := loadNotesFromFile()
	if err != nil {
		logger.Errorf("Failed to load notes: %v", err)
		return ""
	}

	// Filter notes for this user
	var userNotes []UserNote
	for _, note := range notes {
		if note.User == user {
			userNotes = append(userNotes, note)
		}
	}

	// No notes found
	if len(userNotes) == 0 {
		return ""
	}

	// Format output for system prompt inclusion
	var result strings.Builder
	result.WriteString("User-specific context from notes:\n")

	for _, note := range userNotes {
		result.WriteString(fmt.Sprintf("- %s\n", note.Note))
	}

	return result.String()
}