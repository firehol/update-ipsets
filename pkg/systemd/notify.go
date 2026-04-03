package systemd

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

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

func Notify(state string) error {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" || state == "" {
		return nil
	}
	addr := socket
	if strings.HasPrefix(addr, "@") {
		addr = "\x00" + addr[1:]
	}
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: addr, Net: "unixgram"})
	if err != nil {
		return fmt.Errorf("systemd notify dial failed: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte(state)); err != nil {
		return fmt.Errorf("systemd notify write failed: %w", err)
	}
	return nil
}
