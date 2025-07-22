package handlers

import (
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/IlyaMakar/finance_bot/internal/repository"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CallbackRenameCategory      = "rename_cat_"
	CallbackDeleteCategory      = "delete_cat_"
	CallbackEditCategory        = "edit_cat_"
	CallbackToggleNotifications = "toggle_notifs_"
	CallbackEditSaving          = "edit_saving_"
	CallbackDeleteSaving        = "delete_saving_"
	CallbackRenameSaving        = "rename_saving_"
	CallbackClearSaving         = "clear_saving_"
	CallbackManageSavings       = "manage_savings"
)

type Bot struct {
	bot  *tgbotapi.BotAPI
	repo *repository.SQLiteRepository
}

type UserState struct {
	Step             string
	TempCategoryID   int
	TempAmount       float64
	TempCategoryName string
	TempComment      string
	TempType         string
}

var userStates = make(map[int64]UserState)

func NewBot(token string, repo *repository.SQLiteRepository) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{bot: botAPI, repo: repo}, nil
}

func (b *Bot) GetRepo() *repository.SQLiteRepository {
	return b.repo
}

func (b *Bot) send(chatID int64, c tgbotapi.Chattable) {
	_, err := b.bot.Send(c)
	if err != nil {
		log.Printf("Ошибка отправки в чат %d: %v", chatID, err)
	}
}

func (b *Bot) SendMessage(msg tgbotapi.MessageConfig) {
	_, err := b.bot.Send(msg)
	if err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}

func (b *Bot) sendError(chatID int64, err error) {
	b.send(chatID, tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ Ошибка: %s", err.Error())))
}

func (b *Bot) sendMainMenu(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	menu := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Добавить операцию"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Статистика"),
			tgbotapi.NewKeyboardButton("💵 Накопления"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⚙️ Настройки"),
		),
	)
	msg.ReplyMarkup = menu
	msg.ParseMode = "HTML"
	b.send(chatID, msg)
}

func (b *Bot) renderProgressBar(percent float64, width int) string {
	displayPercent := math.Min(percent, 100)
	filled := int(math.Round(displayPercent / 100 * float64(width)))
	remaining := width - filled

	excess := ""
	if percent > 100 {
		excessCount := int(math.Round((percent - 100) / 100 * float64(width)))
		excess = strings.Repeat("🔴", excessCount)
		remaining -= excessCount
	}

	progressBar := strings.Repeat("🟩", filled) +
		strings.Repeat("⬜", remaining)

	if excess != "" {
		progressBar += " " + excess
	}

	return fmt.Sprintf("%s %.1f%%", progressBar, percent)
}

func (b *Bot) Start() {
	log.Printf("🤖 Бот %s успешно запущен!", b.bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	for upd := range b.bot.GetUpdatesChan(u) {
		if upd.Message != nil {
			b.handleMessage(upd.Message)
		} else if upd.CallbackQuery != nil {
			b.handleCallback(upd.CallbackQuery)
		}
	}
}
