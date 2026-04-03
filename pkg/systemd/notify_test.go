package systemd_test

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/systemd"
)

func TestNotifyAndWatchdogInterval(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "notify.sock")
	addr := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	listener, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		t.Fatalf("ListenUnixgram returned error: %v", err)
	}
	defer func() { _ = listener.Close() }()

	t.Setenv("NOTIFY_SOCKET", socketPath)
	t.Setenv("WATCHDOG_USEC", "1000000")

	if got, want := systemd.WatchdogInterval(), 500*time.Millisecond; got != want {
		t.Fatalf("unexpected watchdog interval: got %s want %s", got, want)
	}

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 256)
		n, _, err := listener.ReadFromUnix(buf)
		if err != nil {
			done <- "read error: " + err.Error()
			return
		}
		done <- string(buf[:n])
	}()

	if err := systemd.Ready("listening"); err != nil {
		t.Fatalf("Ready returned error: %v", err)
	}

	select {
	case msg := <-done:
		if msg != "READY=1\nSTATUS=listening" {
			t.Fatalf("unexpected notify payload %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for systemd notification")
	}
}
