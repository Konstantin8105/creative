package creative

import "testing"

func TestMailbox(t *testing.T) {
	var mb MailBox
	mb.Add([]Mail{
		{
			From: "qwe",
			To:   "one",
			Body: "text 1",
		},
		{
			From: "qwe",
			To:   "one",
			Body: "text 2",
		},
		{
			From: "qwe",
			To:   "two",
			Body: "text 4",
		},
	})
	mb.Add([]Mail{
		{
			ID:   1,
			From: "one",
			To:   "qwe",
			Body: "text 3",
		},
	})
	_ = mb
	view := func(agent string) {
		mails := mb.GetThreads(agent)
		t.Logf("thread `%s`: %s", agent, mails)
	}
	view("one")
	view("two")
}
