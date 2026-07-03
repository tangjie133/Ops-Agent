package lang

import (
	"strings"
	"testing"
)

func TestFromIssuePrefersLatestComment(t *testing.T) {
	comments := []string{"这是中文追问", "Please share the register map"}
	if got := FromIssue("SD3031 bug", "CTR2 bit wrong", comments); got != English {
		t.Fatalf("FromIssue=%v want English (latest comment)", got)
	}
	comments = []string{"Please share logs", "请补充寄存器说明"}
	if got := FromIssue("SD3031 bug", "CTR2 bit wrong", comments); got != Chinese {
		t.Fatalf("FromIssue=%v want Chinese (latest comment)", got)
	}
}

func TestFromIssueFallsBackToBody(t *testing.T) {
	if got := FromIssue("Fix timeout", "The I2C read hangs after 100ms on ESP32.", nil); got != English {
		t.Fatalf("FromIssue=%v want English", got)
	}
	if got := FromIssue("修复超时", "ESP32 上 I2C 读取 100ms 后卡住。", nil); got != Chinese {
		t.Fatalf("FromIssue=%v want Chinese", got)
	}
}

func TestPROpenedCommentLanguage(t *testing.T) {
	en := PROpenedComment("https://github.com/o/r/pull/1", "fixed CTR2", "Bug", "Please fix", []string{"Any update?"})
	if !strings.HasPrefix(en, "Opened a PR") {
		t.Fatalf("got %q", en)
	}
	zh := PROpenedComment("https://github.com/o/r/pull/1", "修复 CTR2", "Bug", "请修复", nil)
	if !strings.HasPrefix(zh, "已开 PR") {
		t.Fatalf("got %q", zh)
	}
}
