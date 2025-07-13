package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/bot"
	"github.com/IlyaMakar/finance_bot/internal/logger"
	"github.com/IlyaMakar/finance_bot/internal/repository"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	logger.Init()
	defer func() {
		if r := recover(); r != nil {
			logger.LogError("system", fmt.Sprintf("PANIC: %v", r))
		}
	}()

	err := godotenv.Load()
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Error loading .env file: %v", err))
		log.Println(".env файл не найден или не удалось загрузить")
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		logger.LogError("system", "TELEGRAM_TOKEN not set")
		log.Fatalf("TELEGRAM_TOKEN не задан")
	}

	db, err := repository.NewSQLiteDB("finance.db")
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Failed to connect to DB: %v", err))
		log.Fatalf("не удалось подключиться к БД: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.LogError("system", fmt.Sprintf("Error closing DB: %v", err))
		}
	}()

	if err := repository.InitDB(db); err != nil {
		logger.LogError("system", fmt.Sprintf("Failed to init DB: %v", err))
		log.Fatalf("не удалось инициализировать БД: %v", err)
	}

	repo := repository.NewRepository(db)

	botInstance, err := bot.NewBot(token, repo)
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Failed to create bot: %v", err))
		log.Fatalf("не удалось создать бота: %v", err)
	}

	go botInstance.Start()
	go startReminder(botInstance, repo, false) // false - обычный режим, true - тестовый

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	logger.LogCommandByID(0, "Бот успешно запущен. Ожидание команд...")
	<-quit
	logger.LogCommandByID(0, "Получен сигнал завершения. Остановка бота...")
	log.Println("Завершение работы...")
}

func startReminder(botInstance *bot.Bot, repo *repository.SQLiteRepository, testMode bool) {
	checkInterval := time.Minute
	reminderHour := -1

	if !testMode {
		checkInterval = time.Hour
		reminderHour = 20
	}

	time.Sleep(10 * time.Second)
	sendTestReminder(botInstance, repo, testMode)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for now := range ticker.C {
		if reminderHour >= 0 && now.Hour() != reminderHour {
			continue
		}

		logger.LogCommandByID(0, "Проверка напоминаний...")
		users, err := repo.GetAllUsers()
		if err != nil {
			logger.LogError("system", fmt.Sprintf("Reminder error getting users: %v", err))
			continue
		}

		for _, user := range users {
			enabled, err := repo.GetUserNotificationsEnabled(user.ID)
			if err != nil {
				logger.LogError(user.TelegramID, fmt.Sprintf("Notification check error: %v", err))
				continue
			}

			if !enabled {
				continue
			}

			hasTransactions, err := repo.HasTransactionsToday(user.ID)
			if err != nil {
				logger.LogError(user.TelegramID, fmt.Sprintf("Transaction check error: %v", err))
				continue
			}

			if !hasTransactions {
				logger.LogCommandByID(user.TelegramID, "Отправка напоминания")
				sendReminderMessage(botInstance, user.TelegramID, testMode)
			}
		}
	}
}

func sendTestReminder(botInstance *bot.Bot, repo *repository.SQLiteRepository, testMode bool) {
	if !testMode {
		return
	}

	logger.LogCommandByID(0, "Отправка тестовых напоминаний")
	users, err := repo.GetAllUsers()
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Test reminder error getting users: %v", err))
		return
	}

	for _, user := range users {
		logger.LogCommandByID(user.TelegramID, "Отправка тестового напоминания")
		msg := tgbotapi.NewMessage(
			user.TelegramID,
			"🔔 <b>Тестовое напоминание</b>\n\n"+
				"Это тестовая проверка системы напоминаний.\n"+
				"Реальное напоминание приходит ежедневно в 20:00, если вы не добавили транзакции.",
		)
		msg.ParseMode = "HTML"
		botInstance.SendMessage(msg)
	}
}

func sendReminderMessage(botInstance *bot.Bot, chatID int64, testMode bool) {
	message := "💡 <b>Напоминание о транзакциях</b>\n\n" +
		"Привет! Похоже, ты сегодня еще не добавлял(а) ни одной транзакции.\n\n" +
		"Не забывай вести учет своих финансов — это поможет лучше контролировать бюджет!\n\n" +
		"➕ Нажми \"Добавить операцию\" или просто напиши сумму с комментарием, например:\n" +
		"<code>150 </code>"

	if testMode {
		message = "🔔 <b>ТЕСТ: Напоминание о транзакциях</b>\n\n" +
			"Это тестовое напоминание (в рабочем режиме приходит в 20:00).\n\n" +
			message
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"
	botInstance.SendMessage(msg)
}
