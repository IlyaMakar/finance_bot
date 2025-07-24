package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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

	printInitialStats(db, loc)

	go botInstance.Start()
	go startReminder(botInstance, repo, isTestMode)
	go startBackgroundStats(botInstance.GetRepo(), loc)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	logger.LogCommandByID(0, "Бот успешно запущен. Ожидание команд...")
	<-quit
	logger.LogCommandByID(0, "Получен сигнал завершения. Остановка бота...")
	log.Println("Завершение работы...")
}

func printInitialStats(db *sql.DB, loc *time.Location) {
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	lastMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, loc)
	lastMonthEnd := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	var newUsersToday, totalUsers, activeUsers, lastMonthActive int
	var todayButtonClicks, allTimeButtonClicks, lastMonthClicks map[string]int
	var totalTodayClicks, totalAllTimeClicks int

	err := db.QueryRow(`
		SELECT COUNT(*) FROM users WHERE created_at >= ? AND created_at < ?
	`, startOfDay.Format(time.RFC3339), startOfDay.AddDate(0, 0, 1).Format(time.RFC3339)).Scan(&newUsersToday)
	if err != nil {
		log.Printf("Ошибка при подсчете новых пользователей: %v", err)
		newUsersToday = 0
	}

	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil {
		log.Printf("Ошибка при подсчете общего числа пользователей: %v", err)
		totalUsers = 0
	}

	err = db.QueryRow(`
		SELECT COUNT(*) FROM user_activity WHERE last_active >= ?
	`, now.Add(-24*time.Hour).Format(time.RFC3339)).Scan(&activeUsers)
	if err != nil {
		log.Printf("Ошибка при подсчете активных пользователей: %v", err)
		activeUsers = 0
	}
	inactiveUsers := totalUsers - activeUsers

	rows, err := db.Query(`
		SELECT button_name, COUNT(*) FROM button_clicks WHERE click_time >= ? GROUP BY button_name
	`, startOfDay.Format(time.RFC3339))
	if err != nil {
		log.Printf("Ошибка при подсчете кликов за сегодня: %v", err)
	} else {
		defer rows.Close()
		todayButtonClicks = make(map[string]int)
		for rows.Next() {
			var buttonName string
			var count int
			if err := rows.Scan(&buttonName, &count); err != nil {
				log.Printf("Ошибка при сканировании кликов: %v", err)
				continue
			}
			todayButtonClicks[buttonName] = count
			totalTodayClicks += count
		}
	}

	rows, err = db.Query(`
		SELECT button_name, COUNT(*) FROM button_clicks GROUP BY button_name
	`)
	if err != nil {
		log.Printf("Ошибка при подсчете кликов за все время: %v", err)
	} else {
		defer rows.Close()
		allTimeButtonClicks = make(map[string]int)
		for rows.Next() {
			var buttonName string
			var count int
			if err := rows.Scan(&buttonName, &count); err != nil {
				log.Printf("Ошибка при сканировании кликов: %v", err)
				continue
			}
			allTimeButtonClicks[buttonName] = count
			totalAllTimeClicks += count
		}
	}

	err = db.QueryRow(`
		SELECT COUNT(*) FROM user_activity WHERE last_active >= ? AND last_active < ?
	`, lastMonthStart.Format(time.RFC3339), lastMonthEnd.Format(time.RFC3339)).Scan(&lastMonthActive)
	if err != nil {
		log.Printf("Ошибка при подсчете активности прошлого месяца: %v", err)
		lastMonthActive = 0
	}
	activityChange := 0.0
	if lastMonthActive > 0 {
		activityChange = (float64(activeUsers) - float64(lastMonthActive)) / float64(lastMonthActive) * 100
	}

	rows, err = db.Query(`
		SELECT button_name, COUNT(*) FROM button_clicks WHERE click_time >= ? AND click_time < ? GROUP BY button_name
	`, lastMonthStart.Format(time.RFC3339), lastMonthEnd.Format(time.RFC3339))
	if err != nil {
		log.Printf("Ошибка при подсчете кликов прошлого месяца: %v", err)
	} else {
		defer rows.Close()
		lastMonthClicks = make(map[string]int)
		for rows.Next() {
			var buttonName string
			var count int
			if err := rows.Scan(&buttonName, &count); err != nil {
				log.Printf("Ошибка при сканировании кликов: %v", err)
				continue
			}
			lastMonthClicks[buttonName] = count
		}
	}

	fmt.Println("=== Статистика бота при старте ===")
	fmt.Printf("Дата и время: %s\n", now.Format("02.01.2006 15:04"))
	fmt.Printf("Новых пользователей за сегодня: %d\n", newUsersToday)
	fmt.Printf("Общее количество пользователей: %d\n", totalUsers)
	fmt.Printf("Активных пользователей (за 24 часа): %d (%.2f%% от прошлого месяца)\n", activeUsers, activityChange)
	fmt.Printf("Неактивных пользователей: %d\n", inactiveUsers)
	fmt.Printf("Общее количество кликов за сегодня: %d\n", totalTodayClicks)
	for button, count := range todayButtonClicks {
		last := lastMonthClicks[button]
		change := 0.0
		if last > 0 {
			change = (float64(count) - float64(last)) / float64(last) * 100
		} else if count > 0 {
			change = 100.0
		}
		fmt.Printf("Клик по %s за сегодня: %d (%.2f%% от прошлого месяца)\n", button, count, change)
	}
	fmt.Printf("Общее количество кликов за все время: %d\n", totalAllTimeClicks)
	for button, count := range allTimeButtonClicks {
		last := lastMonthClicks[button]
		change := 0.0
		if last > 0 {
			change = (float64(count) - float64(last)) / float64(last) * 100
		} else if count > 0 {
			change = 100.0
		}
		fmt.Printf("Клик по %s за все время: %d (%.2f%% от прошлого месяца)\n", button, count, change)
	}

	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err = os.MkdirAll(logDir, 0755)
		if err != nil {
			log.Printf("Ошибка создания папки logs: %v", err)
			return
		}
	}

	logFilePath := filepath.Join(logDir, "stats.log")
	logFile, err := os.OpenFile(logFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Ошибка открытия файла логов: %v", err)
		return
	}
	defer logFile.Close()

	logger := log.New(logFile, "STATS ", log.Ldate|log.Ltime)
	logger.Printf("=== Статистика бота при старте ===\n")
	logger.Printf("Дата и время: %s\n", now.Format("02.01.2006 15:04"))
	logger.Printf("Новых пользователей за сегодня: %d\n", newUsersToday)
	logger.Printf("Общее количество пользователей: %d\n", totalUsers)
	logger.Printf("Активных пользователей (за 24 часа): %d (%.2f%% от прошлого месяца)\n", activeUsers, activityChange)
	logger.Printf("Неактивных пользователей: %d\n", inactiveUsers)
	logger.Printf("Общее количество кликов за сегодня: %d\n", totalTodayClicks)
	for button, count := range todayButtonClicks {
		last := lastMonthClicks[button]
		change := 0.0
		if last > 0 {
			change = (float64(count) - float64(last)) / float64(last) * 100
		} else if count > 0 {
			change = 100.0
		}
		logger.Printf("Клик по %s за сегодня: %d (%.2f%% от прошлого месяца)\n", button, count, change)
	}
	logger.Printf("Общее количество кликов за все время: %d\n", totalAllTimeClicks)
	for button, count := range allTimeButtonClicks {
		last := lastMonthClicks[button]
		change := 0.0
		if last > 0 {
			change = (float64(count) - float64(last)) / float64(last) * 100
		} else if count > 0 {
			change = 100.0
		}
		logger.Printf("Клик по %s за все время: %d (%.2f%% от прошлого месяца)\n", button, count, change)
	}
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

func startBackgroundStats(repo *repository.SQLiteRepository, loc *time.Location) {
	ticker := time.NewTicker(2 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().In(loc)
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		lastMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, loc)
		lastMonthEnd := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

		var activeUsers, lastMonthActive int
		var todayButtonClicks, lastMonthClicks map[string]int
		var totalTodayClicks int

		if err := repo.GetActiveUsersCount(now.Add(-24*time.Hour), &activeUsers); err != nil {
			logger.LogError("system", fmt.Sprintf("Ошибка подсчета активных пользователей: %v", err))
			continue
		}

		if err := repo.GetButtonClicksCount(startOfDay, &todayButtonClicks); err != nil {
			logger.LogError("system", fmt.Sprintf("Ошибка подсчета кликов за сегодня: %v", err))
			continue
		}
		for _, count := range todayButtonClicks {
			totalTodayClicks += count
		}

		if err := repo.GetActiveUsersCountForPeriod(lastMonthStart, lastMonthEnd, &lastMonthActive); err != nil {
			logger.LogError("system", fmt.Sprintf("Ошибка подсчета активности прошлого месяца: %v", err))
			continue
		}
		activityChange := 0.0
		if lastMonthActive > 0 {
			activityChange = (float64(activeUsers) - float64(lastMonthActive)) / float64(lastMonthActive) * 100
		}

		if err := repo.GetButtonClicksCountForPeriod(lastMonthStart, lastMonthEnd, &lastMonthClicks); err != nil {
			logger.LogError("system", fmt.Sprintf("Ошибка подсчета кликов прошлого месяца: %v", err))
			continue
		}

		logDir := "logs"
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				log.Printf("Ошибка создания папки logs: %v", err)
				continue
			}
		}

		logFilePath := filepath.Join(logDir, "stats.log")
		logFile, err := os.OpenFile(logFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Ошибка открытия файла логов: %v", err)
			continue
		}
		logger := log.New(logFile, "STATS ", log.Ldate|log.Ltime)
		logger.Printf("=== Статистика бота на %s ===\n", now.Format("02.01.2006 15:04"))
		logger.Printf("Активных пользователей (за 24 часа): %d (%.2f%% от прошлого месяца)\n", activeUsers, activityChange)
		logger.Printf("Общее количество кликов за сегодня: %d\n", totalTodayClicks)
		for button, count := range todayButtonClicks {
			last := lastMonthClicks[button]
			change := 0.0
			if last > 0 {
				change = (float64(count) - float64(last)) / float64(last) * 100
			} else if count > 0 {
				change = 100.0
			}
			logger.Printf("Клик по %s за сегодня: %d (%.2f%% от прошлого месяца)\n", button, count, change)
		}
		logFile.Close()
	}
}

func sendTestReminder(botInstance *handlers.Bot, repo *repository.SQLiteRepository, testMode bool) {
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
