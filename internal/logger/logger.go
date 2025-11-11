package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var (
	logInstance *Logger
	levelNames  = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
		FATAL: "FATAL",
	}
	levelColors = map[LogLevel]string{
		DEBUG: "\033[36m",
		INFO:  "\033[32m",
		WARN:  "\033[33m",
		ERROR: "\033[31m",
		FATAL: "\033[35m",
	}
	resetColor = "\033[0m"
)

type Logger struct {
	*log.Logger
	level    LogLevel
	file     *os.File
	filename string
}

func Init(logLevel string, logToFile bool) error {
	level := getLevelFromString(logLevel)

	var logFile *os.File
	var filename string

	if logToFile {
		if err := os.MkdirAll("logs", 0755); err != nil {
			return fmt.Errorf("failed to create logs directory: %v", err)
		}

		filename = filepath.Join("logs", fmt.Sprintf("bot_%s.log", time.Now().Format("2006-01-02")))

		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %v", err)
		}
		logFile = file
	}

	logInstance = &Logger{
		Logger:   log.New(logFile, "", 0),
		level:    level,
		file:     logFile,
		filename: filename,
	}

	if logToFile {
		logInstance.Logger.SetOutput(os.Stdout)
	}

	Info("Logger initialized", "level", levelNames[level], "file", filename)
	return nil
}

func getLevelFromString(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

func (l *Logger) log(level LogLevel, msg string, fields ...interface{}) {
	if level < l.level {
		return
	}

	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	} else {

		file = filepath.Base(file)
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := levelNames[level]
	levelColor := levelColors[level]

	var fieldsStr string
	if len(fields) > 0 {
		var fieldParts []string
		for i := 0; i < len(fields); i += 2 {
			if i+1 < len(fields) {
				fieldParts = append(fieldParts, fmt.Sprintf("%v=%v", fields[i], fields[i+1]))
			} else {
				fieldParts = append(fieldParts, fmt.Sprintf("%v", fields[i]))
			}
		}
		fieldsStr = " [" + strings.Join(fieldParts, " ") + "]"
	}

	consoleMsg := fmt.Sprintf("%s%-5s%s %s %s:%d %s%s",
		levelColor, levelName, resetColor,
		timestamp,
		file, line,
		msg, fieldsStr)

	fileMsg := fmt.Sprintf("%-5s %s %s:%d %s%s",
		levelName,
		timestamp,
		file, line,
		msg, fieldsStr)

	fmt.Println(consoleMsg)

	if l.file != nil {
		l.Logger.Println(fileMsg)
	}
}

func Debug(msg string, fields ...interface{}) {
	if logInstance != nil {
		logInstance.log(DEBUG, msg, fields...)
	}
}

func Info(msg string, fields ...interface{}) {
	if logInstance != nil {
		logInstance.log(INFO, msg, fields...)
	}
}

func Warn(msg string, fields ...interface{}) {
	if logInstance != nil {
		logInstance.log(WARN, msg, fields...)
	}
}

func Error(msg string, fields ...interface{}) {
	if logInstance != nil {
		logInstance.log(ERROR, msg, fields...)
	}
}

func Fatal(msg string, fields ...interface{}) {
	if logInstance != nil {
		logInstance.log(FATAL, msg, fields...)
		os.Exit(1)
	}
}

func Close() {
	if logInstance != nil && logInstance.file != nil {
		logInstance.file.Close()
	}
}
