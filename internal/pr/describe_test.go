package pr

import "testing"

func TestParseDescribeResponse(t *testing.T) {
	raw := `TITLE: fix: SD3031 CTR2 位定义
BODY:
## 变更摘要
- 修正 enableFrequency 注释

## 测试说明
- 本地编译通过`
	title, body, err := parseDescribeResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if title != "fix: SD3031 CTR2 位定义" {
		t.Fatalf("title=%q", title)
	}
	if body == "" || !contains(body, "变更摘要") {
		t.Fatalf("body=%q", body)
	}
}

func TestParseDescribeResponseFallback(t *testing.T) {
	title, body, err := parseDescribeResponse("Quick fix\n\nDetails here")
	if err != nil {
		t.Fatal(err)
	}
	if title != "Quick fix" || body != "Details here" {
		t.Fatalf("title=%q body=%q", title, body)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
