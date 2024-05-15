package env

import (
	"time"

	"github.com/egdaemon/eg/internal/envx"
)

func Duration(fallback time.Duration, keys ...string) time.Duration {
	return envx.Duration(fallback, keys...)
}

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

func Debug(envs ...string) {
	envx.Debug(envs...)
}

func FromPath(n string) (environ []string, err error) {
	return envx.FromPath(n)
}
