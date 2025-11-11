package handlers

import (
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/IlyaMakar/finance_bot/internal/logger"

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
	CallbackSetPeriodStart      = "set_period_start"
	CallbackCurrencySettings    = "currency_settings"
	CallbackSetCurrency         = "set_currency_"

	CallbackWriteSupport = "write_support"
	CallbackFAQ          = "faq"

	CallbackFeedback       = "feedback"
	CallbackFeedbackSubmit = "feedback_submit"
	CallbackFeedbackCancel = "feedback_cancel"

	CurrencyRUB = "RUB"
	CurrencyUSD = "USD"
	CurrencyEUR = "EUR"
)

type Bot struct {
	bot       *tgbotapi.BotAPI
	repo      *repository.SQLiteRepository
	reportGen *ReportGenerator
}

type UserState struct {
	Step             string
	TempCategoryID   int
	TempAmount       float64
	TempCategoryName string
	TempComment      string
	TempType         string
	FeedbackStep     string
	FeedbackData     map[string]string
}

var userStates = make(map[int64]UserState)

func NewBot(token string, repo *repository.SQLiteRepository) (*Bot, error) {
	logger.Info("Creating new bot instance")
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logger.Error("Failed to create bot API", "error", err)
		return nil, err
	}

	bot := &Bot{
		bot:  botAPI,
		repo: repo,
	}
	bot.reportGen = NewReportGenerator(bot, repo)

	logger.Info("Bot created successfully", "username", botAPI.Self.UserName)
	return bot, nil
}

func (b *Bot) GetRepo() *repository.SQLiteRepository {
	return b.repo
}

func (b *Bot) send(chatID int64, c tgbotapi.Chattable) {
	_, err := b.bot.Send(c)
	if err != nil {
		logger.Error("Error sending message", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) SendMessage(msg tgbotapi.MessageConfig) {
	_, err := b.bot.Send(msg)
	if err != nil {
		logger.Error("Error sending message", "chat_id", msg.ChatID, "error", err)
	} else {
		logger.Debug("Message sent successfully", "chat_id", msg.ChatID)
	}
}

func (b *Bot) sendError(chatID int64, err error) {
	logger.Warn("Sending error to user", "chat_id", chatID, "error", err)
	b.send(chatID, tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ Ошибка: %s", err.Error())))
}

func (b *Bot) sendMainMenu(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💸 Добавить операцию", "start_transaction"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", "show_stats"),
			tgbotapi.NewInlineKeyboardButtonData("💰 Накопления", "show_savings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Настройки", "show_settings"),
		),
	)
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

func (b *Bot) deleteMessage(chatID int64, messageID int) {
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	b.bot.Send(deleteMsg)
}
