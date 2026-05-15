package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareAgentCLIPathCopiesExecutableIntoWorkdir(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	binDir := prepareAgentCLIPath(workDir, nil)
	if binDir == "" {
		t.Fatal("prepareAgentCLIPath returned empty bin dir")
	}

	wantDir := filepath.Join(workDir, ".multica", "bin")
	if binDir != wantDir {
		t.Fatalf("bin dir = %q, want %q", binDir, wantDir)
	}

	dest := filepath.Join(binDir, agentCLIBinaryName())
	destInfo, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("expected copied CLI at %s: %v", dest, err)
	}
	if !destInfo.Mode().IsRegular() {
		t.Fatalf("copied CLI is not a regular file: %s", dest)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		t.Fatalf("stat self executable: %v", err)
	}
	if destInfo.Size() != selfInfo.Size() {
		t.Fatalf("copied CLI size = %d, want %d", destInfo.Size(), selfInfo.Size())
	}
}

func TestPrependAgentPath(t *testing.T) {
	t.Parallel()

	sep := string(os.PathListSeparator)
	env := map[string]string{"PATH": "base"}
	prependAgentPath(env, "task-bin")
	if got, want := env["PATH"], "task-bin"+sep+"base"; got != want {
		t.Fatalf("PATH = %q, want %q", got, want)
	}

	env = map[string]string{}
	prependAgentPath(env, "task-bin")
	if got := env["PATH"]; got != "task-bin" {
		t.Fatalf("PATH = %q, want task-bin", got)
	}
}
