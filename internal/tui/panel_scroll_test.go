package tui

import "testing"

func TestPanelWrapSel(t *testing.T) {
	if got := panelWrapSel(2, 1, 3); got != 0 {
		t.Fatalf("wrap down from last: got %d want 0", got)
	}
	if got := panelWrapSel(0, -1, 3); got != 2 {
		t.Fatalf("wrap up from first: got %d want 2", got)
	}
	if got := panelWrapSel(-1, 1, 3); got != 0 {
		t.Fatalf("from unset down: got %d want 0", got)
	}
	if got := panelWrapSel(-1, -1, 3); got != 2 {
		t.Fatalf("from unset up: got %d want 2", got)
	}
}

func TestPanelScrollStart(t *testing.T) {
	// maxLines=10 → 5 条可见（每条 2 行）
	const max = 10
	cases := []struct {
		sel, total, want int
	}{
		{0, 20, 0},
		{4, 20, 0},
		{5, 20, 1},
		{9, 20, 5},
		{19, 20, 15},
		{-1, 20, 0},
		{0, 3, 0},
	}
	for _, c := range cases {
		got := panelScrollStart(c.sel, c.total, max)
		if got != c.want {
			t.Errorf("panelScrollStart(%d, %d, %d) = %d, want %d", c.sel, c.total, max, got, c.want)
		}
	}
}
