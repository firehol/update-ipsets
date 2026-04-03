package processor

import (
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"strings"
)

// Streaming implementations for IP filters, format-specific parsers,
// and decompression. These are registered in stream.go's init().

func streamFilterIP4(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSuffix(strings.TrimSpace(line), "/32")
		if line != "" && net.ParseIP(line).To4() != nil {
			return []string{line}
		}
		return nil
	}), nil
}

func streamFilterNet4(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSuffix(strings.TrimSpace(line), "/32")
		if _, _, err := net.ParseCIDR(line); err == nil {
			if !strings.HasSuffix(line, "/32") && !strings.HasSuffix(line, "/0") {
				return []string{line}
			}
		}
		return nil
	}), nil
}

func streamFilterAll4(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			return nil
		case net.ParseIP(line).To4() != nil:
			return []string{line}
		case isCIDRv4(line):
			return []string{line}
		}
		return nil
	}), nil
}

func streamFilterInvalid4(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSpace(line)
		if line == "" || line == "0.0.0.0" || strings.HasSuffix(line, "/0") {
			return nil
		}
		return []string{line}
	}), nil
}

func streamFilterIP6(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSuffix(strings.TrimSpace(line), "/128")
		if ip := net.ParseIP(line); ip != nil && ip.To4() == nil {
			return []string{line}
		}
		return nil
	}), nil
}

func streamFilterNet6(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSuffix(strings.TrimSpace(line), "/128")
		if _, network, err := net.ParseCIDR(line); err == nil && network.IP.To4() == nil {
			return []string{line}
		}
		return nil
	}), nil
}

func streamFilterAll6(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			return nil
		case isIPv6(line):
			return []string{line}
		case isCIDRv6(line):
			return []string{line}
		}
		return nil
	}), nil
}

func streamSnortRules(r io.Reader, _ map[string]string) (io.Reader, error) {
	stripped := streamRemoveComments(r, "#")
	return newLineFilterReader(stripped, func(line string) []string {
		if !strings.HasPrefix(line, "alert ") {
			return nil
		}
		start := strings.IndexByte(line, '[')
		end := strings.IndexByte(line, ']')
		if start < 0 || end <= start {
			return nil
		}
		var out []string
		for _, part := range strings.Split(line[start+1:end], ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	}), nil
}

func streamPixDenyRules(r io.Reader, _ map[string]string) (io.Reader, error) {
	stripped := streamRemoveComments(r, "#")
	return newLineFilterReader(stripped, func(line string) []string {
		fields := strings.Fields(line)
		if len(fields) < 7 || fields[0] != "access-list" || !strings.Contains(line, " deny ip ") {
			return nil
		}
		if idx := strings.Index(line, " deny ip host "); idx >= 0 {
			rest := line[idx+len(" deny ip host "):]
			host := strings.Fields(rest)
			if len(host) > 0 {
				return []string{host[0]}
			}
			return nil
		}
		if idx := strings.Index(line, " deny ip "); idx >= 0 {
			rest := strings.Fields(line[idx+len(" deny ip "):])
			if len(rest) >= 2 {
				return []string{rest[0] + "/" + rest[1]}
			}
		}
		return nil
	}), nil
}

func streamDshieldFormat(r io.Reader, _ map[string]string) (io.Reader, error) {
	stripped := streamRemoveComments(r, "#")
	return newLineFilterReader(stripped, func(line string) []string {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil
		}
		if fields[0] == "" || fields[0][0] < '1' || fields[0][0] > '9' {
			return nil
		}
		return []string{fields[0] + "/" + fields[2]}
	}), nil
}

func streamDataplaneColumn3(r io.Reader, _ map[string]string) (io.Reader, error) {
	stripped := streamRemoveComments(r, "#")
	return newLineFilterReader(stripped, func(line string) []string {
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			return nil
		}
		value := strings.Join(strings.Fields(fields[2]), " ")
		if value == "" {
			return nil
		}
		return []string{value}
	}), nil
}

func streamGunzip(r io.Reader, _ map[string]string) (io.Reader, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	// Wrap with a size limit to protect against gzip bombs.
	return &limitedGzipReader{reader: gr, remaining: maxDecompressedSize}, nil
}

// limitedGzipReader wraps a gzip reader with a decompression size limit.
// Once the limit is exceeded, Read returns an error. It also implements
// io.Closer to properly close the underlying gzip reader.
type limitedGzipReader struct {
	reader    io.ReadCloser
	remaining int64
}

func (lr *limitedGzipReader) Read(p []byte) (int, error) {
	if lr.remaining <= 0 {
		return 0, fmt.Errorf("decompressed data exceeds %d bytes limit", maxDecompressedSize)
	}
	if int64(len(p)) > lr.remaining {
		p = p[:lr.remaining]
	}
	n, err := lr.reader.Read(p)
	lr.remaining -= int64(n)
	return n, err
}

func (lr *limitedGzipReader) Close() error {
	return lr.reader.Close()
}
