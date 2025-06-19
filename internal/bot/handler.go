package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	bot      *tgbotapi.BotAPI
	services *service.FinanceService
}

type UserState struct {
	Step           string
	TempCategoryID int
	TempAmount     float64
	TempComment    string
	TempType       string
}

var userStates = make(map[int64]UserState)

func NewBot(token string, services *service.FinanceService) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	initBasicCategories(services)

	return &Bot{
		bot:      botAPI,
		services: services,
	}, nil
}

func initBasicCategories(s *service.FinanceService) {
	basicCategories := []struct {
		name string
		typ  string
	}{
		{"🍎 Продукты", "expense"},
		{"🚗 Транспорт", "expense"},
		{"🏠 ЖКХ", "expense"},
		{"💼 Зарплата", "income"},
		{"🎢 Развлечения", "expense"},
	}

	// Получаем существующие категории
	existingCategories, err := s.GetCategories()
	if err != nil {
		log.Printf("Ошибка при получении категорий: %v", err)
		return
	}

	// Создаем мапу для быстрой проверки существующих категорий
	existingMap := make(map[string]bool)
	for _, cat := range existingCategories {
		existingMap[cat.Name] = true
	}

	// Создаем только отсутствующие категории
	for _, cat := range basicCategories {
		if !existingMap[cat.name] {
			_, err := s.CreateCategory(cat.name, cat.typ, nil)
			if err != nil {
				log.Printf("Ошибка при создании категории %s: %v", cat.name, err)
			}
		}
	}
}

func (b *Bot) Start() {
	log.Printf("Authorized on account %s", b.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			b.handleCallback(update.CallbackQuery)
		}
	}
}

func (b *Bot) sendMainMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "📊 Главное меню")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Добавить операцию"),
			tgbotapi.NewKeyboardButton("📈 Статистика"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("💵 Накопления"),
			tgbotapi.NewKeyboardButton("⚙️ Настройки"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	switch msg.Text {
	case "/start":
		b.sendWelcomeMessage(msg.Chat.ID)
	case "➕ Добавить операцию":
		b.startAddTransaction(msg.Chat.ID)
	case "📈 Статистика":
		b.showReportMenu(msg.Chat.ID)
	case "💵 Накопления":
		b.showSavingsMenu(msg.Chat.ID)
	case "⚙️ Настройки":
		b.showSettingsMenu(msg.Chat.ID)
	case "Пропустить":
		b.handleCommentInput(msg)
	default:
		b.handleUserInput(msg)
	}
}

func (b *Bot) sendWelcomeMessage(chatID int64) {
	text := `💼 Финансовый помощник 💰

Я помогу вам вести учет доходов и расходов.

Выберите действие:`

	msg := tgbotapi.NewMessage(chatID, text)
	b.send(chatID, msg)
	b.sendMainMenu(chatID)
}

func (b *Bot) startAddTransaction(chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 Доход", "type_income"),
			tgbotapi.NewInlineKeyboardButtonData("💸 Расход", "type_expense"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Выберите тип операции:")
	msg.ReplyMarkup = keyboard
	b.send(chatID, msg)
}

func (b *Bot) handleCallback(query *tgbotapi.CallbackQuery) {
	chatID := query.From.ID
	data := query.Data

	switch {
	case data == "type_income" || data == "type_expense":
		b.handleTypeSelection(chatID, query.Message.MessageID, data)
	case strings.HasPrefix(data, "cat_"):
		categoryID, _ := strconv.Atoi(data[4:])
		b.handleCategorySelection(chatID, categoryID)
	case data == "other_cat":
		b.requestNewCategory(chatID)
	default:
		b.bot.Send(tgbotapi.NewCallback(query.ID, ""))
	}
}

func (b *Bot) handleTypeSelection(chatID int64, messageID int, operationType string) {
	userStates[chatID] = UserState{
		Step:     "select_category",
		TempType: operationType[5:], // "type_income" -> "income"
	}

	categories, err := b.services.GetCategories()
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, cat := range categories {
		if cat.Type == operationType[5:] {
			btn := tgbotapi.NewInlineKeyboardButtonData(cat.Name, "cat_"+strconv.Itoa(cat.ID))
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(btn))
		}
	}

	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✏️ Другая категория", "other_cat"),
	))

	msg := tgbotapi.NewEditMessageTextAndMarkup(
		chatID,
		messageID,
		"Выберите категорию:",
		tgbotapi.NewInlineKeyboardMarkup(buttons...),
	)
	b.bot.Send(msg)
}

func (b *Bot) handleCategorySelection(chatID int64, categoryID int) {
	state := userStates[chatID]
	state.Step = "enter_amount"
	state.TempCategoryID = categoryID
	userStates[chatID] = state

	msg := tgbotapi.NewMessage(chatID, "Введите сумму:")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.send(chatID, msg)
}

func (b *Bot) requestNewCategory(chatID int64) {
	state := userStates[chatID]
	state.Step = "enter_new_category"
	userStates[chatID] = state

	msg := tgbotapi.NewMessage(chatID, "Введите название новой категории:")
	b.send(chatID, msg)
}

func (b *Bot) handleUserInput(msg *tgbotapi.Message) {
	state, exists := userStates[msg.From.ID]
	if !exists {
		b.sendMainMenu(msg.Chat.ID)
		return
	}

	switch state.Step {
	case "enter_amount":
		b.handleAmountInput(msg)
	case "enter_comment":
		b.handleCommentInput(msg)
	case "enter_new_category":
		b.handleNewCategoryInput(msg)
	}
}

func (b *Bot) handleAmountInput(msg *tgbotapi.Message) {
	amount, err := strconv.ParseFloat(msg.Text, 64)
	if err != nil {
		b.sendMessage(msg.Chat.ID, "Неверный формат суммы. Введите число, например: 1500")
		return
	}

	state := userStates[msg.From.ID]
	state.Step = "enter_comment"
	state.TempAmount = amount
	userStates[msg.From.ID] = state

	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Пропустить"),
		),
	)

	msgConfig := tgbotapi.NewMessage(msg.Chat.ID, "Введите комментарий:")
	msgConfig.ReplyMarkup = keyboard
	b.send(msg.Chat.ID, msgConfig)
}

func (b *Bot) handleCommentInput(msg *tgbotapi.Message) {
	state, exists := userStates[msg.From.ID]
	if !exists {
		b.sendMainMenu(msg.Chat.ID)
		return
	}

	if msg.Text != "Пропустить" {
		state.TempComment = msg.Text
	}

	// Получаем категорию
	category, err := b.services.GetCategoryByID(state.TempCategoryID)
	if err != nil {
		b.sendError(msg.Chat.ID, fmt.Errorf("не удалось получить категорию: %v", err))
		return
	}

	// Проверяем тип операции
	var amount float64
	if category.Type == "expense" {
		amount = -state.TempAmount
	} else {
		amount = state.TempAmount
	}

	// Добавляем транзакцию
	_, err = b.services.AddTransaction(
		amount,
		state.TempCategoryID,
		"card",
		state.TempComment,
	)
	if err != nil {
		b.sendError(msg.Chat.ID, err)
		return
	}

	// Формируем сообщение подтверждения
	operationType := "Доход"
	if amount < 0 {
		operationType = "Расход"
		amount = -amount
	}

	confirmMsg := fmt.Sprintf(
		"✅ %s добавлен:\n\n"+
			"📌 Категория: %s\n"+
			"💰 Сумма: %.2f руб.\n"+
			"📝 Комментарий: %s",
		operationType,
		category.Name,
		amount,
		state.TempComment,
	)

	b.sendMessage(msg.Chat.ID, confirmMsg)
	b.sendMainMenu(msg.Chat.ID)
	delete(userStates, msg.From.ID)
}

func (b *Bot) handleNewCategoryInput(msg *tgbotapi.Message) {
	state, exists := userStates[msg.From.ID]
	if !exists {
		b.sendMainMenu(msg.Chat.ID)
		return
	}

	_, err := b.services.CreateCategory(msg.Text, state.TempType, nil)
	if err != nil {
		b.sendError(msg.Chat.ID, err)
		return
	}

	b.sendMessage(msg.Chat.ID, fmt.Sprintf("✅ Категория '%s' создана!", msg.Text))
	b.startAddTransaction(msg.Chat.ID)
	delete(userStates, msg.From.ID)
}

func (b *Bot) showReportMenu(chatID int64) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, -1)

	transactions, err := b.services.GetTransactionsForPeriod(start, end)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	var income, expense float64
	for _, t := range transactions {
		// Получаем категорию для проверки типа
		category, err := b.services.GetCategoryByID(t.CategoryID)
		if err != nil {
			log.Printf("Error getting category: %v", err)
			continue
		}

		if category.Type == "income" {
			income += t.Amount
		} else if category.Type == "expense" {
			expense += t.Amount
		}
	}

	msg := fmt.Sprintf("📊 Статистика за %s:\n\n"+
		"Доходы: %.2f руб.\n"+
		"Расходы: %.2f руб.\n"+
		"Баланс: %.2f руб.",
		now.Month().String(),
		income,
		expense,
		income-expense)

	b.sendMessage(chatID, msg)
}

func (b *Bot) showSavingsMenu(chatID int64) {
	// Заглушка для демонстрации
	msg := tgbotapi.NewMessage(chatID, "💵 Ваши накопления:\n\nОбщая сумма: 100 000 руб.\nЦель: 500 000 руб.")
	b.send(chatID, msg)
}

func (b *Bot) showSettingsMenu(chatID int64) {
	// Заглушка для демонстрации
	msg := tgbotapi.NewMessage(chatID, "⚙️ Настройки:\n\n1. Уведомления: включены\n2. Валюта: рубли")
	b.send(chatID, msg)
}

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	b.send(chatID, msg)
}

func (b *Bot) sendError(chatID int64, err error) {
	log.Printf("Error: %v", err)
	b.sendMessage(chatID, "⚠️ Произошла ошибка: "+err.Error())
}

func (b *Bot) send(chatID int64, msg tgbotapi.Chattable) {
	if _, err := b.bot.Send(msg); err != nil {
		log.Printf("Failed to send message to %d: %v", chatID, err)
	}
}
