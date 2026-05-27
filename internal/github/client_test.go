package github

import "testing"

func TestParseAuthStatusNewFormat(t *testing.T) {
	raw := `github.com
  ✓ Logged in to github.com account ZzedJay (keyring)
  - Active account: true`

	status := parseAuthStatusRaw(raw)
	if status.User != "ZzedJay" {
		t.Fatalf("user: got %q want ZzedJay", status.User)
	}
	if status.Host != "github.com" {
		t.Fatalf("host: got %q want github.com", status.Host)
	}
}
