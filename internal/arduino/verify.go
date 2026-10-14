package arduino

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LookPath locates arduino-cli. Tests replace it.
var LookPath = exec.LookPath

// RunCLI runs arduino-cli. Tests replace it.
var RunCLI = defaultRunCLI

func defaultRunCLI(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Verify recognizes the workspace as a library and compiles every examples/*.ino sketch.
func Verify(ctx context.Context, workDir string) error {
	lp := LookPath
	if lp == nil {
		lp = exec.LookPath
	}
	if _, err := lp("arduino-cli"); err != nil {
		return fmt.Errorf("缺少 arduino-cli")
	}
	props, _ := os.ReadFile(filepath.Join(workDir, "library.properties"))
	fqbn := FQBN(string(props))
	var sketches []string
	root := filepath.Join(workDir, "examples")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".ino") {
			rel, _ := filepath.Rel(workDir, path)
			sketches = append(sketches, filepath.ToSlash(rel))
		}
		return nil
	})
	if len(sketches) == 0 {
		return fmt.Errorf("缺少验证")
	}
	run := RunCLI
	if run == nil {
		run = defaultRunCLI
	}
	for _, rel := range sketches {
		sketchDir := filepath.Join(workDir, filepath.FromSlash(filepath.Dir(rel)))
		out, err := run(ctx, workDir, "arduino-cli", "compile", "--fqbn", fqbn, "--library", workDir, sketchDir)
		if err != nil {
			return fmt.Errorf("示例编译失败: %s\n%s", rel, out)
		}
	}
	return nil
}
