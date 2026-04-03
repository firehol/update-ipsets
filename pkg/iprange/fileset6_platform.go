//go:build !(linux || darwin)

package iprange

import "os"

func openFileSet6Platform(f *os.File, path string, fileSize int64, hdr binaryHeader6) (FileSet6, error) {
	return openFileSet6Pread(f, path, fileSize, hdr)
}
