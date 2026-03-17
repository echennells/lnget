package build

import (
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btclog/v2"
)

// LogType indicates the type of logging specified by the build flag.
type LogType byte

const (
	// LogTypeNone indicates no logging.
	LogTypeNone LogType = iota

	// LogTypeStdOut indicates all logging is written directly to stdout.
	LogTypeStdOut

	// LogTypeDefault logs to both stdout and a given io.PipeWriter.
	LogTypeDefault
)

// String returns a human readable identifier for the logging type.
func (t LogType) String() string {
	switch t {
	case LogTypeNone:
		return "none"

	case LogTypeStdOut:
		return "stdout"

	case LogTypeDefault:
		return "default"

	default:
		return "unknown"
	}
}

// logManager is the central log manager that all packages register
// their subsystem loggers with. It allows log levels to be set
// globally or per-subsystem via a debuglevel string.
var logManager = newLogMgr()

// logMgr tracks all registered subsystem loggers and the shared
// backend handler.
type logMgr struct {
	handler *btclog.DefaultHandler
	loggers map[string]SubLoggerEntry
	logFile *os.File
}

// newLogMgr creates a new log manager. Logging starts disabled; call
// SetLogFile to direct output to a file before enabling levels.
func newLogMgr() *logMgr {
	// Use a discard handler until SetLogFile is called.
	handler := btclog.NewDefaultHandler(os.Stderr)

	return &logMgr{
		handler: handler,
		loggers: make(map[string]SubLoggerEntry),
	}
}

// SetLogFile opens the given path for append-mode writing and
// redirects all subsystem loggers to it. The caller should call
// CloseLogFile when done.
func SetLogFile(path string) error {
	f, err := os.OpenFile(
		path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600,
	)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w",
			path, err)
	}

	handler := btclog.NewDefaultHandler(f)

	// Re-register all existing loggers with the new handler
	// and call each package's setter so the package-level var
	// points to the new logger.
	for subsystem, entry := range logManager.loggers {
		newLogger := btclog.NewSLogger(
			handler.SubSystem(subsystem),
		)

		// Preserve the current log level.
		newLogger.SetLevel(entry.logger.Level())

		entry.logger = newLogger
		logManager.loggers[subsystem] = entry

		if entry.setter != nil {
			entry.setter(newLogger)
		}
	}

	logManager.handler = handler
	logManager.logFile = f

	return nil
}

// CloseLogFile closes the log file if one is open.
func CloseLogFile() {
	if logManager.logFile != nil {
		_ = logManager.logFile.Close()
		logManager.logFile = nil
	}
}

// SubLoggerEntry tracks a subsystem's logger and the setter function
// that updates the package-level variable when the handler changes.
type SubLoggerEntry struct {
	logger btclog.Logger
	setter func(btclog.Logger)
}

// RegisterSubLogger creates and registers a new subsystem logger with
// the central log manager. The setter function is called whenever the
// log backend changes (e.g. SetLogFile) so that the package-level
// variable stays in sync.
//
//nolint:whitespace,wsl_v5
func RegisterSubLogger(subsystem string,
	setter func(btclog.Logger)) btclog.Logger {

	logger := btclog.NewSLogger(
		logManager.handler.SubSystem(subsystem),
	)

	// Default to info so all subsystems log to file by default.
	logger.SetLevel(btclog.LevelInfo)

	logManager.loggers[subsystem] = SubLoggerEntry{
		logger: logger,
		setter: setter,
	}

	return logger
}

// SetLogLevel sets the log level for a single logger.
func SetLogLevel(logger btclog.Logger, level string) {
	lvl, ok := btclog.LevelFromString(level)
	if !ok {
		return
	}

	logger.SetLevel(lvl)
}

// SetAllLogLevels sets the given level on every registered subsystem.
func SetAllLogLevels(level string) {
	lvl, ok := btclog.LevelFromString(level)
	if !ok {
		return
	}

	for _, entry := range logManager.loggers {
		entry.logger.SetLevel(lvl)
	}
}

// ParseAndSetDebugLevels parses a debuglevel string and applies the
// levels to the registered subsystem loggers. The format mirrors lnd:
//
//   - A single level applies to all subsystems: "debug", "trace"
//   - Comma-separated subsystem=level pairs for granular control:
//     "LNCE=debug,L402=trace,CLNT=info"
func ParseAndSetDebugLevels(debugLevel string) error {
	if debugLevel == "" {
		return nil
	}

	// Check if it's a single global level.
	_, ok := btclog.LevelFromString(debugLevel)
	if ok {
		SetAllLogLevels(debugLevel)

		return nil
	}

	// Otherwise, parse comma-separated subsystem=level pairs.
	for pair := range strings.SplitSeq(debugLevel, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid debuglevel pair: %q "+
				"(expected SUBSYSTEM=LEVEL)", pair)
		}

		subsystem := strings.TrimSpace(parts[0])
		levelStr := strings.TrimSpace(parts[1])

		entry, exists := logManager.loggers[subsystem]
		if !exists {
			return fmt.Errorf("unknown subsystem %q, "+
				"available: %s", subsystem,
				strings.Join(SubsystemNames(), ", "))
		}

		lvl, ok := btclog.LevelFromString(levelStr)
		if !ok {
			return fmt.Errorf("unknown log level %q for "+
				"subsystem %s", levelStr, subsystem)
		}

		entry.logger.SetLevel(lvl)
	}

	return nil
}

// SubsystemNames returns a sorted list of registered subsystem names.
func SubsystemNames() []string {
	names := make([]string, 0, len(logManager.loggers))
	for name := range logManager.loggers {
		names = append(names, name)
	}

	return names
}

// NewSubLogger constructs a new subsystem logger. For lnget, we use a
// simple stdout-based logger.
//
//nolint:whitespace,wsl_v5
func NewSubLogger(subsystem string,
	gen func(string) btclog.Logger) btclog.Logger {

	if gen != nil {
		return gen(subsystem)
	}

	// Default to stdout logging.
	backend := btclog.NewDefaultHandler(os.Stdout)
	logger := btclog.NewSLogger(backend.SubSystem(subsystem))

	return logger
}

// NewDefaultLogger creates a default logger that writes to stdout.
func NewDefaultLogger(subsystem string) btclog.Logger {
	backend := btclog.NewDefaultHandler(os.Stdout)

	return btclog.NewSLogger(backend.SubSystem(subsystem))
}
