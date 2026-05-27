package headless

import (
	"os"
	"testing"
)

func TestShouldRunCIEnv(t *testing.T) {
	os.Setenv("OPS_AGENT_CI", "1")
	defer os.Unsetenv("OPS_AGENT_CI")
	if !ShouldRun() {
		t.Fatal("expected ShouldRun true when OPS_AGENT_CI=1")
	}
}
