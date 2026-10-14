package arduino

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingCLI(t *testing.T) {
	LookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { LookPath = nil })
	err := Verify(context.Background(), t.TempDir())
	if err == nil || err.Error() != "缺少 arduino-cli" {
		t.Fatalf("%v", err)
	}
}

func TestMissingExamples(t *testing.T) {
	LookPath = func(string) (string, error) { return "/bin/arduino-cli", nil }
	t.Cleanup(func() { LookPath = nil })
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "library.properties"), []byte("architectures=avr\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Verify(context.Background(), dir)
	if err == nil || err.Error() != "缺少验证" {
		t.Fatalf("%v", err)
	}
}

func TestCompileAllExamples(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "library.properties"), []byte("architectures=esp32\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"examples/A/A.ino", "examples/nested/B/B.ino"} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("void setup(){}\nvoid loop(){}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var calls [][]string
	LookPath = func(string) (string, error) { return "/bin/arduino-cli", nil }
	RunCLI = func(ctx context.Context, dir string, args ...string) (string, error) {
		calls = append(calls, append([]string{dir}, args...))
		return "ok", nil
	}
	t.Cleanup(func() { LookPath = nil; RunCLI = nil })
	if err := Verify(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls=%d", len(calls))
	}
	for _, c := range calls {
		joined := strings.Join(c, " ")
		if !strings.Contains(joined, "compile") || !strings.Contains(joined, "--fqbn") || !strings.Contains(joined, "esp32:esp32:esp32") || !strings.Contains(joined, "--library") {
			t.Fatalf("%v", c)
		}
	}
}

func TestCompileFailureIncludesOutput(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "examples/Bad/Bad.ino")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	LookPath = func(string) (string, error) { return "/bin/arduino-cli", nil }
	RunCLI = func(ctx context.Context, dir string, args ...string) (string, error) {
		return "library not found", errors.New("exit 1")
	}
	t.Cleanup(func() { LookPath = nil; RunCLI = nil })
	err := Verify(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "examples/Bad") || !strings.Contains(err.Error(), "library not found") {
		t.Fatalf("%v", err)
	}
}
