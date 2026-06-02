package engine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecompressGzipToFileRejectsExpandedPayloadOverLimit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "provider.tsv.gz")
	dst := filepath.Join(dir, "provider.tsv")
	writeGzipFixture(t, src, []byte("01234567890"))

	err := decompressGzipToFileWithLimit(src, dst, 10)
	if err == nil || !strings.Contains(err.Error(), "expanded payload exceeds") {
		t.Fatalf("error = %v, want expanded payload limit error", err)
	}
	assertPathMissing(t, dst)
	assertPathMissing(t, dst+".tmp")
}

func TestDecompressGzipToFileWritesGeneratedOutput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "provider.tsv.gz")
	dst := filepath.Join(dir, "provider.tsv")
	want := []byte("1.2.3.0/24\t64500\n")
	writeGzipFixture(t, src, want)

	if err := decompressGzipToFileWithLimit(src, dst, 1024); err != nil {
		t.Fatal(err)
	}
	assertFileBodyAndMode(t, dst, want, generatedFileMode)
	assertPathMissing(t, dst+".tmp")
}

func TestExtractMMDBFromArchiveRejectsExpandedPayloadOverLimit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "provider.tar.gz")
	dst := filepath.Join(dir, "provider.mmdb")
	writeTarGzipMMDBFixture(t, src, "GeoLite2-ASN/GeoLite2-ASN.mmdb", []byte("01234567890"))

	err := extractMMDBFromArchiveWithLimit(src, dst, 10)
	if err == nil || !strings.Contains(err.Error(), "expanded payload exceeds") {
		t.Fatalf("error = %v, want expanded payload limit error", err)
	}
	assertPathMissing(t, dst)
	assertPathMissing(t, dst+".tmp")
}

func TestExtractMMDBFromArchiveWritesGeneratedOutput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "provider.tar.gz")
	dst := filepath.Join(dir, "provider.mmdb")
	want := []byte("mmdb fixture")
	writeTarGzipMMDBFixture(t, src, "GeoLite2-ASN/GeoLite2-ASN.mmdb", want)

	if err := extractMMDBFromArchiveWithLimit(src, dst, 1024); err != nil {
		t.Fatal(err)
	}
	assertFileBodyAndMode(t, dst, want, generatedFileMode)
	assertPathMissing(t, dst+".tmp")
}

func writeGzipFixture(t *testing.T, path string, body []byte) {
	t.Helper()
	var payload bytes.Buffer
	gw := gzip.NewWriter(&payload)
	if _, err := gw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeTarGzipMMDBFixture(t *testing.T, path, name string, body []byte) {
	t.Helper()
	var payload bytes.Buffer
	gw := gzip.NewWriter(&payload)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileBodyAndMode(t *testing.T, path string, wantBody []byte, wantMode os.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wantBody) {
		t.Fatalf("body = %q, want %q", got, wantBody)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != wantMode {
		t.Fatalf("mode = %04o, want %04o", gotMode, wantMode)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s error = %v, want missing path", path, err)
	}
}
