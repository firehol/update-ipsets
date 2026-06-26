package systemd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const lifecycleNotifyDeadline = 2 * time.Second

type notifyDialFunc func(context.Context, string) (net.Conn, error)

var notifyDial notifyDialFunc = dialNotifySocket

func Ready(status string) error {
	if status == "" {
		return Notify("READY=1")
	}
	return Notify("READY=1\nSTATUS=" + status)
}

func Stopping(status string) error {
	if status == "" {
		return Notify("STOPPING=1")
	}
	return Notify("STOPPING=1\nSTATUS=" + status)
}

func Status(status string) error {
	if status == "" {
		return nil
	}
	return Notify("STATUS=" + status)
}

func Watchdog(status string) error {
	if status == "" {
		return Notify("WATCHDOG=1")
	}
	return Notify("WATCHDOG=1\nSTATUS=" + status)
}

func WatchdogInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("WATCHDOG_USEC"))
	if raw == "" {
		return 0
	}
	usec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || usec <= 0 {
		return 0
	}
	return time.Duration(usec/2) * time.Microsecond
}

func NotifyDeadline(watchdogInterval time.Duration) time.Duration {
	if watchdogInterval <= 0 {
		return lifecycleNotifyDeadline
	}
	deadline := watchdogInterval / 2
	if deadline <= 0 || deadline > lifecycleNotifyDeadline {
		return lifecycleNotifyDeadline
	}
	return deadline
}

func Notify(state string) error {
	return NotifyWithDeadline(state, NotifyDeadline(WatchdogInterval()))
}

func NotifyWithDeadline(state string, deadline time.Duration) error {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" || state == "" {
		return nil
	}
	if deadline <= 0 {
		deadline = lifecycleNotifyDeadline
	}
	addr := socket
	if strings.HasPrefix(addr, "@") {
		addr = "\x00" + addr[1:]
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	conn, err := notifyDial(ctx, addr)
	if err != nil {
		return fmt.Errorf("systemd notify dial failed: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(deadline))
	if _, err := conn.Write([]byte(state)); err != nil {
		return fmt.Errorf("systemd notify write failed: %w", err)
	}
	return nil
}

func dialNotifySocket(ctx context.Context, addr string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unixgram", addr)
}
