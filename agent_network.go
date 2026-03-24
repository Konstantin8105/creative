package creative

import (
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

// Global configuration variables
var (
	MaxIterations = 3             // Maximum number of agent runs per iteration
	MailBoxFile   = "mailbox.out" // Filename for default mailbox storage
	ReloadMailbox bool            // Whether to reload mailbox from file on startup
)

//go:embed agent/*
var agentFS embed.FS

// AgentNetwork manages connections between agents and coordinates their execution
// Valid ranges:
//   - Agents: non-empty slice of Agent structs
//   - Links: 2D slice where each inner slice represents a fully connected group
type AgentNetwork struct {
	// Agents list in network
	Agents []Agent
	// Links defines fully connected agent groups
	// Principle: "If you can send to me, then I can send to you"
	// Each inner slice represents agents that can communicate with each other
	Links [][]string
	// hidden mailbox for internal message storage
	mailbox MailBox
}

// AddAgent loads an agent definition from file and adds it to the network
// filename: path to agent definition file (e.g., "agent/dreamer.md")
// The agent name is derived from the filename without extension
func (an *AgentNetwork) AddAgent(filename string) {
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
		an.Agents = append(an.Agents, Agent{
			Name: name,
			Role: Prompt(body),
		})
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

// Run executes the agent network with given input task
// input: global task description string, must be non-empty
// Returns: aggregated output from all agents or error
func (an *AgentNetwork) Run(input string) (output string, err error) {
	// Validate input
	if input == "" {
		return "", fmt.Errorf("empty input task")
	}

	// Check agent count
	if len(an.Agents) == 0 {
		return "", fmt.Errorf("empty agents list")
	}

	// Clean agent names and roles
	for i := range an.Agents {
		an.Agents[i].Name = strings.TrimSpace(an.Agents[i].Name)
		an.Agents[i].Role = Prompt(strings.TrimSpace(string(an.Agents[i].Role)))
	}

	// Clean link names with bounds checking
	for i := range an.Links {
		for j := range an.Links[i] {
			an.Links[i][j] = strings.TrimSpace(an.Links[i][j])
		}
	}

	// Validate network agents
	for _, a := range an.Agents {
		if a.Name == "" {
			return "", fmt.Errorf("agent with empty name")
		}
		if a.Role == "" {
			return "", fmt.Errorf("agent `%s` with empty role", a.Name)
		}
	}

	// Sort each link group for consistent processing
	for i := range an.Links {
		sort.Strings(an.Links[i])
	}

	// Validate network topology
	var validAgentName func(name string) bool
	{
		// Check for duplicate agent names
		var allAgentNames []string
		for _, a := range an.Agents {
			allAgentNames = append(allAgentNames, a.Name)
		}
		sort.Strings(allAgentNames)
		for i := 1; i < len(allAgentNames); i++ {
			if allAgentNames[i-1] == allAgentNames[i] {
				return "", fmt.Errorf("duplicate agent names: `%s`", allAgentNames[i])
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
		for _, linkGroup := range an.Links {
			for _, agentName := range linkGroup {
				if !validAgentName(agentName) {
					return "", fmt.Errorf("agent `%s` does not exist in link group: `%s`",
						agentName, strings.Join(linkGroup, ","))
				}
			}
		}

		// Check for duplicate agents within link groups
		for _, linkGroup := range an.Links {
			for i := 1; i < len(linkGroup); i++ {
				if linkGroup[i-1] == linkGroup[i] {
					return "", fmt.Errorf("duplicate agent `%s` in link group `%s`",
						linkGroup[i], strings.Join(linkGroup, ","))
				}
			}
		}

		// Check all agents have at least one link
		hasLinks := make(map[string]bool)
		for _, a := range an.Agents {
			hasLinks[a.Name] = false
		}
		for _, linkGroup := range an.Links {
			for _, agentName := range linkGroup {
				hasLinks[agentName] = true
			}
		}
		for agentName, linked := range hasLinks {
			if !linked {
				return "", fmt.Errorf("agent `%s` has no links", agentName)
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
			name := an.Agents[rand.Intn(len(an.Agents))].Name
			log.Printf("Change to `%s` in email: %s", name, mails[i])
			mails[i].To = name
			mails[i].From = name
		}
		an.mailbox.Add(mails, true)
		output = an.mailbox.GetSolved()
	}

	// Load colleagues for each agent
	for i := range an.Agents {
		an.Agents[i].other = an.getColleagues(an.Agents[i].Name)
	}

	// Run agent iterations
	for iter := 0; iter < MaxIterations; iter++ {
		for _, agent := range an.Agents {
			mails := agent.Run(input, output, an.mailbox.GetThreads(agent.Name), AI.GetContextSize())
			// if field `to` is not not valid then take from `from`
			for i := range mails {
				if validAgentName(mails[i].To) {
					continue
				}
				mails[i].To = mails[i].From
			}
			an.mailbox.Add(mails, false)
			output = an.mailbox.GetSolved()
			an.mailbox.Save(MailBoxFile) // Save intermediate mailbox state
		}
	}

	// Append remaining mail threads to output
	output += an.mailbox.GetThreads("")
	return
}

// getColleagues returns all colleagues for a given agent
// name: agent name to find colleagues for
// Returns: slice of Agent structs representing colleagues
func (an AgentNetwork) getColleagues(name string) (agents []Agent) {
	for _, linkGroup := range an.Links {
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
func (an AgentNetwork) getAgentByName(name string) Agent {
	for i := range an.Agents {
		if an.Agents[i].Name == name {
			return an.Agents[i]
		}
	}
	panic(fmt.Errorf("agent not found: `%s`", name))
}

// CanSend проверяет, может ли агент from отправить сообщение агенту to
// func (an *AgentNetwork) CanSend(from, to string) bool {
// 	for _, conn := range an.connections {
// 		if conn[0] == from && conn[1] == to {
// 			return true
// 		}
// 	}
// 	return false
// }

// FilterColleagues возвращает отфильтрованный список коллег для указанного агента
// В список включаются только те агенты, которым можно отправлять сообщения
// func (an *AgentNetwork) FilterColleagues(agentName string, allAgents []Agent) []Agent {
// 	var filtered []Agent
// 	for _, agent := range allAgents {
// 		if agent.Name == agentName {
// 			continue // пропускаем самого себя
// 		}
// 		if an.CanSend(agentName, agent.Name) {
// 			filtered = append(filtered, agent)
// 		}
// 	}
// 	return filtered
// }

// GetAllConnections возвращает все установленные связи
//
//	func (an *AgentNetwork) GetAllConnections() []link {
//		return an.connections
//	}
//
