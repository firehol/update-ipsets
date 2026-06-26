package iprange

import (
	"bufio"
	"errors"
	"io"
)

func forEachTextLineBytes(br *bufio.Reader, fn func([]byte) error) error {
	if br == nil || fn == nil {
		return nil
	}
	var buffered []byte
	for {
		line, err := br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			buffered = append(buffered, line...)
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if len(buffered) > 0 {
			buffered = append(buffered, line...)
			line = buffered
		}
		if len(line) > 0 {
			if callErr := fn(line); callErr != nil {
				return callErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if len(buffered) > 0 {
			buffered = buffered[:0]
		}
	}
}
