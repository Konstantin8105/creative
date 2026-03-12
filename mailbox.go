package creative

import (
	"encoding/json"
	"log"
	"strings"
)

type Mail struct {
	ID       uint64 `json:"id"` // ID is unique position in mailbox
	From     string `json:"from"`
	To       string `json:"to"`
	Body     string `json:"body"`
	Archived bool   `json:"archived"`
	Solved   bool   `json:"solved"`
	ReplyID  uint64
	// ThreadID uint64   // for create threads of mails, store ID of first mail in thread
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
	cbody := body
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
			// log.Printf("cannot parse mail: `%s`. err = %v", msg, err)
			// try many emails
			var mails []Mail
			err = json.Unmarshal([]byte(msg), &mail)
			if err != nil {
				log.Printf("cannot parse mail: `%s`. err = %v", msg, err)
				continue
			}
			ms = append(ms, mails...)
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
	if len(ms) == 0 && cbody != "" {
		log.Printf("ParseMail. cannot parse mail: `%s`", cbody)
	}
	return
}

const MailBoxPrompt Prompt = `
ПРАВИЛА РАБОТЫ С ПОЧТОЙ:
- Каждое письмо должно быть оформлено как JSON-объект.
- Для ответа на письмо обязательно указывай его ID в поле "id".
- Для нового письма поле "id" не указывай.
- Можно отправлять несколько писем сразу, используя массив JSON.
- Поле "to" должно содержать имя одного получателя.
- Не отправляй письма самому себе.
- Ограничение: не более 20 писем за один раз.
- Ты выполняешь свою роль, а твои коллеги свою, поэтому так можешь идентифицировать кому написать.
- Необходимо стараться использовать по полной свой лимит писем.
- В письме одна изолированная тема, для того чтобы удобней было работать всем.
- Перед отправкой письма проверь, что еще не отвечал на него.
- Не делать отсулку одинаковых вопросов в письмах.
При написании письмо в поле "to" необходимо использовать список имен коллеги, не придумываю новых и аккуртано и точно записываю их имя. Также как и "from", т.е. от кого будет проставлено автоматически.
Не выдумывать id письма, а использовать только те которые доступны. Для новых писем их id будет проставлено автоматически без чьего-либо участия.

При написании коллеге, ты предлагаешь он наилучшим образом решит в дальнейшем на основании его роли, его назначении. Это позволит наибыструйшему поиску решению.
Каждый агент учитывает мнение друг друга с уважением.
Не стоит ссылаться на другие письма, а если нужна ссылка то скопирую текст.

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

Каждое письмо должно отделено друг от друга, к примеру как пишеться множество писем о разном:
` + "```json" + `
[
  {
    "to": "critic",
    "body": "Броскает риск, что мечтательский дизайн не учтет ограничения ОС Windows. Как убедиться, что UI будет работать на всех версиях?"
  },
  {
    "to": "realist",
    "body": "Нужно уточнить, сколько оперативной памяти понадобится для рендеринга 3D‑модели из 10 000 элементов. Это влияет на тайминги разработки."
  },
  {
    "to": "dreamer",
    "body": "В твоих идеях часто упоминаются «грандиозные» графические эффекты, но они могут потребовать OpenGL 4.5+. Какой уровень совместимости нам нужен?"
  },
  {
    "to": "arxiv",
    "body": "Требуется разделить письмо о планах на две части: часть 1 – цели, часть 2 – задачи. Архивировать оригинал."
  },
  {
    "to": "solved",
    "body": "После обсуждения с критиком и реалістом задача «выбор OpenGL версии» завершена."
  },
  {
    "to": "critic",
    "body": "Как избежать утечек памяти при динамическом добавлении/удалении узлов? Нужно ввести менеджер памяти."
  },
  {
    "to": "arxiv",
    "body": "Разделяем письмо о настройках OpenGL на 2 части: 1) установка драйверов, 2) конфигурация контекста."
  }
]` + "```" + `

`

type MailBox struct {
	presentID uint64
	mails     []Mail
}

func (mb *MailBox) Add(mails []Mail) {
	for i, m := range mails {
		if m.ID != 0 {
			m.ReplyID = m.ID
		}
		mb.presentID++
		mails[i].ID = mb.presentID
	}
	for _, m := range mails {
		if m.Solved {
			for i := range mb.mails {
				if mb.mails[i].ID == m.ID {
					mb.mails[i].Archived = true
				}
			}
		}
		log.Printf("send email in mailbox\n%s", m)
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
