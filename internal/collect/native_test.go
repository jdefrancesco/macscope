package collect

import (
	"runtime"
	"strings"
	"testing"
)

func TestMacOSRunner(t *testing.T) {
	runner, err := MacOSRunner()
	if runtime.GOOS != "darwin" {
		if err == nil || !strings.Contains(err.Error(), "requires macOS") {
			t.Fatalf("error = %v, want unsupported-platform error", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.Env) == 0 || runner.Env[len(runner.Env)-1] != "LC_ALL=C" {
		t.Fatal("native runner must set LC_ALL=C for stable parsing")
	}
}
