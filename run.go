package creative

import "strings"

var (
	MaxIterations = 3             // maximal agents runs
	MailBoxFile   = "mailbox.out" // filename for default sdave mailbox
	ReloadMailbox bool
)

func Run(
	agents []Agent,
	input string, // user global task with unsolved tasks
) (
	output string, // result of agents work
) {
	var mailbox MailBox
	for i := range agents {
		agents[i].Name = strings.TrimSpace(agents[i].Name)
		agents[i].Role = Prompt(strings.TrimSpace(string(agents[i].Role)))
	}
	if ReloadMailbox {
		mailbox.Get(MailBoxFile)
	}
	for iter := 0; iter < MaxIterations; iter++ {
		for _, agent := range agents {
			if agent.Name == "" || agent.Role == "" {
				continue
			}
			mails := agent.Run(input, output, agents, mailbox.GetThreads(agent.Name))
			mailbox.Add(mails)
			output = mailbox.GetSolved()
			mailbox.Save(MailBoxFile)
		}
	}
	output += mailbox.GetThreads("")
	return
}
