package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RackSec/srslog"
)

const LogLevelError = 0
const LogLevelInfo = 1
const LogLevelDebug = 2

type Logger struct {
	logger    *log.Logger
	prefix    string
	syslogger *srslog.Writer
	Destroy   func()
	loglevel  int
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.loglevel >= LogLevelDebug {
		if l.syslogger != nil {
			l.syslogger.Debug(fmt.Sprintf(l.prefix+"debug: "+format, v...))
		}
		if l.logger != nil {
			l.logger.Printf(l.prefix+"debug: "+format, v...)
		}
	}
}

func (l *Logger) Infof(format string, v ...interface{}) {
	if l.loglevel >= LogLevelInfo {
		if l.syslogger != nil {
			l.syslogger.Info(fmt.Sprintf(l.prefix+"info: "+format, v...))
		}
		if l.logger != nil {
			l.logger.Printf(l.prefix+"info: "+format, v...)
		}
	}
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.syslogger != nil {
		l.syslogger.Err(fmt.Sprintf(l.prefix+"error: "+format, v...))
	}
	if l.logger != nil {
		l.logger.Printf(l.prefix+"error: "+format, v...)
	}
}

// func initLogger(mconfig *config.MasterConfig) *Logger {
func InitLoggerFactory(
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
	fileName := filepath.Join(logDir, "kodo.log")
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
	return log.New(multiWriter, loggerName+": ", log.LstdFlags|log.Lshortfile), destroyFunc
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
