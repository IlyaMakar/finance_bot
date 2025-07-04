package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/bot"
	"github.com/IlyaMakar/finance_bot/internal/repository"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env файл не найден или не удалось загрузить")
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatalf("TELEGRAM_TOKEN не задан")
	}

	db, err := repository.NewSQLiteDB("finance.db")
	if err != nil {
		log.Fatalf("не удалось подключиться к БД: %v", err)
	}
	defer db.Close()

	if err := repository.InitDB(db); err != nil {
		log.Fatalf("не удалось инициализировать БД: %v", err)
	}

	repo := repository.NewRepository(db)

	botInstance, err := bot.NewBot(token, repo)
	if err != nil {
		log.Fatalf("не удалось создать бота: %v", err)
	}

	go botInstance.Start()
	go startReminder(botInstance, repo, false) //true запуск тестового сообщения

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Завершение работы...")
}

func startReminder(botInstance *bot.Bot, repo *repository.SQLiteRepository, testMode bool) {
	checkInterval := time.Minute
	reminderHour := -1

	if !testMode {
		checkInterval = time.Hour
		reminderHour = 20
	}

	// Первое напоминание через 10 секунд после старта
	time.Sleep(10 * time.Second)
	sendTestReminder(botInstance, repo, testMode)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for now := range ticker.C {
		if reminderHour >= 0 && now.Hour() != reminderHour {
			continue
		}

		log.Println("Проверяем напоминания...")
		users, err := repo.GetAllUsers()
		if err != nil {
			log.Println("Reminder error getting users:", err)
			continue
		}

		for _, user := range users {
			// Проверяем настройки уведомлений
			enabled, err := repo.GetUserNotificationsEnabled(user.ID)
			if err != nil || !enabled {
				continue
			}

			hasTransactions, err := repo.HasTransactionsToday(user.ID)
			if err != nil {
				log.Printf("Reminder error for user %d: %v", user.ID, err)
				continue
			}

			if !hasTransactions {
				log.Printf("Отправляю напоминание пользователю %d", user.TelegramID)
				sendReminderMessage(botInstance, user.TelegramID, testMode)
			}
		}
	}
}

func sendTestReminder(botInstance *bot.Bot, repo *repository.SQLiteRepository, testMode bool) {
	if !testMode {
		return
	}

	users, err := repo.GetAllUsers()
	if err != nil {
		log.Println("Test reminder error getting users:", err)
		return
	}

	for _, user := range users {
		log.Printf("Отправляю ТЕСТОВОЕ напоминание пользователю %d", user.TelegramID)
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
