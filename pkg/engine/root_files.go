package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/firehol/update-ipsets/internal/fileutil"
)

func readFileInRoot(rootDir, rel string) ([]byte, error) {
	return fileutil.ReadFileUnderRoot(rootDir, rel)
}

func openFileInRoot(rootDir, rel string) (*os.File, error) {
	return fileutil.OpenFileUnderRoot(rootDir, rel)
}

func openFileWithFlagsInRoot(rootDir, rel string, flag int, perm os.FileMode) (*os.File, error) {
	return fileutil.OpenFileWithFlagsUnderRoot(rootDir, rel, flag, perm)
}

func readFilePathUnderRoot(rootDir, path string) ([]byte, error) {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return nil, fmt.Errorf("resolve relative path: %w", err)
	}
	return readFileInRoot(rootDir, rel)
}

func openFilePathUnderRoot(rootDir, path string) (*os.File, error) {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return nil, fmt.Errorf("resolve relative path: %w", err)
	}
	return openFileInRoot(rootDir, rel)
}

func openFilePathWithFlagsUnderRoot(rootDir, path string, flag int, perm os.FileMode) (*os.File, error) {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return nil, fmt.Errorf("resolve relative path: %w", err)
	}
	return openFileWithFlagsInRoot(rootDir, rel, flag, perm)
}

func joinRelativePath(elems ...string) string {
	return filepath.Join(elems...)
}
