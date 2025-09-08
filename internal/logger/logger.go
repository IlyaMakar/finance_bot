package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var (
	once        sync.Once
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	currentDate string
	logFile     *os.File
	mu          sync.Mutex
)

func Init() {
	once.Do(func() {
		initLoggers()
		go startDailyRotation()
	})
}

func initLoggers() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
	}

	currentDate = time.Now().Format("2006-01-02")
	logDir := "logs"

	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Не удалось создать папку logs: %v", err)
		return
	}

	logFilePath := filepath.Join(logDir, fmt.Sprintf("bot_%s.log", currentDate))

	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Не удалось открыть файл логов: %v", err)
		return
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	infoLogger = log.New(multiWriter, "INFO  ", log.Ldate|log.Ltime)
	errorLogger = log.New(multiWriter, "ERROR ", log.Ldate|log.Ltime)
	debugLogger = log.New(multiWriter, "DEBUG ", log.Ldate|log.Ltime)

	infoLogger.Printf("🚀 Логгер инициализирован для даты %s", currentDate)
}

func startDailyRotation() {
	for {
		now := time.Now()
		nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, now.Location())
		durationUntilNextDay := nextDay.Sub(now)

		time.Sleep(durationUntilNextDay)
		initLoggers()
	}
}

func LogCommand(username string, command string) {
	if username == "" {
		username = "unknown"
	}
	infoLogger.Printf("💬 CMD @%s: %s", username, command)
}

func LogCommandByID(userID int64, command string) {
	infoLogger.Printf("💬 CMD user_%d: %s", userID, command)
}

func LogButtonClick(username string, buttonName string) {
	if username == "" {
		username = "unknown"
	}
	infoLogger.Printf("🔘 BTN @%s: %s", username, buttonName)
}

func LogButtonClickByID(userID int64, buttonName string) {
	infoLogger.Printf("🔘 BTN user_%d: %s", userID, buttonName)
}

func LogError(userIdentifier interface{}, errorMsg string) {
	var userStr string
	switch v := userIdentifier.(type) {
	case string:
		userStr = v
	case int64:
		userStr = "user_" + strconv.FormatInt(v, 10)
	case int:
		userStr = "user_" + strconv.Itoa(v)
	default:
		userStr = "unknown"
	}
	errorLogger.Printf("❌ ERR %s: %s", userStr, errorMsg)
}

func LogSystem(message string) {
	infoLogger.Printf("⚙️  SYS: %s", message)
}

func LogStartup() {
	infoLogger.Printf("🎉 ===== БОТ ЗАПУЩЕН ===== 🎉")
}

func LogShutdown() {
	infoLogger.Printf("🛑 ===== БОТ ОСТАНОВЛЕН ===== 🛑")
}

func LogDatabase(message string) {
	debugLogger.Printf("🗄️  DB: %s", message)
}

func LogReminder(message string) {
	infoLogger.Printf("🔔 REM: %s", message)
}

func LogTransaction(userID int64, amount float64, category string) {
	infoLogger.Printf("💳 TXN user_%d: %.2f ₽ - %s", userID, amount, category)
}

func LogSaving(userID int64, action string, amount float64, savingName string) {
	infoLogger.Printf("💰 SAV user_%d: %s %.2f ₽ - %s", userID, action, amount, savingName)
}
