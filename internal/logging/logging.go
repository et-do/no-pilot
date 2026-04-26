package logging

import "strings"

// Level controls log verbosity.
type Level int

const (
	LevelError Level = iota
	LevelInfo
	LevelDebug
)

// ParseLevel parses a textual level and defaults to info for unknown values.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return LevelInfo
	case "error":
		return LevelError
	case "debug":
		return LevelDebug
	default:
		return LevelInfo
	}
}

// Enabled reports whether msgLevel should be emitted under current level.
func Enabled(current, msgLevel Level) bool {
	return msgLevel <= current
}

// String returns a printable log level name.
func String(level Level) string {
	switch level {
	case LevelError:
		return "error"
	case LevelDebug:
		return "debug"
	default:
		return "info"
	}
}
