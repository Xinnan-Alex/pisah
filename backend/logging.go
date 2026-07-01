package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func setupLogger() *slog.Logger {
	level := parseLogLevel(envOr("LOG_LEVEL", "info"))
	format := strings.ToLower(envOr("LOG_FORMAT", "text"))

	opts := &slog.HandlerOptions{Level: level, AddSource: level == slog.LevelDebug}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(sw, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", sw.bytes,
		}
		if q := r.URL.RawQuery; q != "" {
			attrs = append(attrs, "query", q)
		}
		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, "user_agent", ua)
		}

		switch {
		case sw.status >= 500:
			slog.ErrorContext(r.Context(), "http request", attrs...)
		case sw.status >= 400:
			slog.WarnContext(r.Context(), "http request", attrs...)
		case r.URL.Path == "/healthz":
			slog.DebugContext(r.Context(), "http request", attrs...)
		default:
			slog.InfoContext(r.Context(), "http request", attrs...)
		}
	})
}

func withRecover(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "panic recovered",
					"recover", rec,
					"method", r.Method,
					"path", r.URL.Path,
				)
				writeErr(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		h.ServeHTTP(w, r)
	})
}

func writeErrWithLog(r *http.Request, w http.ResponseWriter, code int, msg string, err error) {
	if err != nil && code >= 500 {
		slog.ErrorContext(r.Context(), msg,
			"error", err,
			"status", code,
			"method", r.Method,
			"path", r.URL.Path,
		)
	} else if code >= 400 {
		slog.DebugContext(r.Context(), msg,
			"status", code,
			"method", r.Method,
			"path", r.URL.Path,
		)
	}
	writeErr(w, code, msg)
}
