package types

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
)

func isJournaldAvailable() bool {
	conn, err := net.Dial("unixgram", "/run/systemd/journal/socket")
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// NewKairosLogger creates a new logger with the given name and level.
// The level is used to set the log level, defaulting to info
// The log level can be overridden by setting the environment variable $NAME_DEBUG to any parseable value.
// If quiet is true, the logger will not log to the console.
func NewKairosLogger(name, level string, quiet bool) KairosLogger {
	var loggers []io.Writer
	var l zerolog.Level
	var fileLock *flock.Flock
	var logfile *os.File
	var err error

	// Add journald logger
	if isJournaldAvailable() {
		loggers = append(loggers, journald.NewJournalDWriter())
	} else {
		// Default to file logging
		logName := fmt.Sprintf("%s.log", name)
		_ = os.MkdirAll("/var/log/kairos/", os.ModeDir|os.ModePerm)
		logFileName := filepath.Join("/var/log/kairos/", logName)

		logfile, err = os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			loggers = append(loggers, zerolog.ConsoleWriter{Out: logfile, TimeFormat: time.RFC3339, NoColor: true})
		}

		fileLock = flock.New(logFileName + ".lock")
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
	// Set trace level if set on ENV
	traceFromEnv := os.Getenv(fmt.Sprintf("%s_TRACE", strings.ToUpper(name))) != ""
	if traceFromEnv {
		l = zerolog.TraceLevel
	}
	k := KairosLogger{
		zerolog.New(multi).With().Timestamp().Logger().Level(l),
		fileLock,
		logfile,
		isJournaldAvailable(),
	}

	// Set the finalizer to call the cleanup method
	runtime.SetFinalizer(&k, func(k *KairosLogger) {
		k.Cleanup()
	})

	return k
}

func (k *KairosLogger) Cleanup() {
	if k.fileLock != nil {
		k.fileLock.Lock()
		defer k.fileLock.Unlock()
	}

	if k.logFile != nil {
		k.logFile.Close()
		k.logFile = nil
	}
	if k.fileLock != nil {
		k.fileLock.Unlock()
		k.fileLock = nil
	}
}

func NewBufferLogger(b *bytes.Buffer) KairosLogger {
	return KairosLogger{
		zerolog.New(b).With().Timestamp().Logger(),
		nil,
		nil,
		true,
	}
}

func NewNullLogger() KairosLogger {
	return KairosLogger{
		zerolog.New(io.Discard).With().Timestamp().Logger(),
		nil,
		nil,
		true,
	}
}

// KairosLogger implements the bridge between zerolog and the logger.Interface that yip needs.
type KairosLogger struct {
	zerolog.Logger
	fileLock *flock.Flock
	logFile  *os.File
	journald bool // Whether we are logging to journald, to avoid the file lock
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

// Functions to implement the logger.Interface that most of our other stuff needs

func (m KairosLogger) Infof(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		// Add the pid to the log line so searching for it is easier
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Info().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Info(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Info().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Warnf(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Warn().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Warn(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Warn().Msg(fmt.Sprint(args...))
}

func (m KairosLogger) Warning(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Warn().Msg(fmt.Sprint(args...))
}

func (m KairosLogger) Warningf(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Warn().Msg(fmt.Sprintf(tpl, args...))
}

func (m KairosLogger) Debugf(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Debug().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Debug(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Debug().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Errorf(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Error().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Error(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Error().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Fatalf(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Fatal().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Fatal(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Fatal().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Panicf(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Panic().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Panic(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Panic().Msg(fmt.Sprint(args...))
}
func (m KairosLogger) Tracef(tpl string, args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		tpl = fmt.Sprintf("[%v] ", os.Getpid()) + tpl
	}
	m.Logger.Trace().Msg(fmt.Sprintf(tpl, args...))
}
func (m KairosLogger) Trace(args ...interface{}) {
	if !m.journald {
		m.fileLock.Lock()
		defer m.fileLock.Unlock()
		args = append([]interface{}{fmt.Sprintf("[%v]", os.Getpid())}, args...)
	}
	m.Logger.Trace().Msg(fmt.Sprint(args...))
}
