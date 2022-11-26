package env

import "github.com/james-lawrence/eg/internal/envx"

func Boolean(fallback bool, keys ...string) bool {
	return envx.Boolean(fallback, keys...)
}

func String(fallback string, keys ...string) string {
	return envx.String(fallback, keys...)
}

func Int(fallback int, keys ...string) int {
	return envx.Int(fallback, keys...)
}

func Float64(fallback float64, keys ...string) float64 {
	return envx.Float64(fallback, keys...)
}
