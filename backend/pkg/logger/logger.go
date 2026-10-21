package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	Log = slog.New(handler)
	slog.SetDefault(Log)
}

// Info registra mensagem informativa com campos estruturados
func Info(msg string, args ...any) {
	Log.Info(msg, args...)
}

// Warn registra aviso com campos estruturados
func Warn(msg string, args ...any) {
	Log.Warn(msg, args...)
}

// Error registra erro com campos estruturados
func Error(msg string, args ...any) {
	Log.Error(msg, args...)
}

// Debug registra mensagem de depuração
func Debug(msg string, args ...any) {
	Log.Debug(msg, args...)
}
