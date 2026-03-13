package creative

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Prompt string

type Agent struct {
	Name  string // short name of agent
	Role  Prompt
	Other []Agent
}

func AgentFile(filename string) (a Agent) {
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
	a.Name = name
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Errorf("Error reading input file: %v", err))
	}
	a.Role = Prompt(string(data))
	return
}

func (a Agent) Run(input, output string, colleguase []Agent, mails string) (results []Mail) {
	log.Printf("Run agent: %s", a.Name)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Описание твоей имени\n")
	fmt.Fprintf(&buf, "%s\n", string(a.Name))
	fmt.Fprintf(&buf, "Окончание описания твоей имени\n")
	fmt.Fprintf(&buf, "Описание твоей роли\n")
	fmt.Fprintf(&buf, "%s\n", string(a.Role))
	fmt.Fprintf(&buf, "Окончание описания твоей роли\n")
	for _, c := range colleguase {
		if c.Name == a.Name {
			continue
		}
		fmt.Fprintf(&buf, "Описание роли твоего коллеги по имени: `%s`\n", c.Name)
		fmt.Fprintf(&buf, "%s\n", string(c.Role))
		fmt.Fprintf(&buf, "Окончание описания роли `%s`\n", c.Name)
	}
	fmt.Fprintf(&buf, "Общая задача, которую решается с точки зрения твоей роли\n")
	fmt.Fprintf(&buf, "%s\n", input)
	fmt.Fprintf(&buf, "Окончание описания общей задачи\n")
	if output != "" {
		fmt.Fprintf(&buf, "Достигнутые договоренности\n")
		fmt.Fprintf(&buf, "%s\n", output)
		fmt.Fprintf(&buf, "Окончание описания достигнутых договоренности\n")
	}
	if 0 < len(mails) {
		fmt.Fprintf(&buf, "Твой почтовый ящик\n")
		fmt.Fprintf(&buf, "%s\n", mails)
		log.Printf("Mails of agent %s: %s", a.Name, mails)
		fmt.Fprintf(&buf, "Окончание твоего почтового ящика\n")
	}
	fmt.Fprintf(&buf, "%s\n", MailBoxPrompt) // написание писем

	// запуск агента
	if AI == nil {
		panic(fmt.Errorf("empty AI"))
	}
	request := buf.String()
	log.Printf("agent `%s` request: %d", a.Name, len(request))
	responce, err := AI.Run(request)
	log.Printf("agent `%s` response: %s. Error = %v", a.Name, responce, err)
	if err != nil {
		log.Printf("Error of running %s: `%v`", a.Name, err)
		return
	}
	results, err = ParseMails(responce)
	if err != nil {
		log.Printf("Error of parse mails %s: `%v`", a.Name, err)
		return
	}
	for i := range results {
		results[i].From = a.Name
	}
	return
}
