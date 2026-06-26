package systemd

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNotifyWithDeadlineBoundsBlockedDial(t *testing.T) {
	previousDial := notifyDial
	t.Cleanup(func() { notifyDial = previousDial })
	notifyDial = func(ctx context.Context, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Setenv("NOTIFY_SOCKET", "/tmp/update-ipsets-blocked-notify.sock")

	started := time.Now()
	err := NotifyWithDeadline("READY=1", 20*time.Millisecond)
	elapsed := time.Since(started)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NotifyWithDeadline() error = %v, want context deadline exceeded", err)
	}
	if !strings.Contains(err.Error(), "systemd notify dial failed") {
		t.Fatalf("NotifyWithDeadline() error = %q, want dial context", err)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("NotifyWithDeadline() elapsed = %s, want bounded by deadline", elapsed)
	}
}
