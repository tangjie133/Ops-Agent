package libtest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZzedJay/Ops-Agent/internal/config"
	"github.com/ZzedJay/Ops-Agent/internal/repovalidate"
)

// DemoReport examples/demo 目录检测结果。
type DemoReport struct {
	Dir         string
	Count       int
	Demos       []string
	OK          bool
	Failures    []string
	Warnings    []string
	Suggestions []string
}

func (d *DemoReport) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "── demo 验收 (%s) ──\n", d.Dir)
	if d.OK {
		b.WriteString("结果: PASS\n")
	} else {
		b.WriteString("结果: FAIL\n")
	}
	fmt.Fprintf(&b, "发现 %d 个 demo:\n", d.Count)
	for _, name := range d.Demos {
		fmt.Fprintf(&b, "  · %s\n", name)
	}
	for _, f := range d.Failures {
		fmt.Fprintf(&b, "  ✗ %s\n", f)
	}
	for _, w := range d.Warnings {
		fmt.Fprintf(&b, "  ! %s\n", w)
	}
	for _, s := range d.Suggestions {
		fmt.Fprintf(&b, "  → 建议: %s\n", s)
	}
	return strings.TrimSpace(b.String())
}

// CheckDemos 检查 examples 等 demo 目录。
func CheckDemos(repoPath string, cfg config.LibTestConfig, std *repovalidate.Standard) *DemoReport {
	cfg.Normalize()
	dir := cfg.DemoDir
	if std != nil && std.DemoDir != "" {
		dir = std.DemoDir
	}
	min := cfg.MinDemos
	if std != nil && std.MinDemos > 0 {
		min = std.MinDemos
	}

	r := &DemoReport{Dir: dir, OK: true}
	demoRoot := filepath.Join(repoPath, filepath.FromSlash(dir))
	info, err := os.Stat(demoRoot)
	if err != nil || !info.IsDir() {
		r.OK = false
		r.Failures = append(r.Failures, "demo 目录不存在: "+dir)
		r.Suggestions = append(r.Suggestions, fmt.Sprintf("添加 %s/ 并在其下放置至少 %d 个示例 sketch", dir, min))
		return r
	}

	exts := demoExtensions(std)
	_ = filepath.WalkDir(demoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		for _, want := range exts {
			if ext == want {
				rel, _ := filepath.Rel(demoRoot, path)
				r.Demos = append(r.Demos, filepath.ToSlash(rel))
				r.Count++
				checkSketchContent(path, r)
				break
			}
		}
		return nil
	})

	if r.Count < min {
		r.OK = false
		r.Failures = append(r.Failures, fmt.Sprintf("demo 数量不足: %d < %d", r.Count, min))
		r.Suggestions = append(r.Suggestions, suggestMissingDemos(repoPath, dir, min-r.Count))
	}

	if r.Count == 1 && min >= 1 {
		r.Warnings = append(r.Warnings, "仅 1 个 demo，建议增加更多使用场景示例")
	}

	if len(r.Failures) > 0 {
		r.OK = false
	}
	return r
}

func demoExtensions(std *repovalidate.Standard) []string {
	if std != nil && len(std.DemoExtensions) > 0 {
		return std.DemoExtensions
	}
	return []string{".ino", ".cpp", ".c"}
}

func checkSketchContent(path string, r *DemoReport) {
	data, err := os.ReadFile(path)
	if err != nil {
		r.Warnings = append(r.Warnings, "无法读取: "+path)
		return
	}
	body := string(data)
	if len(strings.TrimSpace(body)) < 20 {
		r.Warnings = append(r.Warnings, "demo 文件过短: "+path)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".ino" || ext == ".cpp" || ext == ".c" {
		if !strings.Contains(body, "setup") {
			r.Warnings = append(r.Warnings, "缺少 setup(): "+path)
		}
		if !strings.Contains(body, "loop") {
			r.Warnings = append(r.Warnings, "缺少 loop(): "+path)
		}
	}
}

func suggestMissingDemos(repoPath, demoDir string, need int) string {
	if need <= 0 {
		return ""
	}
	var names []string
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := strings.ToLower(d.Name())
		if strings.HasPrefix(base, "dfrobot_") && strings.HasSuffix(base, ".h") {
			names = append(names, strings.TrimSuffix(d.Name(), ".h")+"_demo")
		}
		return nil
	})
	if len(names) == 0 {
		return fmt.Sprintf("在 %s/ 下新增 %d 个 .ino 示例", demoDir, need)
	}
	if len(names) > need {
		names = names[:need]
	}
	return fmt.Sprintf("可考虑新增 demo: %s", strings.Join(names, ", "))
}
