package appmiddleware

import (
	"context"
	"log/slog"
)

type contextKey string

const loggerKey contextKey = "logger"

func withLogger(
	ctx context.Context,
	logger *slog.Logger,
) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}

	return slog.Default()
}
