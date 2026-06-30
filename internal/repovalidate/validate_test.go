package repovalidate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateArduinoLibrary(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"README.md", "keywords.txt", "library.properties"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("# Installation\n# Example\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}

	std := &Standard{
		Name:                "test",
		RequiredFiles:       []string{"README.md", "library.properties"},
		RequiredDirs:        []string{"examples"},
		ReadmeShouldContain: []string{"Installation"},
	}
	r := Validate(dir, std)
	if !r.OK {
		t.Fatalf("expected pass: %+v", r.Failures)
	}
}

func TestValidateMissingFile(t *testing.T) {
	dir := t.TempDir()
	r := Validate(dir, &Standard{Name: "t", RequiredFiles: []string{"missing.txt"}})
	if r.OK {
		t.Fatal("expected fail")
	}
}
