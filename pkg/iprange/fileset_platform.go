//go:build !(linux || darwin)

package iprange

import "os"

// openFileSetPlatform on non-mmap platforms uses pread directly.
func openFileSetPlatform(f *os.File, path string, fileSize int64, hdr binaryHeader, opts FileSetOpenOptions) (FileSet, error) {
	return openFileSetPread(f, path, fileSize, hdr, opts)
}
