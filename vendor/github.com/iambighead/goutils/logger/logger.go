package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger_file zerolog.Logger
var logger_console zerolog.Logger

type Logger struct {
	name string
}

func Init(log_filename string, log_level_env_name string) {
	// create logger
	fileLogger := &lumberjack.Logger{
		Filename:   log_filename,
		MaxSize:    10,    // megabytes
		MaxBackups: 10,    // files
		MaxAge:     7,     // days
		Compress:   false, // disabled by default
	}

	loglevel := zerolog.InfoLevel
	env_LOG_LEVEL := strings.ToLower(os.Getenv(log_level_env_name))
	switch env_LOG_LEVEL {
	case "info":
		loglevel = zerolog.InfoLevel
	case "debug":
		loglevel = zerolog.DebugLevel
	case "error":
		loglevel = zerolog.ErrorLevel
	}

	logger_file = zerolog.New(fileLogger).Level(loglevel).With().Timestamp().Logger()
	logger_console = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).Level(loglevel).With().Timestamp().Logger()
}

func (mylogger *Logger) Info(msg string) {
	msg = fmt.Sprintf("%s: %s", mylogger.name, msg)
	logger_file.Info().Msg(msg)
	logger_console.Info().Msg(msg)
}

func (mylogger *Logger) Debug(msg string) {
	msg = fmt.Sprintf("%s: %s", mylogger.name, msg)
	logger_file.Debug().Msg(msg)
	logger_console.Debug().Msg(msg)
}

func (mylogger *Logger) Error(msg string) {
	msg = fmt.Sprintf("%s: %s", mylogger.name, msg)
	logger_file.Error().Msg(msg)
	logger_console.Error().Msg(msg)
}

func NewLogger(name string) Logger {
	var new_logger Logger
	new_logger.name = name
	return new_logger
}
