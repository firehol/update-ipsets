package engine

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// decompressGzipToFile reads a gzip-compressed source file and writes
// the decompressed contents atomically to dst (via tmp+rename). Used by
// every single-file gzip ASN format (DB-IP MMDB, iptoasn TSV, CAIDA
// prefix2as).
func decompressGzipToFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("gzip reader %s: %w", src, err)
	}
	defer func() { _ = gz.Close() }()
	tmpPath := dst + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmpPath, err)
	}
	if _, err := io.Copy(out, gz); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("decompress %s: %w", src, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s: %w", dst, err)
	}
	return nil
}

// extractMMDBFromArchive opens a tar.gz archive and writes the first
// .mmdb file found inside to dst. The MaxMind GeoLite2-ASN tar.gz
// contains the database file under a date-stamped directory along with
// a few text files; we accept any single .mmdb entry regardless of its
// directory.
func extractMMDBFromArchive(archivePath, dstPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("no .mmdb file in archive")
		}
		if err != nil {
			return fmt.Errorf("tar reader: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasSuffix(header.Name, ".mmdb") {
			continue
		}
		// Found it. Stream to a temp file and rename atomically.
		tmpPath := dstPath + ".tmp"
		out, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", tmpPath, err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("copy mmdb: %w", err)
		}
		if err := out.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("close %s: %w", tmpPath, err)
		}
		if err := os.Rename(tmpPath, dstPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rename %s: %w", dstPath, err)
		}
		return nil
	}
}
