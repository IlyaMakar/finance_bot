package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/IlyaMakar/finance_bot/internal/bot"
	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	db, err := repository.NewSQLiteDB("finance.db")
	if err != nil {
		log.Fatalf("failed to initialize db: %s", err.Error())
	}

	if err := repository.InitDB(db); err != nil {
		log.Fatalf("failed to init db: %s", err.Error())
	}

	repos := repository.NewRepository(db)
	services := service.NewService(repos)

	botInstance, err := bot.NewBot(os.Getenv("TELEGRAM_TOKEN"), services)
	if err != nil {
		log.Fatalf("failed to create bot: %s", err.Error())
	}

	go botInstance.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

}
