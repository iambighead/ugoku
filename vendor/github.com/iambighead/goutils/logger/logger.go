package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/RackSec/srslog"
)

const LogLevelError = 0
const LogLevelInfo = 1
const LogLevelDebug = 2

type LoggerConfig struct {
	LoggerName           string
	Level                string
	EnableConsoleLog     bool
	EnableSyslog         bool
	SyslogHost           string
	SyslogPort           int
	SyslogProtocol       string // "udp" or "tcp"
	OutputFolder         string
	RotationBySize       bool
	MaxFileSizeMB        int // in bytes
	MaxLogFiles          int // maximum number of log files to keep
	RotationIntervalHour int // rotate log every N hours if LogRotationBySize is false
}

type Logger struct {
	logger    *log.Logger
	prefix    string
	syslogger *srslog.Writer
	Destroy   func()
	loglevel  int
}

// --------------------

func getCallerFuncName() string {
	pc, _, _, ok := runtime.Caller(2) // Caller(2) to get the caller of the function that called getCallerFuncName
	if !ok {
		return "unknown"
	}
	details := runtime.FuncForPC(pc)
	if details == nil {
		return "unknown"
	}
	name := details.Name()
	// Extract just the function name (without package path)
	lastSlash := strings.LastIndex(name, "/")
	if lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	dotIndex := strings.LastIndex(name, ".")
	if dotIndex >= 0 {
		name = name[dotIndex+1:]
	}
	return name
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.loglevel >= LogLevelDebug {
		caller := getCallerFuncName() + ": "
		if l.syslogger != nil {
			l.syslogger.Debug(fmt.Sprintf(l.prefix+caller+"debug: "+format, v...))
		}
		if l.logger != nil {
			l.logger.Printf(l.prefix+caller+"debug: "+format, v...)
		}
	}
}

func (l *Logger) Infof(format string, v ...interface{}) {
	if l.loglevel >= LogLevelInfo {
		caller := ""
		if l.loglevel >= LogLevelDebug {
			caller = getCallerFuncName() + ": "
		}
		if l.syslogger != nil {
			l.syslogger.Info(fmt.Sprintf(l.prefix+caller+"info: "+format, v...))
		}
		if l.logger != nil {
			l.logger.Printf(l.prefix+caller+"info: "+format, v...)
		}
	}
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	caller := ""
	if l.loglevel >= LogLevelDebug {
		caller = getCallerFuncName() + ": "
	}
	if l.syslogger != nil {
		l.syslogger.Err(fmt.Sprintf(l.prefix+caller+"error: "+format, v...))
	}
	if l.logger != nil {
		l.logger.Printf(l.prefix+caller+"error: "+format, v...)
	}
}

// ---------------------

// Validate checks if the LoggerConfig is valid and returns an error if it's not
func Validate(c LoggerConfig) error {
	// Check required fields
	if c.LoggerName == "" {
		return fmt.Errorf("logger name cannot be empty")
	}

	// Validate log level
	logLevel := strings.ToLower(c.Level)
	if logLevel != "error" && logLevel != "info" && logLevel != "debug" {
		return fmt.Errorf("invalid log level: %s (must be error, info, or debug)", c.Level)
	}

	// Validate syslog configuration if enabled
	if c.EnableSyslog {
		if c.SyslogHost == "" {
			return fmt.Errorf("syslog host cannot be empty when syslog is enabled")
		}
		if c.SyslogPort <= 0 || c.SyslogPort > 65535 {
			return fmt.Errorf("invalid syslog port: %d (must be between 1 and 65535)", c.SyslogPort)
		}
		protocol := strings.ToLower(c.SyslogProtocol)
		if protocol != "udp" && protocol != "tcp" {
			return fmt.Errorf("invalid syslog protocol: %s (must be udp or tcp)", c.SyslogProtocol)
		}
	}

	// Validate log rotation settings
	if c.RotationBySize {
		if c.MaxFileSizeMB <= 0 {
			return fmt.Errorf("log max file size must be greater than 0 when size-based rotation is enabled")
		}
	} else {
		if c.RotationIntervalHour <= 0 {
			return fmt.Errorf("log rotation interval must be greater than 0 when time-based rotation is enabled")
		}
	}

	if c.MaxLogFiles <= 0 {
		return fmt.Errorf("maximum number of log files must be greater than 0")
	}

	return nil
}

func InitLoggerFactoryByObj(loggerCfg LoggerConfig) func(logname string) *Logger {
	if err := Validate(loggerCfg); err != nil {
		log.Fatalf("invalid logger configuration: %v", err)
	}

	return initLoggerFactory(
		loggerCfg.LoggerName,
		loggerCfg.Level,
		loggerCfg.OutputFolder,
		loggerCfg.RotationBySize,
		loggerCfg.MaxFileSizeMB,
		loggerCfg.MaxLogFiles,
		loggerCfg.RotationIntervalHour,
		loggerCfg.EnableConsoleLog,
		loggerCfg.EnableSyslog,
		loggerCfg.SyslogHost,
		loggerCfg.SyslogPort,
		loggerCfg.SyslogProtocol)
}

func initLoggerFactory(
	loggerName string,
	logLevel string,
	logOutputFolder string,
	logRotationBySize bool,
	logMaxFileSize int,
	logMaxFiles int,
	logRotationIntervalHour int,
	enableConsoleLog bool,
	enableSyslog bool,
	syslogHost string,
	syslogPort int,
	syslogProtocol string) func(logname string) *Logger {

	var myLogger Logger
	loglevellc := strings.ToLower(logLevel)
	switch loglevellc {
	case "debug":
		myLogger.loglevel = LogLevelDebug
	case "info":
		myLogger.loglevel = LogLevelInfo
	case "error":
		myLogger.loglevel = LogLevelError
	default:
		myLogger.loglevel = LogLevelInfo
	}

	// Initialize the logger
	var locallogger_destroy func()
	var syslogger_destroy func()

	// Rotate every hour if logRotationBySize is false
	logRotationInterval := time.Duration(logRotationIntervalHour) * time.Hour
	myLogger.logger, locallogger_destroy = createCustomFileLogger(
		loggerName, logOutputFolder, logRotationBySize, logMaxFileSize,
		logMaxFiles, logRotationInterval, enableConsoleLog)

	if enableSyslog {
		myLogger.syslogger, syslogger_destroy = createSysLogger(
			loggerName,
			syslogHost,
			syslogPort,
			syslogProtocol)
	}

	myLogger.Destroy = func() {
		if locallogger_destroy != nil {
			locallogger_destroy()
		}
		if syslogger_destroy != nil {
			syslogger_destroy()
		}
	}

	return func(logPrefix string) *Logger {
		thisLogger := myLogger
		thisLogger.prefix = ""
		if logPrefix != "" {
			thisLogger.prefix = logPrefix + ": "
		}
		return &thisLogger
	}
}

func createSysLogger(loggerName string, host string, port int, protocol string) (*srslog.Writer, func()) {
	// Initialize syslog writer
	if protocol != "udp" && protocol != "tcp" {
		fmt.Printf("invalid syslog protocol: %s. Use 'udp' or 'tcp'.", protocol)
		return nil, nil
	}
	connStr := fmt.Sprintf("%s:%d", host, port)
	syslogWriter, err := srslog.Dial(protocol, connStr, srslog.LOG_INFO, loggerName)
	if err != nil {
		fmt.Printf("failed to connect to syslog: %v", err)
		return nil, nil
	}

	destroyFunc := func() {
		if syslogWriter != nil {
			syslogWriter.Close() // Properly close the syslog writer
		}
	}

	// Return a logger that writes to syslog
	return syslogWriter, destroyFunc
}

// createCustomFileLogger initializes a custom file logger with rotation support
func createCustomFileLogger(
	loggerName,
	logDir string,
	rotateBySize bool,
	maxSize int,
	maxLogFiles int,
	rotationInterval time.Duration,
	EnableConsoleLog bool) (*log.Logger, func()) {

	// Ensure the log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("failed to create log directory: %v", err)
	}

	// Open the log file
	fileName := filepath.Join(logDir, loggerName+".log")
	logFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}

	// Combine log file and console output
	var multiWriter io.Writer
	if EnableConsoleLog {
		multiWriter = io.MultiWriter(logFile, os.Stdout)
	} else {
		multiWriter = io.MultiWriter(logFile)
	}

	// Start a goroutine for log rotation
	var quitRotation = false
	go func() {

		cleanupLogFiles(logDir, maxLogFiles)
		// get the current hour
		lastHour := time.Now().Hour()
		for {
			if quitRotation {
				return
			}
			if rotateBySize {
				// Rotate by size
				fileInfo, err := logFile.Stat()
				if err == nil && fileInfo.Size() >= int64(maxSize) {
					rotateLogFile(logFile, fileName)
					cleanupLogFiles(logDir, maxLogFiles) // Clean up old log files
				}
				time.Sleep(10 * time.Second) // Check periodically
			} else {
				// Rotate by time
				currentHour := time.Now().Hour()
				if currentHour != lastHour {
					if (currentHour % int(rotationInterval.Hours())) == 0 {
						fmt.Println("rotating log file by time")
						rotateLogFile(logFile, fileName)
						cleanupLogFiles(logDir, maxLogFiles) // Clean up old log files
					}
					lastHour = currentHour
				}
				time.Sleep(1 * time.Second) // Check periodically
			}
		}
	}()

	destroyFunc := func() {
		quitRotation = true
		if logFile != nil {
			logFile.Close()
		}
	}
	// Create and return the logger
	logger := log.New(multiWriter, loggerName+": ", log.LstdFlags|log.Lshortfile)
	currentFlags := logger.Flags()                               // Get the current flags
	newFlags := currentFlags &^ (log.Lshortfile | log.Llongfile) // Remove the Lshortfile and Llongfile flags
	logger.SetFlags(newFlags)                                    // Set the modified flags
	return logger, destroyFunc
}

func cleanupLogFiles(logOutputFolder string, maxLogFiles int) {
	files, err := os.ReadDir(logOutputFolder)
	if err != nil {
		fmt.Printf("failed to read log output folder: %v\n", err)
		return
	}

	var logFiles []string
	for _, file := range files {
		if file.Name() != "kodo.log" && !file.IsDir() {
			logFiles = append(logFiles, file.Name())
		}
	}

	// Sort log files by filename
	sort.Strings(logFiles)

	// Keep only the newest maxLogFiles copies
	if len(logFiles) > maxLogFiles {
		filesToDelete := logFiles[:len(logFiles)-maxLogFiles]
		for _, file := range filesToDelete {
			filePath := filepath.Join(logOutputFolder, file)
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("failed to delete old log file %s: %v", filePath, err)
			}
		}
	}
}

// rotateLogFile handles log file rotation
func rotateLogFile(logFile *os.File, fileName string) {
	// Close the current log file
	err := logFile.Close()
	if err != nil {
		fmt.Printf("log rotation: failed to closed current log file: %v\n", err)
		return
	}

	// Rename the current log file with a timestamp
	timestamp := time.Now().Format("20060102-15")
	rotatedFileName := fmt.Sprintf("%s.%s", fileName, timestamp)
	if err := os.Rename(fileName, rotatedFileName); err != nil {
		fmt.Printf("log rotation: failed to rotate log file: %v", err)
	}

	// Open a new log file
	newLogFile, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("log rotation: failed to open new log file: %v", err)
	}

	// Update the log file reference
	*logFile = *newLogFile
}
