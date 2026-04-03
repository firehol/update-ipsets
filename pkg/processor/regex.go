package processor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

func init() {
	registry["regex"] = regexExtract
}

var regexCache sync.Map // pattern string → *regexp.Regexp

func cachedRegexp(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		return v.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.Store(pattern, re)
	return re, nil
}

func regexExtract(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	pattern := strings.TrimSpace(args["pattern"])
	if pattern == "" {
		pattern = strings.TrimSpace(args["value"])
	}
	if pattern == "" {
		return nil, fmt.Errorf("missing regex pattern")
	}
	re, err := cachedRegexp(pattern)
	if err != nil {
		return nil, err
	}
	matches := re.FindAllStringSubmatch(string(input), -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			value := strings.TrimSpace(match[1])
			if value != "" {
				out = append(out, value)
			}
			continue
		}
		value := strings.TrimSpace(match[0])
		if value != "" {
			out = append(out, value)
		}
	}
	return joinLines(out), nil
}
