package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	logLevel := getEnv("LOG_LEVEL", "INFO")
	logToFile := getEnv("LOG_TO_FILE", "false") == "true"

	if err := logger.Init(logLevel, logToFile); err != nil {
		log.Printf("Failed to initialize logger: %v", err)
		log.Println("Using basic logging")
	}
	defer logger.Close()

	defer func() {
		if r := recover(); r != nil {
			logger.Error("PANIC recovered", "error", r)
		}
	}()

	logger.Info("Starting Finance Bot")

	err := godotenv.Load()
	if err != nil {
		logger.Warn(".env file not found or could not be loaded")
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		logger.Fatal("TELEGRAM_TOKEN not set")
	}

	isTestMode := os.Getenv("TEST_MODE") == "true"
	dbPath := "finance.db"
	if isTestMode {
		dbPath = "finance_test.db"
		logger.Info("Running in test mode", "db_path", dbPath)
	}

	logger.Info("Connecting to database", "path", dbPath)
	db, err := repository.NewSQLiteDB(dbPath)
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Error closing database", "error", err)
		}
	}()

	logger.Info("Initializing database")
	if err := repository.InitDB(db); err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}

	repo := repository.NewRepository(db)

	logger.Info("Creating bot instance")
	botInstance, err := handlers.NewBot(token, repo)
	if err != nil {
		logger.Fatal("Failed to create bot", "error", err)
	}

	botInstance.CheckForUpdates()
	botInstance.NotifyUsersAboutUpdate()

	loc, err := time.LoadLocation("Asia/Yekaterinburg")
	if err != nil {
		logger.Fatal("Failed to load timezone", "error", err)
	}

	printSimpleStats(db, loc)

	go botInstance.Start()
	go startAdminAPI(botInstance, repo)
	go startReminder(botInstance, repo, isTestMode)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Bot successfully started and waiting for commands")
	logger.Info("Press Ctrl+C to stop")

	<-quit
	logger.Info("Received shutdown signal, stopping bot...")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func startAdminAPI(botInstance *handlers.Bot, repo *repository.SQLiteRepository) {
	statsAPI := handlers.NewStatsAPI(repo)

	http.HandleFunc("/api/stats", statsAPI.GetStats)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	port := 8080
	logger.Info("📊 Starting admin API server", "port", port)
	if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
		logger.Error("❌ Failed to start admin API", "error", err)
	}
}

func printSimpleStats(db *sql.DB, loc *time.Location) {
	now := time.Now().In(loc)

	var totalUsers int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil {
		logger.Error("Error counting users", "error", err)
		totalUsers = 0
	}

	var activeUsers int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM user_activity WHERE last_active >= ?
	`, now.Add(-24*time.Hour).Format(time.RFC3339)).Scan(&activeUsers)
	if err != nil {
		logger.Error("Error counting active users", "error", err)
		activeUsers = 0
	}

	logger.Info("Startup statistics",
		"date", now.Format("02.01.2006 15:04"),
		"total_users", totalUsers,
		"active_24h", activeUsers)
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
		logger.Fatal("Failed to load Moscow timezone", "error", err)
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

		logger.Debug("Checking reminders")
		users, err := repo.GetAllUsers()
		if err != nil {
			logger.Error("Reminder error getting users", "error", err)
			continue
		}

		remindersSent := 0
		for _, user := range users {
			enabled, err := repo.GetUserNotificationsEnabled(user.ID)
			if err != nil {
				logger.Error("Notification check error", "user_id", user.TelegramID, "error", err)
				continue
			}

			if !enabled {
				continue
			}

			hasTransactions, err := repo.HasTransactionsToday(user.ID)
			if err != nil {
				logger.Error("Transaction check error", "user_id", user.TelegramID, "error", err)
				continue
			}

			if !hasTransactions {
				logger.Info("Sending reminder", "user_id", user.TelegramID)
				sendReminderMessage(botInstance, user.TelegramID, testMode)
				remindersSent++
			}
		}
		logger.Info("Reminders completed", "sent", remindersSent, "total_users", len(users))
	}
}

func sendTestReminder(botInstance *handlers.Bot, repo *repository.SQLiteRepository, testMode bool) {
	if !testMode {
		return
	}

	logger.Info("Sending test reminders")
	users, err := repo.GetAllUsers()
	if err != nil {
		logger.Error("Test reminder error getting users", "error", err)
		return
	}

	for _, user := range users {
		logger.Debug("Sending test reminder", "user_id", user.TelegramID)
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
