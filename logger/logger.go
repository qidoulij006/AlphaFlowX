package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// Log is the global logger instance
	Log *logrus.Logger
	// logFile holds the current log file handle
	logFile *os.File
)

const (
	maxLogMessageBytes = 16 * 1024
	maxLogFileSize     = 50 * 1024 * 1024
)

type rotatingFileWriter struct {
	mu          sync.Mutex
	dir         string
	baseName    string
	currentDate string
	file        *os.File
}

// compactFormatter is a custom formatter for cleaner log output
type compactFormatter struct {
	logrus.TextFormatter
}

func (f *compactFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())[0:4]
	timestamp := entry.Time.Format("01-02 15:04:05")

	// Skip frames to find actual caller (skip logrus + our wrapper functions)
	caller := ""
	for i := 3; i < 10; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Skip logrus internal and our logger.go
		if !strings.Contains(file, "logrus") && !strings.HasSuffix(file, "logger/logger.go") {
			// Get package name from path (e.g., "nofx/manager/trader_manager.go" -> "manager")
			dir := filepath.Dir(file)
			pkg := filepath.Base(dir)
			caller = fmt.Sprintf("%s/%s:%d", pkg, filepath.Base(file), line)
			break
		}
	}

	msg := fmt.Sprintf("%s [%s] %s %s\n", timestamp, level, caller, truncateLogMessage(entry.Message))
	return []byte(msg), nil
}

func init() {
	// Auto-initialize default logger to ensure it works before Init is called
	Log = logrus.New()
	Log.SetLevel(logrus.InfoLevel)
	Log.SetFormatter(&compactFormatter{})
	Log.SetOutput(os.Stdout)
	Log.SetReportCaller(false)
}

// ============================================================================
// Initialization functions
// ============================================================================

// Init initializes the global logger
// If config is nil, uses default configuration (console output, info level)
func Init(cfg *Config) error {
	Log = logrus.New()

	// Use default values if no config provided
	if cfg == nil {
		cfg = &Config{Level: "info"}
	}

	// Set default values
	cfg.SetDefaults()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// Set compact formatter
	Log.SetFormatter(&compactFormatter{})

	// Setup log file output (write to both stdout and file)
	logDir := "data"
	if err := os.MkdirAll(logDir, 0755); err == nil {
		rotatingWriter, err := newRotatingFileWriter(logDir, "nofx")
		if err == nil {
			// Write to both stdout and file
			Log.SetOutput(io.MultiWriter(os.Stdout, rotatingWriter))
		} else {
			Log.SetOutput(os.Stdout)
		}
	} else {
		Log.SetOutput(os.Stdout)
	}

	Log.SetReportCaller(false)

	return nil
}

// InitWithSimpleConfig initializes logger with simplified config
// Suitable for scenarios that only need basic functionality
func InitWithSimpleConfig(level string) error {
	return Init(&Config{Level: level})
}

// Shutdown gracefully shuts down the logger
func Shutdown() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func truncateLogMessage(msg string) string {
	if len(msg) <= maxLogMessageBytes {
		return msg
	}
	return fmt.Sprintf("%s... [truncated %d bytes]", msg[:maxLogMessageBytes], len(msg)-maxLogMessageBytes)
}

func newRotatingFileWriter(dir, baseName string) (*rotatingFileWriter, error) {
	w := &rotatingFileWriter{
		dir:      dir,
		baseName: baseName,
	}
	if err := w.rotateIfNeededLocked(time.Now()); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeededLocked(time.Now()); err != nil {
		return 0, err
	}

	if w.file == nil {
		return 0, fmt.Errorf("log file is not initialized")
	}

	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}

	if err := w.rotateIfNeededLocked(time.Now()); err != nil {
		return n, err
	}

	return n, nil
}

func (w *rotatingFileWriter) rotateIfNeededLocked(now time.Time) error {
	date := now.Format("2006-01-02")
	needReopen := w.file == nil || w.currentDate != date

	if !needReopen && w.file != nil {
		if info, err := w.file.Stat(); err == nil && info.Size() >= maxLogFileSize {
			rotatedName := filepath.Join(w.dir, fmt.Sprintf("%s_%s_%s.log", w.baseName, w.currentDate, now.Format("150405")))
			if err := w.file.Close(); err != nil {
				return err
			}
			if err := os.Rename(w.currentLogPath(w.currentDate), rotatedName); err != nil {
				return err
			}
			logFile = nil
			w.file = nil
			needReopen = true
		}
	}

	if !needReopen {
		return nil
	}

	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		logFile = nil
	}

	f, err := os.OpenFile(w.currentLogPath(date), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w.file = f
	w.currentDate = date
	logFile = f
	return nil
}

func (w *rotatingFileWriter) currentLogPath(date string) string {
	return filepath.Join(w.dir, fmt.Sprintf("%s_%s.log", w.baseName, date))
}

// ============================================================================
// Logging functions
// ============================================================================

// WithFields creates logger entry with fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log.WithFields(fields)
}

// WithField creates logger entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return Log.WithField(key, value)
}

// add debug, info, warn
func Debug(args ...interface{}) {
	Log.Debug(args...)
}

func Info(args ...interface{}) {
	Log.Info(args...)
}

func Warn(args ...interface{}) {
	Log.Warn(args...)
}

func Debugf(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	Log.Warnf(format, args...)
}

func Error(args ...interface{}) {
	Log.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	Log.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

func Panic(args ...interface{}) {
	Log.Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	Log.Panicf(format, args...)
}

// ============================================================================
// MCP Logger adapter
// ============================================================================

// MCPLogger adapter that allows MCP package to use the global logger
// Implements mcp.Logger interface
type MCPLogger struct{}

// NewMCPLogger creates MCP log adapter
func NewMCPLogger() *MCPLogger {
	return &MCPLogger{}
}

func (l *MCPLogger) Debugf(format string, args ...any) {
	Log.Debugf(format, args...)
}

func (l *MCPLogger) Infof(format string, args ...any) {
	Log.Infof(format, args...)
}

func (l *MCPLogger) Warnf(format string, args ...any) {
	Log.Warnf(format, args...)
}

func (l *MCPLogger) Errorf(format string, args ...any) {
	Log.Errorf(format, args...)
}
