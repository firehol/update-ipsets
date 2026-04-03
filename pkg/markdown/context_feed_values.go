package markdown

import "fmt"

func strVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func intVal(v any) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func firstIntVal(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if n := intVal(m[key]); n != 0 {
			return n
		}
	}
	return 0
}

func int64Val(v any) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}

func uintVal(v any) uint64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case int:
		return uint64(n)
	case uint64:
		return n
	case int64:
		return uint64(n)
	}
	return 0
}

func uint32Val(v any) uint32 {
	return uint32(uintVal(v))
}

func float64Val(v any) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func boolVal(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}
