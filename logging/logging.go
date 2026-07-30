package logging

import (
	"log/slog"
	"os"
)

func New(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "DEBUG":
		l = slog.LevelDebug
	case "ERROR":
		l = slog.LevelError
	case "WARN":
		l = slog.LevelWarn
	case "INFO":
		l = slog.LevelInfo
	default:
		l = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})
	return slog.New(handler)
}
