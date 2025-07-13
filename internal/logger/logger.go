package logger

import (
	"io"
	"log"
	"os"
	"strconv"
	"sync"
)

var (
	once        sync.Once
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func Init() {
	once.Do(func() {
		if err := os.MkdirAll("logs", 0755); err != nil {
			log.Fatalf("Не удалось создать папку logs: %v", err)
		}

		file, err := os.OpenFile("logs/bot.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Не удалось открыть файл логов: %v", err)
		}

		multiWriter := io.MultiWriter(os.Stdout, file)
		infoLogger = log.New(multiWriter, "INFO ", log.Ldate|log.Ltime|log.Lshortfile)
		errorLogger = log.New(multiWriter, "ERROR ", log.Ldate|log.Ltime|log.Lshortfile)
	})
}

func LogCommand(username string, command string) {
	if username == "" {
		username = "unknown"
	}
	infoLogger.Printf("CMD @%s: %s", username, command)
}

func LogCommandByID(userID int64, command string) {
	infoLogger.Printf("CMD user_%d: %s", userID, command)
}

func LogButtonClick(username string, buttonName string) {
	if username == "" {
		username = "unknown"
	}
	infoLogger.Printf("BTN @%s: %s", username, buttonName)
}

func LogButtonClickByID(userID int64, buttonName string) {
	infoLogger.Printf("BTN user_%d: %s", userID, buttonName)
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
	errorLogger.Printf("ERR %s: %s", userStr, errorMsg)
}
