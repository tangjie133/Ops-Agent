package repovalidate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Standard 仓库格式规范（standards/*.yaml）。
type Standard struct {
	Name                string   `yaml:"name"`
	Description         string   `yaml:"description"`
	RequiredFiles       []string `yaml:"required_files"`
	RequiredDirs        []string `yaml:"required_dirs"`
	ForbiddenPaths      []string `yaml:"forbidden_paths"`
	ReadmeShouldContain []string `yaml:"readme_should_contain"`
	MinDemos            int      `yaml:"min_demos"`
	DemoDir             string   `yaml:"demo_dir"`
	DemoExtensions      []string `yaml:"demo_extensions"`
	Notes               string   `yaml:"notes"`
}

// Report 检测结果。
type Report struct {
	Standard string
	OK       bool
	Passed   []string
	Failures []string
	Warnings []string
}

func (r *Report) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "── repo_validate (%s) ──\n", r.Standard)
	if r.OK {
		b.WriteString("结果: PASS\n")
	} else {
		b.WriteString("结果: FAIL\n")
	}
	for _, p := range r.Passed {
		fmt.Fprintf(&b, "  ✓ %s\n", p)
	}
	for _, f := range r.Failures {
		fmt.Fprintf(&b, "  ✗ %s\n", f)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "  ! %s\n", w)
	}
	return strings.TrimSpace(b.String())
}

// LoadStandard 从 standards 目录加载规范。
func LoadStandard(standardsDir, name string) (*Standard, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty standard name")
	}
	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		name = name + ".yaml"
	}
	path := filepath.Join(standardsDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load standard %s: %w", name, err)
	}
	var std Standard
	if err := yaml.Unmarshal(data, &std); err != nil {
		return nil, fmt.Errorf("parse standard: %w", err)
	}
	if std.Name == "" {
		std.Name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	}
	return &std, nil
}

// Validate 对照规范检查克隆仓库目录。
func Validate(repoPath string, std *Standard) *Report {
	r := &Report{Standard: std.Name, OK: true}
	if std == nil {
		r.OK = false
		r.Failures = append(r.Failures, "no standard loaded")
		return r
	}

	for _, f := range std.RequiredFiles {
		p := filepath.Join(repoPath, filepath.FromSlash(f))
		if _, err := os.Stat(p); err != nil {
			r.OK = false
			r.Failures = append(r.Failures, "missing required file: "+f)
		} else {
			r.Passed = append(r.Passed, "file: "+f)
		}
	}

	for _, d := range std.RequiredDirs {
		p := filepath.Join(repoPath, filepath.FromSlash(d))
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			r.OK = false
			r.Failures = append(r.Failures, "missing required dir: "+d)
		} else {
			r.Passed = append(r.Passed, "dir: "+d)
		}
	}

	for _, fp := range std.ForbiddenPaths {
		p := filepath.Join(repoPath, filepath.FromSlash(fp))
		if _, err := os.Stat(p); err == nil {
			r.OK = false
			r.Failures = append(r.Failures, "forbidden path present: "+fp)
		}
	}

	readmePath := findReadme(repoPath)
	if len(std.ReadmeShouldContain) > 0 {
		if readmePath == "" {
			r.Warnings = append(r.Warnings, "README not found for content checks")
		} else {
			data, err := os.ReadFile(readmePath)
			if err != nil {
				r.Warnings = append(r.Warnings, "cannot read README")
			} else {
				body := string(data)
				for _, want := range std.ReadmeShouldContain {
					if !strings.Contains(body, want) {
						r.Warnings = append(r.Warnings, "README missing section: "+want)
					} else {
						r.Passed = append(r.Passed, "README contains: "+want)
					}
				}
			}
		}
	}

	return r
}

func findReadme(repoPath string) string {
	for _, name := range []string{"README.md", "readme.md", "Readme.md", "README"} {
		p := filepath.Join(repoPath, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ListStandards 返回 standards 目录下可用规范名。
func ListStandards(standardsDir string) ([]string, error) {
	entries, err := os.ReadDir(standardsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ext))
	}
	return names, nil
}
