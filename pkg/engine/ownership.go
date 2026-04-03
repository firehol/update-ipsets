package engine

import (
	"fmt"
	"os/exec"
	"strings"
)

func chownPath(owner, path string) error {
	if owner == "" || path == "" {
		return nil
	}
	out, err := exec.Command("chown", owner, path).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("chown %s %s: %w: %s", owner, path, err, msg)
		}
		return fmt.Errorf("chown %s %s: %w", owner, path, err)
	}
	return nil
}
