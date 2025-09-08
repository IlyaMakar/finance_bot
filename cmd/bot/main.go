package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/bot/handlers"
	"github.com/IlyaMakar/finance_bot/internal/logger"
	"github.com/IlyaMakar/finance_bot/internal/repository"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	_ "modernc.org/sqlite"
)

func main() {
	logger.Init()
	logger.LogStartup()

	defer func() {
		if r := recover(); r != nil {
			logger.LogError("system", fmt.Sprintf("PANIC: %v", r))
		}
		logger.LogShutdown()
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

	isTestMode := os.Getenv("TEST_MODE") == "true"
	dbPath := "finance.db"
	if isTestMode {
		dbPath = "finance_test.db"
		logger.LogCommandByID(0, "Запуск в тестовом режиме с базой finance_test.db")
	}

	db, err := repository.NewSQLiteDB(dbPath)
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

	botInstance, err := handlers.NewBot(token, repo)
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Failed to create bot: %v", err))
		log.Fatalf("не удалось создать бота: %v", err)
	}

	botInstance.CheckForUpdates()
	botInstance.NotifyUsersAboutUpdate()

	loc, err := time.LoadLocation("Asia/Yekaterinburg")
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Не удалось загрузить временную зону Asia/Yekaterinburg: %v", err))
		log.Fatalf("Ошибка загрузки временной зоны: %v", err)
	}

	printSimpleStats(db, loc) // Упрощенная статистика

	go botInstance.Start()
	go startReminder(botInstance, repo, isTestMode)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.LogCommandByID(0, "Бот успешно запущен. Ожидание команд...")
	log.Println("Завершение работы...")

	<-quit
	logger.LogCommandByID(0, "Получен сигнал завершения. Остановка бота...")
}

// Упрощенная статистика при запуске
func printSimpleStats(db *sql.DB, loc *time.Location) {
	now := time.Now().In(loc)

	var totalUsers int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Ошибка подсчета пользователей: %v", err))
		totalUsers = 0
	}

	var activeUsers int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM user_activity WHERE last_active >= ?
	`, now.Add(-24*time.Hour).Format(time.RFC3339)).Scan(&activeUsers)
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Ошибка подсчета активных пользователей: %v", err))
		activeUsers = 0
	}

	logger.LogSystem(fmt.Sprintf("📊 Статистика при запуске - Дата: %s", now.Format("02.01.2006 15:04")))
	logger.LogSystem(fmt.Sprintf("👥 Всего пользователей: %d", totalUsers))
	logger.LogSystem(fmt.Sprintf("🎯 Активных за 24ч: %d", activeUsers))
	logger.LogSystem("========================================")
}

func startReminder(botInstance *handlers.Bot, repo *repository.SQLiteRepository, testMode bool) {
	checkInterval := time.Minute
	reminderHour := -1

	if !testMode {
		checkInterval = time.Hour
		reminderHour = 16
	}

	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Fatalf("Не удалось загрузить временную зону Europe/Moscow: %v", err)
	}

	time.Sleep(10 * time.Second)
	sendTestReminder(botInstance, repo, testMode)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for now := range ticker.C {
		localTime := now.In(loc)

		if reminderHour >= 0 && localTime.Hour() != reminderHour {
			continue
		}

		logger.LogReminder("Проверка напоминаний...")
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
				logger.LogReminder(fmt.Sprintf("Отправка напоминания user_%d", user.TelegramID))
				sendReminderMessage(botInstance, user.TelegramID, testMode)
			}
		}
	}
}

func sendTestReminder(botInstance *handlers.Bot, repo *repository.SQLiteRepository, testMode bool) {
	if !testMode {
		return
	}

	logger.LogReminder("Отправка тестовых напоминаний")
	users, err := repo.GetAllUsers()
	if err != nil {
		logger.LogError("system", fmt.Sprintf("Test reminder error getting users: %v", err))
		return
	}

	for _, user := range users {
		logger.LogReminder(fmt.Sprintf("Отправка тестового напоминания user_%d", user.TelegramID))
		msg := tgbotapi.NewMessage(
			user.TelegramID,
			"🔔 <b>Тестовое напоминание</b>\n\n"+
				"Это тестовая проверка системы напоминаний.\n"+
				"Реальное напоминание приходит ежедневно в 16:00, если вы не добавили транзакции.",
		)
		msg.ParseMode = "HTML"
		botInstance.SendMessage(msg)
	}
}

func sendReminderMessage(botInstance *handlers.Bot, chatID int64, testMode bool) {
	message := "💡 <b>Напоминание о транзакциях</b>\n\n" +
		"Привет! Похоже, ты сегодня еще не добавлял(а) ни одной транзакции.\n\n" +
		"Не забывай вести учет своих финансов — это поможет лучше контролировать бюджет!\n\n" +
		"➕ Нажми \"Добавить операцию\" "

	if testMode {
		message = "🔔 <b>ТЕСТ: Напоминание о транзакциях</b>\n\n" +
			"Это тестовое напоминание (в рабочем режиме приходит в 20:00).\n\n" +
			message
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"
	botInstance.SendMessage(msg)
}
