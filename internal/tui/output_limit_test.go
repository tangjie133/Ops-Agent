package tui

import "testing"

func TestTruncateLogDisplay(t *testing.T) {
	short := "hello"
	if got := truncateLogDisplay(short); got != short {
		t.Fatalf("got %q", got)
	}
	long := stringsRepeat("a", maxLogDisplayChars+10)
	got := truncateLogDisplay(long)
	if len(got) != maxLogDisplayChars+len("…") {
		t.Fatalf("len=%d want %d", len(got), maxLogDisplayChars+len("…"))
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
