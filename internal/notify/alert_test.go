package notify

import "testing"

func TestFormatBody(t *testing.T) {
	body := FormatBody(Alert{
		Title:    "test",
		Repo:     "o/r",
		PRNumber: 7,
		Failures: []string{"check failed"},
	})
	if body == "" {
		t.Fatal("empty body")
	}
	if !contains(body, "o/r") || !contains(body, "check failed") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestSlackPayload(t *testing.T) {
	b, err := slackPayload(Alert{Title: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("empty payload")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
