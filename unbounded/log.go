package unbounded

import (
	"io"
	"log/slog"
)

// SetupLogging routes broflake's slog output, which is the only logging it
// does, to w at the given level. Returns the previous default logger.
func SetupLogging(w io.Writer, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})))
	return prev
}
