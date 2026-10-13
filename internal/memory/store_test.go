package memory

import (
	"path/filepath"
	"testing"
)

func TestAppendAndListFiltersByTask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := Append(path, Event{TaskID: "a", Kind: "start", Message: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, Event{TaskID: "b", Kind: "start", Message: "2"}); err != nil {
		t.Fatal(err)
	}
	got, err := List(path, "a")
	if err != nil || len(got) != 1 || got[0].Message != "1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
