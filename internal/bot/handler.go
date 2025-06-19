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

	return &Bot{bot: botAPI, services: services}, nil
}

func initBasicCategories(s *service.FinanceService) {
	basic := []struct{ name, typ string }{
		{"🍎 Продукты", "expense"},
		{"🚗 Транспорт", "expense"},
		{"🏠 ЖКХ", "expense"},
		{"💼 Зарплата", "income"},
		{"🎢 Развлечения", "expense"},
	}
	exists, _ := s.GetCategories()
	wrap := map[string]bool{}
	for _, c := range exists {
		wrap[c.Name] = true
	}
	for _, b := range basic {
		if !wrap[b.name] {
			if _, err := s.CreateCategory(b.name, b.typ, nil); err != nil {
				log.Println("category init:", err)
			}
		}
	}
}

func (b *Bot) Start() {
	log.Printf("Bot %s запущен", b.bot.Self.UserName)
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

func (b *Bot) handleMessage(m *tgbotapi.Message) {
	switch m.Text {
	case "/start":
		b.sendMainMenu(m.Chat.ID, "Привет! Я бот для учёта финансов.")
	case "➕ Добавить операцию":
		b.startAddTransaction(m.Chat.ID)
	case "📈 Статистика":
		b.showReport(m.Chat.ID)
	case "💵 Накопления":
		b.showSavings(m.Chat.ID)
	case "⚙️ Настройки":
		b.sendMainMenu(m.Chat.ID, "⚙️ Настройки")
	case "Пропустить":
		b.handleComment(m)
	default:
		b.handleUserInput(m)
	}
}

func (b *Bot) sendMainMenu(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	menu := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("➕ Добавить операцию"), tgbotapi.NewKeyboardButton("📈 Статистика")),
		tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("💵 Накопления"), tgbotapi.NewKeyboardButton("⚙️ Настройки")),
	)
	msg.ReplyMarkup = menu
	b.send(chatID, msg)
}

func (b *Bot) startAddTransaction(chatID int64) {
	keyb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💵 Доход", "type_income"),
			tgbotapi.NewInlineKeyboardButtonData("💸 Расход", "type_expense"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Выберите тип операции:")
	msg.ReplyMarkup = keyb
	b.send(chatID, msg)
}

func (b *Bot) handleCallback(q *tgbotapi.CallbackQuery) {
	id := q.From.ID
	switch {
	case q.Data == "type_income" || q.Data == "type_expense":
		b.handleTypeSelect(id, q.Message.MessageID, q.Data)
	case strings.HasPrefix(q.Data, "cat_"):
		catID, _ := strconv.Atoi(q.Data[4:])
		b.handleCatSelect(int(id), catID)
	case q.Data == "other_cat":
		state := userStates[id]
		state.Step = "new_cat"
		userStates[id] = state
		b.send(id, tgbotapi.NewMessage(id, "Введите название новой категории:"))
	case q.Data == "create_saving":
		state := userStates[id]
		state.Step = "create_saving_name"
		userStates[id] = state
		b.send(id, tgbotapi.NewMessage(id, "Введите название копилки:"))
	default:
		b.bot.Send(tgbotapi.NewCallback(q.ID, ""))
	}
}

func (b *Bot) handleTypeSelect(chatID int64, msgID int, data string) {
	u := UserState{Step: "select_cat", TempType: data[5:]}
	userStates[chatID] = u

	cats, _ := b.services.GetCategories()
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range cats {
		if c.Type == u.TempType {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(c.Name, "cat_"+strconv.Itoa(c.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✏️ Другая категория", "other_cat"),
	))
	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msgID, "Выберите категорию:", tgbotapi.NewInlineKeyboardMarkup(rows...))
	b.send(chatID, edit)
}

func (b *Bot) handleCatSelect(chatID, catID int) {
	s := userStates[int64(chatID)]
	s.Step = "enter_amount"
	s.TempCategoryID = catID
	userStates[int64(chatID)] = s

	msg := tgbotapi.NewMessage(int64(chatID), "Введите сумму:")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.send(int64(chatID), msg)
}

func (b *Bot) handleUserInput(m *tgbotapi.Message) {
	s, ok := userStates[m.From.ID]
	if !ok {
		b.sendMainMenu(m.Chat.ID, "Выберите действие:")
		return
	}
	switch s.Step {
	case "enter_amount":
		b.handleAmount(m)
	case "enter_comment":
		b.handleComment(m)
	case "new_cat":
		b.handleNewCategory(m)
	case "create_saving_name":
		b.handleCreateSavingName(m)
	case "create_saving_goal":
		b.handleCreateSavingGoal(m)
	}
}
func (b *Bot) handleCreateSavingName(m *tgbotapi.Message) {
	name := strings.TrimSpace(m.Text)
	if name == "" {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "Название не может быть пустым. Введите название копилки:"))
		return
	}

	s := userStates[m.From.ID]
	s.TempComment = name // временно сохраняем имя копилки здесь
	s.Step = "create_saving_goal"
	userStates[m.From.ID] = s

	msg := tgbotapi.NewMessage(m.Chat.ID, "Введите цель копилки (число) или отправьте 'Пропустить':")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Пропустить"),
		),
	)
	b.send(m.Chat.ID, msg)
}

func (b *Bot) handleCreateSavingGoal(m *tgbotapi.Message) {
	s := userStates[m.From.ID]

	var goal *float64
	if strings.ToLower(m.Text) != "пропустить" {
		value, err := strconv.ParseFloat(m.Text, 64)
		if err != nil || value < 0 {
			b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "Введите корректное положительное число для цели или 'Пропустить':"))
			return
		}
		goal = &value
	}

	// Создаем копилку через сервис
	if err := b.services.CreateSaving(s.TempComment, goal); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Копилка успешно создана!"))
	delete(userStates, m.From.ID)

	// Показываем обновленный список накоплений
	b.showSavings(m.Chat.ID)
}

func (b *Bot) handleAmount(m *tgbotapi.Message) {
	a, err := strconv.ParseFloat(m.Text, 64)
	if err != nil {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "Введите корректную сумму, например: 1500"))
		return
	}
	s := userStates[m.From.ID]
	s.Step = "enter_comment"
	s.TempAmount = a
	userStates[m.From.ID] = s

	msg := tgbotapi.NewMessage(m.Chat.ID, "Введите комментарий:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Пропустить")))
	b.send(m.Chat.ID, msg)
}

func (b *Bot) handleComment(m *tgbotapi.Message) {
	s := userStates[m.From.ID]
	if m.Text != "Пропустить" {
		s.TempComment = m.Text
	}
	c, err := b.services.GetCategoryByID(s.TempCategoryID)
	if err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}
	amt := s.TempAmount
	if c.Type == "expense" {
		amt = -amt
	}
	if _, err := b.services.AddTransaction(amt, s.TempCategoryID, "card", s.TempComment); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}
	label := "Доход"
	if amt < 0 {
		label = "Расход"
		amt = -amt
	}
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
		fmt.Sprintf("✅ %s: %s, %.2f руб.", label, c.Name, amt)))
	delete(userStates, m.From.ID)
	b.sendMainMenu(m.Chat.ID, "✅ Операция сохранена")
}

func (b *Bot) handleNewCategory(m *tgbotapi.Message) {
	s := userStates[m.From.ID]
	if _, err := b.services.CreateCategory(m.Text, s.TempType, nil); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Категория создана"))
	delete(userStates, m.From.ID)
	b.sendMainMenu(m.Chat.ID, "Что дальше?")
}

func (b *Bot) showReport(chatID int64) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)

	trans, err := b.services.GetTransactionsForPeriod(start, end)
	if err != nil {
		b.sendError(chatID, err)
		return
	}
	var inc, exp float64
	for _, t := range trans {
		c, _ := b.services.GetCategoryByID(t.CategoryID)
		if c.Type == "income" {
			inc += t.Amount
		} else {
			exp += t.Amount
		}
	}
	text := fmt.Sprintf("📊 Статистика текущего месяца:\nДоходы: %.2f\nРасходы: %.2f\nБаланс: %.2f", inc, exp, inc-exp)
	b.send(chatID, tgbotapi.NewMessage(chatID, text))
}

func (b *Bot) showSavings(chatID int64) {
	s, err := b.services.GetSavings()
	if err != nil {
		b.send(chatID, tgbotapi.NewMessage(chatID, "Ошибка при получении накоплений"))
		return
	}

	text := "💵 Накопления:\n"
	if len(s) == 0 {
		text += "Пока нет накоплений\n"
	} else {
		for _, sv := range s {
			goalText := ""
			if sv.Goal != nil {
				goalText = fmt.Sprintf(", цель %.2f", *sv.Goal)
			}
			text += fmt.Sprintf("- %s: %.2f%s\n", sv.Name, sv.Amount, goalText)
		}
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Создать копилку", "create_saving"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	b.send(chatID, msg)
}

func (b *Bot) sendError(chatID int64, err error) {
	log.Println("bot error:", err)
	b.send(chatID, tgbotapi.NewMessage(chatID, "⚠️ Ошибка: "+err.Error()))
}

func (b *Bot) send(chatID int64, c tgbotapi.Chattable) {
	_ = chatID
	if _, err := b.bot.Send(c); err != nil {
		log.Println("send:", err)
	}
}
