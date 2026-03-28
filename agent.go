package creative

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
)

var MaxAgentIterations = 10

// TODO
const (
	ContinueDisscussion Prompt = "Продолжай"
	FinishDisscussion   Prompt = "Пока писать нечего и работа агента закончена"
)

func NewAgent(prv AIrunner, name string, role Prompt) *Agent {
	// validate
	if prv == nil {
		panic(fmt.Errorf("ai provider is nil"))
	}
	if name == "" {
		panic(fmt.Errorf("empty name of agent"))
	}
	if strings.Contains(name, " ") {
		panic(fmt.Errorf("name have space `%s`", name))
	}
	if name != strings.TrimSpace(name) {
		panic(fmt.Errorf("extra space in name of agent `%s`", name))
	}
	if role == "" {
		panic(fmt.Errorf("empty role of agent `%s`", name))
	}
	if role != Prompt(strings.TrimSpace(string(role))) {
		panic(fmt.Errorf("extra space in role of agent `%s`", name))
	}
	// create
	agent := new(Agent)
	agent.chat = NewChat(prv)
	agent.name = name
	agent.role = role
	return agent
}

type Agent struct {
	chat *Chat

	name string
	role Prompt
}

func (a Agent) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "name: %s\n", a.name)
	fmt.Fprintf(&buf, "role: %s\n", a.role)
	fmt.Fprintf(&buf, "%s", a.chat.String())
	return buf.String()
}

func (a *Agent) Init() {
	var buf bytes.Buffer
	// name
	fmt.Fprintf(&buf, "Описание твоей имени\n")
	fmt.Fprintf(&buf, "%s\n", string(a.name))
	fmt.Fprintf(&buf, "Окончание описания твоей имени\n")
	fmt.Fprintf(&buf, "\n")
	// role
	fmt.Fprintf(&buf, "Описание твоей роли\n")
	fmt.Fprintf(&buf, "%s\n", string(a.role))
	fmt.Fprintf(&buf, "Окончание описания твоей роли\n")
	fmt.Fprintf(&buf, "\n")
	// end of discussion
	a.chat.AddSystem(buf.String())

	a.chat.AddSystem(fmt.Sprintf(`

Агент выполняет работу на основании первого запроса, дальнейшие сообщения к агенту (к примеру: "%s") будут предназначены для продолжения работы агента. Если работа агента выполнена, то агент должен вывести фразу "%s".

`, ContinueDisscussion, FinishDisscussion))
}

func (a *Agent) AddSystem(system ...string) {
	a.chat.AddSystem(system...)
}

func (a *Agent) Reset() {
	a.chat.system = nil
	a.chat.msgs = nil
}

func isFinish(resp string) bool {
	if strings.Contains(resp, string(FinishDisscussion)) {
		return true
	}
	fs := strings.Fields(string(FinishDisscussion))
	counter := 0
	for _, f := range fs {
		if strings.Contains(resp, f) {
			counter++
			continue
		}
	}
	return 0.7 < float64(counter)/float64(len(fs))
}

func (a *Agent) Send(input string) (responce string, err error) {
	// run
	var res []string
	defer func() {
		responce = strings.Join(res, "\n\n")
	}()
	r, err := a.chat.Send(a.name, input, true)
	if err != nil {
		return
	}
	res = append(res, r)
	if isFinish(r) {
		return
	}
	// continue disscusion
	for range MaxAgentIterations {
		r, err = a.chat.Send(a.name, string(ContinueDisscussion), true)
		if err != nil {
			return
		}
		res = append(res, r)
		if isFinish(r) {
			return
		}
	}
	return
}

func NewAgentMailBox(prv AIrunner, name string, role Prompt, mb *MailBox, mp MailBoxPermission) *AgentMailBox {
	// validate
	if mb == nil {
		panic(fmt.Errorf("mailbox is nil"))
	}
	// create
	agent := new(AgentMailBox)
	agent.internal = NewAgent(prv, name, role)
	agent.mb = mb
	agent.mp = mp
	return agent
}

type AgentMailBox struct {
	internal *Agent
	mb       *MailBox
	mp       MailBoxPermission
}

func (a AgentMailBox) String() string {
	return a.internal.String()
}

func (a *AgentMailBox) AddSystem(system ...string) {
	a.internal.AddSystem(system...)
}

func (a *AgentMailBox) Init() {
	a.internal.Init()
	if a.mb == nil {
		panic(fmt.Errorf("mailbox is not init for `%s`", a.internal.name))
	}

	var buf bytes.Buffer
	// Add mailbox prompt for email generation
	fmt.Fprintf(&buf, "Правила работы с почтой\n")
	fmt.Fprintf(&buf, "%s\n", MailBoxPrompt)
	fmt.Fprintf(&buf, "Окончание правил работы с почтой\n")
	fmt.Fprintf(&buf, "\n")
	// mail box permissions
	fmt.Fprintf(&buf, "Разрешения у тебя для работы с почтой\n")
	fmt.Fprintf(&buf, "%s\n", a.mp.String())
	fmt.Fprintf(&buf, "Окончание разрешения для работы с почтой\n")
	fmt.Fprintf(&buf, "\n")

	a.internal.chat.AddSystem(buf.String())
}

type MailDirection struct {
	Self  bool
	Other bool
}

type MailBoxPermission struct {
	Read     MailDirection
	Send     MailDirection
	Archived MailDirection
	Solved   MailDirection
}

func (mp MailBoxPermission) String() string {
	res := func(par bool) string {
		if par {
			return "Допустимо"
		}
		return "Не допустимо"
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Начало описания разрешений по работе с почтой\n")

	fmt.Fprintf(&buf, "Чтение своего почтового ящика: %s\n", res(mp.Read.Self))
	fmt.Fprintf(&buf, "Чтение чужого почтового ящика: %s\n", res(mp.Read.Other))

	fmt.Fprintf(&buf, "Отправка писем самому себе(заметки): %s\n", res(mp.Send.Self))
	fmt.Fprintf(&buf, "Отправка писем другим агентам: %s\n", res(mp.Send.Other))

	fmt.Fprintf(&buf, "Архивирование \"archived\" писем самому себе(заметки): %s\n", res(mp.Archived.Self))
	fmt.Fprintf(&buf, "Архивирование \"archived\" писем от других агентов: %s\n", res(mp.Archived.Other))

	fmt.Fprintf(&buf, "Изменение в статус \"solved\" писем самому себе(заметки): %s\n", res(mp.Solved.Self))
	fmt.Fprintf(&buf, "Изменение в статус \"solved\" писем от других агентов: %s\n", res(mp.Solved.Other))

	fmt.Fprintf(&buf, "Окончание описания разрешений по работе с почтой\n")
	fmt.Fprintf(&buf, "\n")
	return buf.String()
}

func DefaultMailPermission() MailBoxPermission {
	return MailBoxPermission{
		Read:     MailDirection{Self: true, Other: false},
		Send:     MailDirection{Self: true, Other: true},
		Archived: MailDirection{Self: true, Other: false},
		Solved:   MailDirection{Self: false, Other: false},
	}
}

// a.AddSystem для :
// * загрузки коллеги
// * общей задачи
// * договоренности
// * конфликты

func (a *AgentMailBox) Run() (err error) {
	input := "Твой почтовый ящик\n" + a.mb.GetThreads(a.internal.name) + "Окончание твоего почтового ящика\n\n"
	if tokens := float64(len(input)) / 2.; 0.7*float64(a.internal.chat.prv.GetContextSize()) < tokens {
		input += "ВАЖНО: Почта переполнена. Срочно подвести необходимо прийти к согласию, чтобы перевести письма в состояние solved. Заархивируй неиспользуемые заметки самому себе\n\n"
	}
	toMail := func(r string) error {
		ms, err := ParseMails[Mail](r)
		if err != nil {
			return err
		}
		a.mb.Add(ms, true, a.internal.name)
		return nil
	}
	r, err := a.internal.chat.Send(a.internal.name, input, true)
	if err != nil {
		return
	}
	err = toMail(r)
	if err != nil {
		return
	}
	if isFinish(r) {
		return
	}
	// continue disscusion
	for range MaxAgentIterations {
		r, err = a.internal.chat.Send(a.internal.name, string(ContinueDisscussion), true)
		if err != nil {
			return
		}
		if isFinish(r) {
			return
		}
		err = toMail(r)
		if err != nil {
			return
		}
	}
	return
}
