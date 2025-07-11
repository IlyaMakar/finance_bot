package logger

import (
	"io"
	"log"
	"os"
	"sync"
)

var (
	once        sync.Once
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func Init() {
	once.Do(func() {
		// Создаем папку logs, если её нет
		if err := os.MkdirAll("logs", 0755); err != nil {
			log.Fatalf("Не удалось создать папку logs: %v", err)
		}

		// Открываем файл логов
		file, err := os.OpenFile("logs/bot.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Не удалось открыть файл логов: %v", err)
		}

		// Настраиваем логгеры для записи и в файл, и в консоль
		multiWriter := io.MultiWriter(os.Stdout, file)

		infoLogger = log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
		errorLogger = log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	})
}

func LogButtonClick(userID int64, buttonName string) {
	infoLogger.Printf("Кнопка: %s, UserID: %d", buttonName, userID)
}

func LogCommand(userID int64, command string) {
	infoLogger.Printf("Команда: %s, UserID: %d", command, userID)
}

func LogError(userID int64, errorMsg string) {
	errorLogger.Printf("Ошибка: %s, UserID: %d", errorMsg, userID)
}
