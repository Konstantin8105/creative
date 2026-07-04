package creative

import "testing"

func TestGetPrompt(t *testing.T) {
	t.Run("universal", func(t *testing.T) {
		v, err := GetPrompt("universal")
		if err != nil {
			t.Error(err)
		}
		t.Logf("%s", v)
	})
	t.Run("bad_name", func(t *testing.T) {
		_, err := GetPrompt("bad_name")
		if err == nil {
			t.Errorf("not valid data")
		}
	})
}
