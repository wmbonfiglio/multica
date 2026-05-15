package daemon

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

func agentCLIBinaryName() string {
	if runtime.GOOS == "windows" {
		return "multica.exe"
	}
	return "multica"
}

func prependPathValue(dir, current string) string {
	if current == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + current
}

func prependAgentPath(env map[string]string, dir string) {
	if env == nil || dir == "" {
		return
	}
	env["PATH"] = prependPathValue(dir, env["PATH"])
}

func copyFileExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	mode := info.Mode().Perm() | 0o755
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}

	// Windows cannot rename over an existing executable. The per-task copy is
	// only used after the previous agent process exits, so replacing it here is
	// safe and keeps reused workdirs aligned with a freshly updated daemon.
	_ = os.Remove(dst)
	if err := os.Rename(tmpPath, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func prepareAgentCLIPath(workDir string, logger *slog.Logger) string {
	selfBin, err := os.Executable()
	if err != nil {
		if logger != nil {
			logger.Warn("agent env: resolve multica executable failed", "error", err)
		}
		return ""
	}
	if workDir == "" {
		return filepath.Dir(selfBin)
	}

	destDir := filepath.Join(workDir, ".multica", "bin")
	dest := filepath.Join(destDir, agentCLIBinaryName())
	if err := copyFileExecutable(selfBin, dest); err == nil {
		return destDir
	} else if logger != nil {
		logger.Warn("agent env: copy multica CLI into task workdir failed", "source", selfBin, "dest", dest, "error", err)
	}

	return filepath.Dir(selfBin)
}
