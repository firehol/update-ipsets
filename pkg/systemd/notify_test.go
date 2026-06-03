package systemd_test

import (
	"net"
	"path/filepath"
	"strings"
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

func TestNotificationPayloads(t *testing.T) {
	cases := []struct {
		name string
		call func(string) error
		arg  string
		want string
	}{
		{name: "ready", call: systemd.Ready, arg: "", want: "READY=1"},
		{name: "ready status", call: systemd.Ready, arg: "listening", want: "READY=1\nSTATUS=listening"},
		{name: "stopping", call: systemd.Stopping, arg: "", want: "STOPPING=1"},
		{name: "stopping status", call: systemd.Stopping, arg: "shutdown", want: "STOPPING=1\nSTATUS=shutdown"},
		{name: "status", call: systemd.Status, arg: "warming", want: "STATUS=warming"},
		{name: "watchdog", call: systemd.Watchdog, arg: "", want: "WATCHDOG=1"},
		{name: "watchdog status", call: systemd.Watchdog, arg: "healthy", want: "WATCHDOG=1\nSTATUS=healthy"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			socketPath, listener := listenNotifySocket(t)
			t.Setenv("NOTIFY_SOCKET", socketPath)

			done := readNotifyPayload(t, listener)
			if err := tc.call(tc.arg); err != nil {
				t.Fatalf("notification call returned error: %v", err)
			}
			if got := waitNotifyPayload(t, done); got != tc.want {
				t.Fatalf("notification payload = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNotifyNoopAndErrorPaths(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "")

	if err := systemd.Notify(""); err != nil {
		t.Fatalf("Notify(empty) error = %v, want nil", err)
	}
	if err := systemd.Status(""); err != nil {
		t.Fatalf("Status(empty) error = %v, want nil", err)
	}

	t.Setenv("NOTIFY_SOCKET", filepath.Join(t.TempDir(), "missing", "notify.sock"))
	err := systemd.Notify("READY=1")
	if err == nil {
		t.Fatal("Notify() error = nil, want dial error")
	}
	if !strings.Contains(err.Error(), "systemd notify dial failed") {
		t.Fatalf("Notify() error = %q, want dial context", err)
	}
}

func TestWatchdogIntervalInvalidValues(t *testing.T) {
	for _, raw := range []string{"", "0", "-1", "not-a-number"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("WATCHDOG_USEC", raw)
			if got := systemd.WatchdogInterval(); got != 0 {
				t.Fatalf("WatchdogInterval(%q) = %s, want 0", raw, got)
			}
		})
	}
}

func listenNotifySocket(t *testing.T) (string, *net.UnixConn) {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "notify.sock")
	addr := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	listener, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		t.Fatalf("ListenUnixgram returned error: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return socketPath, listener
}

func readNotifyPayload(t *testing.T, listener *net.UnixConn) <-chan string {
	t.Helper()
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
	return done
}

func waitNotifyPayload(t *testing.T, done <-chan string) string {
	t.Helper()
	select {
	case msg := <-done:
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for systemd notification")
		return ""
	}
}
