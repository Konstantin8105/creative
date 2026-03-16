package creative

import (
	"embed"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

var (
	MaxIterations = 3             // maximal agents runs
	MailBoxFile   = "mailbox.out" // filename for default sdave mailbox
	ReloadMailbox bool
)

//go:embed agent/*
var agentFS embed.FS

// AgentNetwork управляет связями между агентами
type AgentNetwork struct {
	// Agents list in network
	Agents []Agent
	// Links is connected agents
	// principe: "If you send to me, then I can send to you"
	Links [][]string
	// hidden mail box
	mailbox MailBox
}

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
	// check embedded folder
	bodyEmbed, errEmbed := agentFS.ReadFile(filename)
	if errEmbed == nil {
		add(filename, string(bodyEmbed))
		return
	}
	// check file system
	bodyFs, errFs := os.ReadFile(filename)
	if errFs == nil {
		add(filename, string(bodyFs))
	}
	// error handling
	if errEmbed == nil && errFs == nil {
		panic("both files are exist")
	}
	if errEmbed != nil && errFs != nil {
		panic(fmt.Errorf("filename angent is not found: `%s`", filename))
	}
}

// Run выполняет запуск агентов с использованием сети связей
func (an *AgentNetwork) Run(
	// user global task with unsolved tasks
	input string,
) (
	output string, // result of agents work
	err error, // error handling
) {
	// check amount agents
	if len(an.Agents) == 0 {
		err = fmt.Errorf("empty agents list")
		return
	}
	// clearing agents
	for i := range an.Agents {
		an.Agents[i].Name = strings.TrimSpace(an.Agents[i].Name)
		an.Agents[i].Role = Prompt(strings.TrimSpace(string(an.Agents[i].Role)))
	}
	// cleaning agents links in links
	for i := range an.Links {
		for j := range an.Links {
			an.Links[i][j] = strings.TrimSpace(an.Links[i][j])
		}
	}
	// check network agents
	for _, a := range an.Agents {
		if a.Name == "" {
			err = fmt.Errorf("agent with empty name")
			return
		}
		if a.Role == "" {
			err = fmt.Errorf("agent `%s` with empty role", a.Name)
			return
		}
	}
	// sort each links
	for i := range an.Links {
		sort.Strings(an.Links[i])
	}
	// check network agent links
	{
		// not same agents names
		var aan []string // all agent names
		for _, a := range an.Agents {
			aan = append(aan, a.Name)
		}
		sort.Strings(aan)
		for i := 1; i < len(aan); i++ {
			if aan[i-1] != aan[i] {
				continue
			}
			err = fmt.Errorf("Same agent names: `%s`", aan[i])
			return
		}
		exist := func(name string) bool {
			for _, n := range aan {
				if n == name {
					return true
				}
			}
			return false
		}
		// check all agent exist
		for _, ls := range an.Links {
			for _, a := range ls {
				if exist(a) {
					continue
				}
				err = fmt.Errorf("agent `%s` not exist in link: `%s` in list `%s`",
					a, strings.Join(ls, ","), strings.Join(aan, ","))
				return
			}
		}
		// check self-links
		for _, ls := range an.Links {
			for i := 1; i < len(ls); i++ {
				if ls[i-1] == ls[i] {
					err = fmt.Errorf("dublicate of agent `%s` in `%s`",
						ls[i], ls)
					return
				}
			}
		}
		// check all links
		fullLinks := map[string]bool{}
		for _, a := range an.Agents {
			fullLinks[a.Name] = false
		}
		for _, ls := range an.Links {
			for _, l := range ls {
				fullLinks[l] = true
			}
		}
		for a, v := range fullLinks {
			if v {
				continue
			}
			err = fmt.Errorf("agent `%s` have not links", a)
			return
		}
	}
	// reload mail box for avoid lose data
	if ReloadMailbox {
		an.mailbox.Get(MailBoxFile)
	}
	// load colleagues for each agent
	for i := range an.Agents {
		an.Agents[i].other = an.getColleguase(an.Agents[i].Name)
	}
	// run agent action
	for iter := 0; iter < MaxIterations; iter++ {
		for _, agent := range an.Agents {
			mails := agent.Run(input, output, an.mailbox.GetThreads(agent.Name))
			an.mailbox.Add(mails)
			output = an.mailbox.GetSolved()
			an.mailbox.Save(MailBoxFile) // save intermediant mailbox
		}
	}
	output += an.mailbox.GetThreads("")
	return
}

func (an AgentNetwork) getColleguase(name string) (agents []Agent) {
	for _, ls := range an.Links {
		if !slices.Contains(ls, name) {
			continue
		}
		for _, l := range ls {
			if l == name {
				continue
			}
			agents = append(agents, an.getAgentByName(l))
		}
	}
	return
}

func (an AgentNetwork) getAgentByName(name string) Agent {
	for i := range an.Agents {
		if an.Agents[i].Name == name {
			return an.Agents[i]
		}
	}
	panic(fmt.Errorf("not found `%s`", name))
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
