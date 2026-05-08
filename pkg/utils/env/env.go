package env

import (
	"os"
	"strconv"
)

func ParseEnvWithFallback[T any](envName string, fallback T) T {
	val := os.Getenv(envName)
	if val == "" {
		return fallback
	}

	var result any
	switch any(fallback).(type) {
	case string:
		result = val
	case int:
		parsed, err := strconv.Atoi(val)
		if err != nil {
			return fallback
		}
		result = parsed
	case bool:
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			return fallback
		}
		result = parsed
	case float64:
		parsed, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fallback
		}
		result = parsed
	default:
		return fallback
	}

	return result.(T)
}
