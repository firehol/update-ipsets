package engine

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxASNExpandedFileBytes int64 = 2 << 30

// decompressGzipToFile reads a gzip-compressed source file and writes
// the decompressed contents atomically to dst (via tmp+rename). Used by
// every single-file gzip ASN format (DB-IP MMDB, iptoasn TSV, CAIDA
// prefix2as).
func decompressGzipToFile(src, dst string) error {
	return decompressGzipToFileWithLimit(src, dst, maxASNExpandedFileBytes)
}

func decompressGzipToFileWithLimit(src, dst string, limit int64) error {
	in, err := openFilePathUnderRoot(filepath.Dir(src), src)
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
	out, err := openGeneratedTempFile(tmpPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmpPath, err)
	}
	if err := copyExpandedPayloadWithLimit(out, gz, limit); err != nil {
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
	return extractMMDBFromArchiveWithLimit(archivePath, dstPath, maxASNExpandedFileBytes)
}

func extractMMDBFromArchiveWithLimit(archivePath, dstPath string, limit int64) error {
	f, err := openFilePathUnderRoot(filepath.Dir(archivePath), archivePath)
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
		if header.Size < 0 {
			return fmt.Errorf("mmdb entry %s has negative size %d", header.Name, header.Size)
		}
		if header.Size > limit {
			return fmt.Errorf("mmdb entry %s expanded payload exceeds %d-byte limit", header.Name, limit)
		}
		// Found it. Stream to a temp file and rename atomically.
		tmpPath := dstPath + ".tmp"
		out, err := openGeneratedTempFile(tmpPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", tmpPath, err)
		}
		if err := copyExpandedPayloadWithLimit(out, tr, limit); err != nil {
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

func openGeneratedTempFile(path string) (*os.File, error) {
	file, err := openFilePathWithFlagsUnderRoot(filepath.Dir(path), path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, generatedFileMode)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(generatedFileMode); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return file, nil
}

func copyExpandedPayloadWithLimit(dst io.Writer, src io.Reader, limit int64) error {
	if limit <= 0 {
		return fmt.Errorf("expanded payload limit must be positive")
	}
	written, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return err
	}
	if written > limit {
		return fmt.Errorf("expanded payload exceeds %d-byte limit", limit)
	}
	return nil
}
