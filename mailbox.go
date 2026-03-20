package creative

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// ShortMessage represents a simplified mail structure for display
// Valid ranges:
//   - From: non-empty string (sender agent name)
//   - To: non-empty string (recipient agent name)
//   - Body: string content, can be empty but not recommended
type ShortMessage struct {
	ID   int    `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Body string `json:"body"`
}

// String returns JSON representation of ShortMessage
func (s ShortMessage) String() string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Printf("mail string error: %v", err)
	}
	return string(data)
}

// Convert transforms a Mail to ShortMessage format
func Convert(m Mail) ShortMessage {
	return ShortMessage{
		ID:   m.ID,
		From: m.From,
		To:   m.To,
		Body: m.Body,
	}
}

// Mail represents an email message between agents
// Valid ranges:
//   - ID: positive integer, unique identifier
//   - From: non-empty string (sender)
//   - To: non-empty string (recipient)
//   - Body: string content
//   - Archived: boolean flag
//   - Solved: boolean flag
//   - Next: list of agents sorted by priority
//   - ReplyID: -1 for new threads, positive for replies
type Mail struct {
	// ID is unique position in mailbox
	ID       int      `json:"id"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Body     string   `json:"body"`
	Archived bool     `json:"archived"`
	Solved   bool     `json:"solved"`
	Next     []string `json:"next"` // TODO add implementation
	ReplyID  int      // -1 for new threads, ID of parent mail for replies
}

// String returns JSON representation of Mail
func (m Mail) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Printf("mail string error: %v", err)
	}
	return string(data)
}

// ParseMails extracts Mail objects from AI response text
// body: AI response string, may contain JSON arrays, single JSON objects, or code blocks
// Returns: slice of valid Mail objects and any parsing error
func ParseMails(body string) (ms []Mail, err error) {
	// Clean and validate input
	body = strings.TrimSpace(body)
	if len(body) == 0 {
		return []Mail{}, nil
	}

	originalBody := body // Keep original for error reporting

	// Remove markdown code block markers
	body = strings.ReplaceAll(body, "```json", "")
	body = strings.ReplaceAll(body, "```", "")
	body = strings.TrimSpace(body)

	// Try parsing as JSON array of mails
	var mailArray []Mail
	if err = json.Unmarshal([]byte(body), &mailArray); err == nil {
		ms = append(ms, mailArray...)
	} else {
		// Try parsing as single mail object
		var singleMail Mail
		if err = json.Unmarshal([]byte(body), &singleMail); err == nil {
			ms = append(ms, singleMail)
		} else {
			// Try extracting JSON from code blocks
			ms = extractMailsFromFragments(body)
		}
	}

	// Filter out invalid mails (empty To or Body)
	ms = filterValidMails(ms)

	log.Printf("ParseMails: extracted %d valid mails", len(ms))
	if len(ms) == 0 {
		log.Printf("ParseMails: could not parse any mails from: %s",
			shortenString(originalBody, 200))
	}

	return ms, nil
}

// extractMailsFromFragments attempts to extract mail JSON from text fragments
func extractMailsFromFragments(text string) []Mail {
	var mails []Mail

	// Look for JSON objects in the text
	start := 0
	for start < len(text) {
		// Find opening brace
		openIdx := strings.Index(text[start:], "{")
		if openIdx < 0 {
			break
		}
		openIdx += start

		// Find matching closing brace
		braceCount := 0
		closeIdx := -1
		for i := openIdx; i < len(text); i++ {
			if text[i] == '{' {
				braceCount++
			} else if text[i] == '}' {
				braceCount--
				if braceCount == 0 {
					closeIdx = i
					break
				}
			}
		}

		if closeIdx < 0 {
			break // No matching closing brace
		}

		// Extract and parse JSON
		jsonStr := text[openIdx : closeIdx+1]
		var mail Mail
		if err := json.Unmarshal([]byte(jsonStr), &mail); err == nil {
			mails = append(mails, mail)
		} else {
			// Try as array
			var mailArray []Mail
			if err := json.Unmarshal([]byte(jsonStr), &mailArray); err == nil {
				mails = append(mails, mailArray...)
			}
		}

		start = closeIdx + 1
	}

	return mails
}

// filterValidMails removes mails with empty required fields
func filterValidMails(mails []Mail) []Mail {
	var valid []Mail
	for _, mail := range mails {
		if (mail.To != "" && mail.Body != "") || // for mail criteria
			mail.Archived || mail.Solved || 0 < len(mail.Next) { // for commands
			valid = append(valid, mail)
		}
	}
	return valid
}

// shortenString truncates a string for logging
func shortenString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// MailBoxPrompt contains the prompt template for email generation
//
//go:embed mailbox.md
var MailBoxPrompt Prompt

// MailBox manages a collection of mail messages with thread support
type MailBox struct {
	presentID int    // Next available mail ID
	mails     []Mail // All mail messages
}

// Get loads mails from a JSON file
// filename: path to JSON file containing mail data
func (mb *MailBox) Get(filename string) (mails []Mail) {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("mail get error: %v", err)
		return
	}
	err = json.Unmarshal([]byte(data), &mails)
	if err != nil {
		log.Printf("mail get error: %v", err)
		return
	}
	// Reset IDs before adding
	for i := range mails {
		mails[i].ID = 0
	}
	return
}

// Save writes all mails to a JSON file
// filename: path to save JSON file
func (mb MailBox) Save(filename string) {
	data, err := json.MarshalIndent(mb.mails, "", "  ")
	if err != nil {
		log.Printf("mail save error: %v", err)
		return
	}
	err = os.WriteFile(filename, data, 0644) // Use more restrictive permissions
	if err != nil {
		log.Printf("mail save error: %v", err)
	}
}

// Add adds new mails to the mailbox, assigning IDs and managing threads
// mails: slice of Mail objects to add
func (mb *MailBox) Add(mails []Mail, addAll bool) {
	if len(mails) == 0 {
		return
	}
	{
		// solved
		for i := range mails {
			if !mails[i].Solved {
				continue
			}
			id := mails[i].ID
			for j, m := range mb.mails {
				if m.ID == id || m.ReplyID == id {
					mb.mails[j].Archived = true
					mb.mails[j].Solved = true
					log.Printf("SOLVED: %s", mb.mails[j])
				}
			}
		}
	}
	{
		// archive
		for i := range mails {
			if !mails[i].Archived {
				continue
			}
			id := mails[i].ID
			for j, m := range mb.mails {
				if m.ID == id || m.ReplyID == id {
					mb.mails[j].Archived = true
					log.Printf("ARCHIVED: %s", mb.mails[j])
				}
			}
		}
	}
	// Assign IDs and set reply relationships
	for i := range mails {
		mails[i].ReplyID = -1 // Default: new thread
		// If mail has existing ID and it's within current mailbox bounds, set as reply
		if mails[i].ID != 0 && mails[i].ID < len(mb.mails) {
			mails[i].ReplyID = mails[i].ID
		}
		// Assign new unique ID
		mails[i].ID = mb.presentID
		mb.presentID++
	}

	// Process each mail
	for _, m := range mails {
		log.Printf("Added email to mailbox: %s", m)
		if !addAll && m.Archived {
			log.Printf("ignore adding Archived mail: %s", m)
			continue
		}
		if !addAll && m.Solved {
			log.Printf("ignore adding Solved mail: %s", m)
			continue
		}
		if 0 < len(m.Next) {
			log.Printf("ignore adding Next mail: %s", m)
			continue
		}
		mb.mails = append(mb.mails, m)
	}
}

// GetThreads returns formatted mail threads for a specific agent or all agents
// agent: agent name to filter threads for, empty string returns all threads
// Returns: formatted string with mail threads in JSON code blocks
func (mb MailBox) GetThreads(agent string) (mails string) {
	// Collect relevant mails
	var relevantMails []Mail
	for _, m := range mb.mails {
		if m.Archived || m.Solved {
			continue // Skip archived or solved mails
		}
		// Filter by agent if specified
		if agent == "" || agent == m.From || agent == m.To {
			relevantMails = append(relevantMails, m)
		}
	}

	// Group mails by thread (using ReplyID for thread grouping)
	threads := make(map[int][]Mail)
	for _, mail := range relevantMails {
		threadID := mail.ReplyID
		if threadID < 0 {
			threadID = mail.ID // New thread starts with its own ID
		}
		threads[threadID] = append(threads[threadID], mail)
	}

	// Format each thread
	for threadID, threadMails := range threads {
		if threadID < 0 {
			continue // Skip invalid thread IDs
		}
		mails += fmt.Sprintf("Email thread with base email id: %d\n", threadID)

		// Convert to ShortMessage for cleaner output
		var shortMessages []ShortMessage
		for _, mail := range threadMails {
			shortMessages = append(shortMessages, Convert(mail))
		}

		// Format as JSON
		data, err := json.MarshalIndent(shortMessages, "", "  ")
		if err != nil {
			log.Printf("mail string error: %v", err)
			continue
		}

		mails += "```json\n"
		mails += string(data) + "\n"
		mails += "```\n"
	}

	return mails
}

// GetSolved returns formatted solved mails
// Returns: concatenated JSON representations of solved mails
func (mb MailBox) GetSolved() (res string) {
	// Collect relevant mails
	relevantMails := make(map[int]bool)
	for _, m := range mb.mails {
		if !m.Solved {
			continue // Skip unsolved mails
		}
		relevantMails[m.ID] = true
		if 0 <= m.ReplyID {
			relevantMails[m.ReplyID] = true
		}
	}
	for range 10 {
		for _, m := range mb.mails {
			if !m.Solved || !m.Archived {
				continue
			}
			link, ok := relevantMails[m.ID]
			if !ok {
				continue
			}
			if !link {
				continue
			}
			relevantMails[m.ID] = true
			if 0 <= m.ReplyID {
				relevantMails[m.ReplyID] = true
			}
		}
	}
	for _, m := range mb.mails {
		link, ok := relevantMails[m.ID]
		if !ok || !link {
			continue
		}
		res += Convert(m).String() + "\n"
		// res += m.Body + "\n"
	}
	return res
}
