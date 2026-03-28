package creative

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMailbox(t *testing.T) {
	t.Run("threads", func(t *testing.T) {
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
		}, false, "")
		mb.Add([]Mail{
			{
				ID:   1,
				From: "one",
				To:   "qwe",
				Body: "text 3",
			},
		}, false, "")
		_ = mb
		view := func(agent string) {
			mails := mb.GetThreads(agent)
			t.Logf("thread `%s`: %s", agent, mails)
		}
		t.Run("one", func(t *testing.T) { view("one") })
		t.Run("two", func(t *testing.T) { view("two") })
	})
	t.Run("parse", func(t *testing.T) {
		files, err := filepath.Glob(filepath.Join("testdata", "mp*"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			fs := strings.Split(file, "_")
			expect, err := strconv.ParseInt(fs[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			t.Run(strings.ReplaceAll(file, string(filepath.Separator), "/"), func(t *testing.T) {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				mails, err := ParseMails[Mail](string(data))
				if err != nil {
					t.Fatal(err)
				}
				if len(mails) != int(expect) {
					t.Logf("%v", mails)
					t.Errorf("not same amount: %d != %d", len(mails), expect)
				}
			})
		}
	})
}
