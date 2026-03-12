package creative

import (
	"encoding/json"
	"log"
	"strings"
)

type Mail struct {
	ID       uint64 `json:"id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Body     string `json:"body"`
	Archived bool   `json:"archived"`
	Solved bool   `json:"solved"`
}

func (m Mail) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Printf("mail string error: %v", err)
	}
	return string(data)
}

// ParseMails return mails From AI answer
func ParseMails(body string) (ms []Mail, err error) {
	for {
		{
			start := strings.Index(body, "```json")
			if start < 0 {
				break
			}
			start += 7
			body = body[start:]
		}
		var msg string
		finish := strings.Index(body, "```")
		if finish < 0 {
			msg = body
		} else {
			msg = body[:finish]
			body = body[finish+3:]
		}
		// parse message
		msg = strings.TrimSpace(msg)
		var mail Mail
		err = json.Unmarshal([]byte(msg), &mail)
		if err != nil {
			log.Printf("cannot parse mail: `%s`. err = %v", msg, err)
			continue
		}
		ms = append(ms, mail)
	}
	if len(ms) == 0 {
		for {
			{
				start := strings.Index(body, "{")
				if start < 0 {
					break
				}
				body = body[start:]
			}
			var msg string
			finish := strings.Index(body, "}")
			if finish < 0 {
				msg = body
			} else {
				msg = body[:finish+1]
				body = body[finish+1:]
			}
			// parse message
			msg = strings.TrimSpace(msg)
			var mail Mail
			err = json.Unmarshal([]byte(msg), &mail)
			if err != nil {
				log.Printf("cannot parse mail: `%s`. err = %v", msg, err)
				continue
			}
			ms = append(ms, mail)
		}
	}
	log.Printf("ParseMails. amount mails: %d", len(ms))
	return
}

const MainBoxPrompt Prompt = `
Полностью реализуй свою роль путем написания новых писем коллегам для решения общей задачи или ответить на их вопросы.
Ты выполняешь свою роль, а твои коллеги свою, поэтому так можешь идентифицировать кому написать.
Но не более чем 5 новых/отвеченных писем.
Необходимо стараться использовать по полной свой лимит писем.
нет смысла высылать письма самому себе.
Необходимо стараться 1 письмо - одна изолированная тема, для того чтобы удобней было работать всем.
При написании письмо в поле "to" необходимо использовать список имен коллеги, не придумываю новых и аккуртано и точно записываю их имя. Также как и "from", т.е. от кого будет проставлено автоматически.
Не выдумывать id письма, а использовать только те которые доступны. Для новых писем их id будет проставлено автоматически без чьего-либо участия.

При написании коллеге, ты предлагаешь он наилучшим образом решит в дальнейшем на основании его роли, его назначении. Это позволит наибыструйшему поиску решению.
Каждый агент учитывает мнение друг друга с уважением.

Для написания или ответа на письмо используется следующий формат:
` + "```json" + `
{
	"id": здесь пишеться число письма на которое отвечает, но если это новое письмо, то этой строки не должно быть,
	"to": "здесь имя твоего коллеги",
	"body" : "здесь пишешь ему сообщение. Старайся использовать однострочный текст, а если надо многострочный, то учитывай экранирование для JSON формата"
}
` + "```" + `
ВАЖНО: Ответ должен быть строго в JSON формат без лишнего текста.

Пример нового письма:
` + "```json" + `
{
	"to": "critic",
	"body" : "Предлагаю учесть как это будет реализовано при программировании"
}
` + "```" + `

Пример ответа на письмо с номером 42:
` + "```json" + `
{
	"id": 42,
	"to": "dreamer",
	"body" : "На мой взгляд, добавь больше идей для разбиения задачи на подзадачи"
}
` + "```" + `

Пример ответа на письмо с номером 65342:
` + "```json" + `
{
	"id": 65342,
	"to": "writer",
	"body" : "По вопросу версионности из письма 34 достигнута согласие между всеми и переношу в список решенных задач"
}
` + "```" + `

Пример того как архивировать письма у всех под номером 234:
` + "```json" + `
{
	"id": 234,
	"archived": true,
}
` + "```" + `

Пример перевода письма в состояние solved под номером 345:
` + "```json" + `
{
	"id": 345,
	"solved": true,
}
` + "```" + `
Перевод письма в состояние solved автоматически переводит письма в архивное письмо.

Каждое письмо должно отделено друг от друга, к примеру как пишеться 3 письма о разном:
` + "```json" + `
{
	"id": 345,
	"solved": true,
}
` + "```" + `
` + "```json" + `
{
	"id": 234,
	"archived": true,
}
` + "```" + `
` + "```json" + `
{
	"id": 343,
	"solved": true,
}
` + "```" + `

`

type MailBox struct {
	presentID uint64
	mails     []Mail
}

func (mb *MailBox) Add(mails []Mail) {
	for _, m := range mails {
		if m.ID == 0 {
			mb.presentID++
			m.ID = mb.presentID
		}
		if m.Solved {
			for i := range mb.mails {
				if mb.mails[i].ID == m.ID {
					mb.mails[i].Archived = true
				}
			}
		}
		mb.mails = append(mb.mails, m)
	}
}

func (mb MailBox) GetUnsolved() (mails string) {
	for _, m := range mb.mails {
		if m.Archived {
			continue
		}
		if m.Solved {
			continue
		}
		mails += m.String()
	}
	return
}


func (mb MailBox) Get(To string) (mails string) {
	for _, m := range mb.mails {
		if m.Archived {
			continue
		}
		if m.Solved {
			continue
		}
		if To == m.From || To == m.To {
			mails += m.String()
		}
	}
	return
}

func (mb MailBox) GetSolved() (mails string) {
	for _, m := range mb.mails {
		if m.Archived {
			continue
		}
		if !m.Solved {
			continue
		}
			mails += m.String()
	}
	return
}
