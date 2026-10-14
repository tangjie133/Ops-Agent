package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFakeWritesChange(t *testing.T) {
	dir := t.TempDir()
	f := &Fake{Changed: true}
	res, err := f.Run(context.Background(), Request{WorkDir: dir})
	if err != nil || !res.Changed {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "FAKE_CHANGE.md")); err != nil {
		t.Fatal(err)
	}
}

func TestFakeReturnsError(t *testing.T) {
	f := &Fake{Err: fmt.Errorf("boom")}
	_, err := f.Run(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected error")
	}
}
