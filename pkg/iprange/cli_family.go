package iprange

import (
	"context"
	"fmt"
	"io"
)

func RunCLI(ctx context.Context, stdout, stderr io.Writer, stdin io.Reader, args []string) int {
	for _, arg := range args {
		if arg == "--has-ipv6" {
			_, _ = fmt.Fprintln(stderr, "yes, IPv6 support is present.")
			return 0
		}
	}

	family, filtered, err := detectCLIFamily(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
		return 1
	}

	if family == FamilyIPv6 {
		return runCLIV6(ctx, stdout, stderr, stdin, filtered)
	}
	return runCLIV4(ctx, stdout, stderr, stdin, filtered)
}

func detectCLIFamily(args []string) (AddressFamily, []string, error) {
	family := FamilyIPv4
	seenFamily := false
	filtered := make([]string, 0, len(args))

	for _, arg := range args {
		switch arg {
		case "-4", "--ipv4":
			if seenFamily && family != FamilyIPv4 {
				return "", nil, fmt.Errorf("cannot combine IPv4 and IPv6 flags in one invocation")
			}
			family = FamilyIPv4
			seenFamily = true
		case "-6", "--ipv6":
			if seenFamily && family != FamilyIPv6 {
				return "", nil, fmt.Errorf("cannot combine IPv4 and IPv6 flags in one invocation")
			}
			family = FamilyIPv6
			seenFamily = true
			continue
		default:
			filtered = append(filtered, arg)
		}
	}

	return family, filtered, nil
}
