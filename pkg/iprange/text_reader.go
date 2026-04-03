package iprange

import (
	"bufio"
	"errors"
	"io"
)

func forEachTextLine(br *bufio.Reader, fn func(string) error) error {
	if br == nil || fn == nil {
		return nil
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if len(line) > 0 {
			if callErr := fn(line); callErr != nil {
				return callErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}
