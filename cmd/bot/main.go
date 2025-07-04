package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/IlyaMakar/finance_bot/internal/bot"
	"github.com/IlyaMakar/finance_bot/internal/repository"
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Завершение работы...")
}
