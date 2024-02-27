package types

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// NewKairosLogger creates a new logger with the given name and level.
// The name is used to create a log file in /run/kairos/NAME-DATE.log and /var/log/kairos/NAME-DATE.log
// The level is used to set the log level, defaulting to info
// The log level can be overridden by setting the environment variable $NAME_DEBUG to any parseable value.
// If quiet is true, the logger will not log to the console.
func NewKairosLogger(name, level string, quiet bool) KairosLogger {
	var loggers []io.Writer
	var l zerolog.Level

	// Have I ever mentioned how terrible the format of time is in golang?
	// Whats with this 20060102150405 format? Do anyone actually remembers that?
	logName := fmt.Sprintf("%s-%s.log", name, time.Now().Format("20060102150405"))
	_ = os.MkdirAll("/run/kairos/", os.ModeDir|os.ModePerm)
	_ = os.MkdirAll("/var/log/kairos/", os.ModeDir|os.ModePerm)
	logfileRun, err := os.Create(filepath.Join("/run/kairos/", logName))
	if err == nil {
		loggers = append(loggers, zerolog.ConsoleWriter{Out: logfileRun, TimeFormat: time.RFC3339, NoColor: true})
	}
	logfileVar, err := os.Create(filepath.Join("/var/log/kairos/", logName))
	if err == nil {
		loggers = append(loggers, zerolog.ConsoleWriter{Out: logfileVar, TimeFormat: time.RFC3339, NoColor: true})
	}

	if !quiet {
		loggers = append(loggers, zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.TimeFormat = time.RFC3339
		}))
	}

	// Parse the level, default to info
	l, err = zerolog.ParseLevel(level)
	if err != nil {
		l = zerolog.InfoLevel
	}

	multi := zerolog.MultiLevelWriter(loggers...)

	// Set debug level if set on ENV
	debugFromEnv := os.Getenv(fmt.Sprintf("%s_DEBUG", strings.ToUpper(name))) != ""
	if debugFromEnv {
		l = zerolog.DebugLevel
	}
	k := KairosLogger{
		zerolog.New(multi).With().Timestamp().Logger().Level(l),
		loggers,
	}

	return k
}

func NewBufferLogger(b *bytes.Buffer) KairosLogger {
	return KairosLogger{
		zerolog.New(b).With().Timestamp().Logger(),
		[]io.Writer{},
	}
}

func NewNullLogger() KairosLogger {
	return KairosLogger{
		zerolog.New(io.Discard).With().Timestamp().Logger(),
		[]io.Writer{},
	}
}

// KairosLogger implements the bridge between zerolog and the logger.Interface that yip needs.
type KairosLogger struct {
	zerolog.Logger
	logFiles []io.Writer
}

func (m *KairosLogger) SetLevel(level string) {
	l, _ := zerolog.ParseLevel(level)
	// I think this returns a full child logger so we need to overwrite the logger
	m.Logger = m.Logger.Level(l)
}

func (m KairosLogger) GetLevel() zerolog.Level {
	return m.Logger.GetLevel()
}

func (m KairosLogger) IsDebug() bool {
	return m.Logger.GetLevel() == zerolog.DebugLevel
}

// Close Try to close all log files
func (m KairosLogger) Close() {
	for _, f := range m.logFiles {
		if c, ok := f.(io.Closer); ok {
			_ = c.Close()
		}
	}
}

// Functions to implement the logger.Interface that most of our other stuff needs

func (m KairosLogger) Infof(tpl string, args ...interface{}) {
	m.Logger.Info().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Info(args ...interface{}) {
	m.Logger.Info().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Warnf(tpl string, args ...interface{}) {
	m.Logger.Warn().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Warn(args ...interface{}) {
	m.Logger.Warn().Msg(fmt.Sprint(args...))
}

func (m KairosLogger) Warning(args ...interface{}) {
	m.Logger.Warn().Msg(fmt.Sprint(args...))
}

func (m KairosLogger) Debugf(tpl string, args ...interface{}) {
	m.Logger.Debug().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Debug(args ...interface{}) {
	m.Logger.Debug().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Errorf(tpl string, args ...interface{}) {
	m.Logger.Error().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Error(args ...interface{}) {
	m.Logger.Error().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Fatalf(tpl string, args ...interface{}) {
	m.Logger.Fatal().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Fatal(args ...interface{}) {
	m.Logger.Fatal().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Panicf(tpl string, args ...interface{}) {
	m.Logger.Panic().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Panic(args ...interface{}) {
	m.Logger.Panic().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Tracef(tpl string, args ...interface{}) {
	m.Logger.Trace().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Trace(args ...interface{}) {
	m.Logger.Trace().Msg(fmt.Sprint(args...))
}
