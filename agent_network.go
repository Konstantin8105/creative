package creative

import (
	"bytes"
	"embed"
	_ "embed"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ConflictPrompt contains the prompt template for conflict solving
//
//go:embed conflict.md
var ConflictPrompt Prompt

// Global configuration variables
var (
	MailBoxFile   = "mailbox.out" // Filename for default mailbox storage
	ReloadMailbox bool            // Whether to reload mailbox from file on startup
)

func NewMailNetwork(ai AIrunner) *MailNetwork {
	mn := new(MailNetwork)
	mn.ai = ai
	return mn
}

// MailNetwork manages connections between agents and coordinates their execution
// Valid ranges:
//   - Agents: non-empty slice of Agent structs
//   - Links: 2D slice where each inner slice represents a fully connected group
type MailNetwork struct {
	ai     AIrunner
	system []string
	// Agents list in network
	agents []*AgentMailBox
	// Links defines fully connected agent groups
	// Principle: "If you can send to me, then I can send to you"
	// Each inner slice represents agents that can communicate with each other
	links [][]string
	// hidden mailbox for internal message storage
	mailbox MailBox
}

//go:embed agent/*
var agentFS embed.FS

// AddAgent loads an agent definition from file and adds it to the network
// filename: path to agent definition file (e.g., "agent/dreamer.md")
// The agent name is derived from the filename without extension
func (an *MailNetwork) AddAgent(filename string, mp MailBoxPermission) {
	add := func(filename string, body string) {
		name := filename
		{
			index := strings.LastIndex(name, string(filepath.Separator))
			index += len(string(filepath.Separator))
			name = name[index:]
			index = strings.LastIndex(name, ".")
			if 0 < index {
				name = name[:index]
			}
		}
		an.agents = append(an.agents,
			NewAgentMailBox(an.ai, name, Prompt(body), &an.mailbox, mp),
		)
	}
	// Check embedded file system first
	bodyEmbed, errEmbed := agentFS.ReadFile(filename)
	if errEmbed == nil {
		add(filename, string(bodyEmbed))
		return
	}
	// Check regular file system
	bodyFs, errFs := os.ReadFile(filename)
	if errFs == nil {
		add(filename, string(bodyFs))
		return
	}
	// Error handling - neither file exists
	panic(fmt.Errorf("agent file not found: `%s`", filename))
}

func (an *MailNetwork) AddLinks(links []string) {
	if len(links) == 0 {
		panic("empty links")
	}
	for i := range links {
		if links[i] != strings.TrimSpace(links[i]) {
			panic("extra spaces in name not acceptable")
		}
		if links[i] == "" {
			panic("empty links")
		}
	}
	an.links = append(an.links, links)
}

func (an *MailNetwork) AddSystem(system ...string) {
	an.system = append(an.system, system...)
}

// Run executes the agent network with given input task
// input: global task description string, must be non-empty
// Returns: aggregated output from all agents or error
func (an *MailNetwork) Run(MaxIterations int) (err error) {
	// Check agent count
	if len(an.agents) == 0 {
		err = fmt.Errorf("empty agents list")
		return
	}

	// Sort each link group for consistent processing
	for i := range an.links {
		sort.Strings(an.links[i])
	}

	// Validate network topology
	var validAgentName func(name string) bool
	{
		// Check for duplicate agent names
		var allAgentNames []string
		for _, a := range an.agents {
			allAgentNames = append(allAgentNames, a.internal.name)
		}
		sort.Strings(allAgentNames)
		for i := 1; i < len(allAgentNames); i++ {
			if allAgentNames[i-1] == allAgentNames[i] {
				err = fmt.Errorf("duplicate agent names: `%s`", allAgentNames[i])
				return
			}
		}

		// Helper to check if agent exists
		validAgentName = func(name string) bool {
			for _, n := range allAgentNames {
				if n == name {
					return true
				}
			}
			return false
		}

		// Check all agents in links exist
		for _, linkGroup := range an.links {
			for _, agentName := range linkGroup {
				if !validAgentName(agentName) {
					err = fmt.Errorf("agent `%s` does not exist in link group: `%s`",
						agentName, strings.Join(linkGroup, ","))
					return
				}
			}
		}

		// Check for duplicate agents within link groups
		for _, linkGroup := range an.links {
			for i := 1; i < len(linkGroup); i++ {
				if linkGroup[i-1] == linkGroup[i] {
					err = fmt.Errorf("duplicate agent `%s` in link group `%s`",
						linkGroup[i], strings.Join(linkGroup, ","))
					return
				}
			}
		}

		// Check all agents have at least one link
		hasLinks := make(map[string]bool)
		for _, a := range an.agents {
			hasLinks[a.internal.name] = false
		}
		for _, linkGroup := range an.links {
			for _, agentName := range linkGroup {
				hasLinks[agentName] = true
			}
		}
		for agentName, linked := range hasLinks {
			if !linked {
				err = fmt.Errorf("agent `%s` has no links", agentName)
				return
			}
		}
	}

	// Reload mailbox if configured
	if ReloadMailbox {
		mails := an.mailbox.Get(MailBoxFile)
		// if email have wrong "To" or "From", then change to any acceptable
		// agent in network
		for i := range mails {
			if validAgentName(mails[i].To) && validAgentName(mails[i].From) {
				continue
			}
			// self-note to any agent
			name := an.agents[rand.Intn(len(an.agents))].internal.name
			log.Printf("Change to `%s` in email: %s", name, mails[i])
			mails[i].To = name
			mails[i].From = name
		}
		an.mailbox.Add(mails, true, an.agents[0].internal.name)
	}

	// Run agent iterations
	for range MaxIterations {
		for _, agent := range an.agents {
			agent.internal.Reset()
			agent.Init()
			// Add colleague descriptions
			for _, c := range an.getColleagues(agent.internal.name) {
				if c.internal.name == agent.internal.name {
					continue // Skip self-reference
				}
				var buf bytes.Buffer
				fmt.Fprintf(&buf, "Описание роли твоего коллеги по имени: `%s`\n", c.internal.name)
				fmt.Fprintf(&buf, "%s\n", string(c.internal.role))
				fmt.Fprintf(&buf, "Окончание описания роли `%s`\n", c.internal.name)
				fmt.Fprintf(&buf, "\n")
				agent.internal.AddSystem(buf.String())
			}
			// Add conflict prompt
			agent.internal.AddSystem(string(ConflictPrompt))
			// run
			err = agent.Run()
			if err != nil {
				return
			}
			an.mailbox.Save(MailBoxFile) // Save intermediate mailbox state
		}
	}
	return
}

// getColleagues returns all colleagues for a given agent
// name: agent name to find colleagues for
// Returns: slice of Agent structs representing colleagues
func (an MailNetwork) getColleagues(name string) (agents []*AgentMailBox) {
	for _, linkGroup := range an.links {
		if !slices.Contains(linkGroup, name) {
			continue
		}
		for _, colleagueName := range linkGroup {
			if colleagueName == name {
				continue // Skip self
			}
			agents = append(agents, an.getAgentByName(colleagueName))
		}
	}
	return
}

// getAgentByName finds an agent by name in the network
// name: agent name to search for
// Returns: Agent struct or panics if not found
func (an MailNetwork) getAgentByName(name string) *AgentMailBox {
	for i := range an.agents {
		if an.agents[i].internal.name == name {
			return an.agents[i]
		}
	}
	panic(fmt.Errorf("agent not found: `%s`", name))
}
