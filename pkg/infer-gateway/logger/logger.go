package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logSubsys = "subsys"
)

var (
	defaultLogger  = initDefaultLogger()
	fileOnlyLogger = initFileLogger()

	// defaultLogLevel = logrus.InfoLevel
	defaultLogLevel = logrus.DebugLevel
	defaultLogFile  = "/var/run/kmesh/ai-gateway.log"

	defaultLogFormat = &logrus.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: false,
	}

	loggerMap = map[string]*logrus.Logger{
		"default":  defaultLogger,
		"fileOnly": fileOnlyLogger,
	}
)

func SetLoggerLevel(loggerName string, level logrus.Level) error {
	logger, exists := loggerMap[loggerName]
	if !exists || logger == nil {
		return fmt.Errorf("logger %s does not exist", loggerName)
	}
	logger.SetLevel(level)
	return nil
}

func GetLoggerLevel(loggerName string) (logrus.Level, error) {
	logger, exists := loggerMap[loggerName]
	if !exists || logger == nil {
		return 0, fmt.Errorf("logger %s does not exist", loggerName)
	}
	return logger.Level, nil
}

func GetLoggerNames() []string {
	names := make([]string, 0, len(loggerMap))
	for loggerName := range loggerMap {
		names = append(names, loggerName)
	}
	return names
}

// initDefaultLogger return a default logger
func initDefaultLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(defaultLogFormat)
	logger.SetLevel(defaultLogLevel)
	return logger
}

// initFileLogger return a file only logger
func initFileLogger() *logrus.Logger {
	logger := initDefaultLogger()
	logFilePath := defaultLogFile
	path, fileName := filepath.Split(logFilePath)
	err := os.MkdirAll(path, 0o700)
	if err != nil {
		logger.Warnf("failed to create log directory: %v, consider running with root user", err)
		// if error occurs, fall back to current working directory
		logFilePath = fileName
	}

	logfile := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28,    //days
		Compress:   false, // disabled by default
	}
	logger.SetOutput(io.Writer(logfile))
	return logger
}

// NewLogger allocates a new log entry for a specific scope.
func NewLogger(subsys string) *logrus.Entry {
	if subsys == "" {
		return logrus.NewEntry(defaultLogger)
	}
	return defaultLogger.WithField(logSubsys, subsys)
}

// NewFileLogger don't output log to stdout
func NewFileLogger(pkgSubsys string) *logrus.Entry {
	if pkgSubsys == "" {
		return logrus.NewEntry(fileOnlyLogger)
	}
	return fileOnlyLogger.WithField(logSubsys, pkgSubsys)
}
