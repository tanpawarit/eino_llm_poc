package logger

import (
	"eino_llm_poc/src/model"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

// InitLogger initializes the global logger with the provided configuration
func InitLogger(config model.LogConfig) error {
	// Set global log level
	level, err := zerolog.ParseLevel(strings.ToLower(config.Level))
	if err != nil {
		return fmt.Errorf("invalid log level '%s': %w", config.Level, err)
	}
	zerolog.SetGlobalLevel(level)

	// Configure time format
	switch strings.ToLower(config.TimeFormat) {
	case "rfc3339":
		zerolog.TimeFieldFormat = time.RFC3339
	case "unix":
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	case "iso8601":
		zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"
	default:
		zerolog.TimeFieldFormat = time.RFC3339
	}

	// Configure output writer
	var output io.Writer
	switch strings.ToLower(config.Output) {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	case "file":
		// Create logs directory if it doesn't exist
		if err := os.MkdirAll("logs", 0755); err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}
		file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file '%s': %w", config.FilePath, err)
		}
		output = file
	default:
		output = os.Stdout
	}

	// Configure log format
	if strings.ToLower(config.Format) == "console" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
		}
	}

	// Create the global logger
	Logger = zerolog.New(output).With().
		Timestamp().
		Caller().
		Logger()

	// Also set the global zerolog logger for compatibility
	log.Logger = Logger

	Logger.Info().
		Str("level", config.Level).
		Str("format", config.Format).
		Str("output", config.Output).
		Msg("Logger initialized successfully")

	return nil
}

// GetLogger returns the configured logger instance
func GetLogger() *zerolog.Logger {
	return &Logger
}

// Convenience methods for common logging patterns
func Info() *zerolog.Event {
	return Logger.Info()
}

func Debug() *zerolog.Event {
	return Logger.Debug()
}

func Warn() *zerolog.Event {
	return Logger.Warn()
}

func Error() *zerolog.Event {
	return Logger.Error()
}

func Fatal() *zerolog.Event {
	return Logger.Fatal()
}
