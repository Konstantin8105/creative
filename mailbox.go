package creative

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type ShortMessage struct {
	From string `json:"from"`
	To   string `json:"to"`
	Body string `json:"body"`
}

func (s ShortMessage) String() string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Printf("mail string error: %v", err)
	}
	return string(data)
}

func Convert(m Mail) ShortMessage {
	return ShortMessage{
		From: m.From,
		To:   m.To,
		Body: m.Body,
	}
}

type Mail struct {
	// ID is unique position in mailbox
	ID       int    `json:"id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Body     string `json:"body"`
	Archived bool   `json:"archived"`
	Solved   bool   `json:"solved"`
	ReplyID  int
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
	defer func() {
		// remove empty mails
	again:
		for i := range ms {
			if ms[i].To == "" || ms[i].Body == "" {
				ms = append(ms[:i], ms[i+1:]...)
				goto again
			}
		}
		log.Printf("ParseMails. amount mails: %d", len(ms))
	}()
	body = strings.TrimSpace(body)
	if len(body) == 0 {
		return
	}
	cbody := body
	body = strings.ReplaceAll(body, "```json", "")
	body = strings.ReplaceAll(body, "```", "")
	{
		var mails []Mail
		err = json.Unmarshal([]byte(body), &mails)
		if err == nil {
			ms = append(ms, mails...)
			return
		}
	}
	{
		var mail Mail
		err = json.Unmarshal([]byte(body), &mail)
		if err == nil {
			ms = append(ms, mail)
			return
		}
	}
	for range 1000 { // avoid infinity
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
				log.Printf("cannot parse mail 1: `%s`. err = %v", msg, err)
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
				break
			} else {
				msg = body[:finish+1]
				body = body[finish+1:]
			}
			// parse message
			msg = strings.TrimSpace(msg)
			var mail Mail
			err = json.Unmarshal([]byte(msg), &mail)
			if err != nil {
				log.Printf("cannot parse mail 2: `%s`. err = %v", msg, err)
				continue
			}
			ms = append(ms, mail)
		}
	}
	if len(ms) == 0 {
		log.Printf("ParseMail. cannot parse mail 4: `%s`", cbody)
	}
	return
}

//go:embed mailbox.md
var MailBoxPrompt Prompt

type MailBox struct {
	presentID int
	mails     []Mail
}

func (mb MailBox) Save(filename string) {
	data, err := json.MarshalIndent(mb.mails, "", "  ")
	if err != nil {
		log.Printf("mail save error: %v", err)
	}
	err = os.WriteFile(filename, data, 0777)
	if err != nil {
		log.Printf("mail save error: %v", err)
	}
}

func (mb *MailBox) Add(mails []Mail) {
	// prepare new mails for thread mails
	for i := range mails {
		mails[i].ReplyID = -1 // default value
		if mails[i].ID != 0 && mails[i].ID < len(mb.mails) {
			mails[i].ReplyID = mails[i].ID
		}
		mails[i].ID = mb.presentID
		mb.presentID++
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
		mails += Convert(m).String()
	}
	return
}

func (mb MailBox) GetThreads(agent string) (mails string) {
	var all []Mail
	for _, m := range mb.mails {
		if m.Archived {
			continue
		}
		if m.Solved {
			continue
		}
		if agent == m.From || agent == m.To {
			all = append(all, m)
		}
	}
	threads := map[int][]Mail{}
	for i := range all {
		if all[i].ReplyID < 0 {
			threads[all[i].ID] = append(threads[all[i].ID], all[i])
			continue
		}
		threads[all[i].ReplyID] = append(threads[all[i].ReplyID], all[i])
	}
	for k, ms := range threads {
		if k < 0 {
			continue
		}
		mails += fmt.Sprintf("Email thread with base email id: %d\n", k)
		var sms []ShortMessage
		for i := range ms {
			sms = append(sms, Convert(ms[i]))
		}
		data, err := json.MarshalIndent(sms, "", "  ")
		if err != nil {
			log.Printf("mail string error: %v", err)
		}
		mails += "```json\n"
		mails += string(data) + "\n"
		mails += "```\n"
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
