//go:build !unix

package output

import (
	"os"
	"os/exec"
)

func prepareGitCommand(_ *exec.Cmd) {}

func killGitCommandTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return cmd.Process.Kill()
}
