//go:build !linux && !darwin

package iprange

import "context"

func overlapCountFastPathPlatform(context.Context, RangeSource, RangeSource) (uint64, bool, error) {
	return 0, false, nil
}
