package config

import (
	"log/slog"
	"os"
	"strings"
)

type PrettyHandler struct {
	level slog.Level
}

func NewLogger() *slog.Logger {
	level := slog.LevelInfo

	switch strings.ToLower(os.Getenv("LOGLEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler)
}