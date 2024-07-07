package log

import (
	"testing"
	"time"
)

// wraps around testing.T.Logf to add a timestamp prefix
func Logf(t *testing.T, format string, args ...any) {
	currentTime := time.Now().Format("[15:04:05.000]")
	prefixedFormat := currentTime + " " + format
	t.Logf(prefixedFormat, args...)
}
