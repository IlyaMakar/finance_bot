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
	case "💰 Пополнить копилку":
		b.startAddToSaving(m.Chat.ID)
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
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Добавить операцию"),
			tgbotapi.NewKeyboardButton("💰 Пополнить копилку"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📈 Статистика"),
			tgbotapi.NewKeyboardButton("💵 Накопления"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⚙️ Настройки"),
		),
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

func (b *Bot) startAddToSaving(chatID int64) {
	savings, err := b.services.GetSavings()
	if err != nil || len(savings) == 0 {
		b.send(chatID, tgbotapi.NewMessage(chatID, "Нет доступных копилок для пополнения"))
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, s := range savings {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(s.Name, fmt.Sprintf("add_to_saving_%d", s.ID)),
		))
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите копилку для пополнения:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(chatID, msg)
}

func (b *Bot) handleCallback(q *tgbotapi.CallbackQuery) {
	chatID := q.From.ID
	switch {
	case q.Data == "type_income" || q.Data == "type_expense":
		b.handleTypeSelect(chatID, q.Message.MessageID, q.Data)
	case strings.HasPrefix(q.Data, "add_to_saving_"):
		parts := strings.Split(q.Data, "_")
		if len(parts) < 4 {
			b.sendError(chatID, fmt.Errorf("неверный формат ID копилки"))
			return
		}

		savingID, err := strconv.Atoi(parts[3])
		if err != nil {
			b.sendError(chatID, fmt.Errorf("ошибка преобразования ID копилки"))
			return
		}

		// Получаем название копилки для подтверждения
		saving, err := b.services.GetSavingByID(savingID)
		if err != nil {
			b.sendError(chatID, fmt.Errorf("не удалось найти копилку"))
			return
		}

		state := userStates[chatID]
		state.Step = "enter_saving_amount"
		state.TempCategoryID = savingID
		userStates[chatID] = state

		// Удаляем inline-клавиатуру
		edit := tgbotapi.NewEditMessageReplyMarkup(chatID, q.Message.MessageID, tgbotapi.InlineKeyboardMarkup{})
		b.bot.Send(edit)

		b.send(chatID, tgbotapi.NewMessage(chatID, fmt.Sprintf("Вы выбрали копилку: %s\nВведите сумму для пополнения:", saving.Name)))

	case strings.HasPrefix(q.Data, "cat_"):
		catID, err := strconv.Atoi(q.Data[4:])
		if err != nil {
			b.sendError(chatID, fmt.Errorf("ошибка обработки ID категории"))
			return
		}
		b.handleCatSelect(int(chatID), catID)

	case q.Data == "other_cat":
		state := userStates[chatID]
		state.Step = "new_cat"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "Введите название новой категории:"))

	case q.Data == "create_saving":
		state := userStates[chatID]
		state.Step = "create_saving_name"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "Введите название копилки:"))

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

func (b *Bot) handleCatSelect(chatID int, catID int) {
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
	case "enter_saving_amount":
		b.handleSavingAmount(m)
	case "new_cat":
		b.handleNewCategory(m)
	case "create_saving_name":
		b.handleCreateSavingName(m)
	case "create_saving_goal":
		b.handleCreateSavingGoal(m)
	}
}

func (b *Bot) handleSavingAmount(m *tgbotapi.Message) {
	amount, err := strconv.ParseFloat(m.Text, 64)
	if err != nil || amount <= 0 {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "Введите корректную положительную сумму:"))
		return
	}

	state := userStates[m.From.ID]
	savingID := state.TempCategoryID

	// Получаем текущую копилку
	saving, err := b.services.GetSavingByID(savingID)
	if err != nil {
		b.sendError(m.Chat.ID, fmt.Errorf("не удалось получить данные копилки"))
		return
	}

	// Обновляем сумму
	newAmount := saving.Amount + amount
	if err := b.services.UpdateSavingAmount(savingID, newAmount); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	// Отправляем подтверждение
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
		fmt.Sprintf("✅ Копилка '%s' пополнена на %.2f. Новый баланс: %.2f",
			saving.Name, amount, newAmount)))

	// Сбрасываем состояние
	delete(userStates, m.From.ID)

	// Показываем обновленный список
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

	var income, expense float64
	for _, t := range trans {
		if t.Amount > 0 {
			income += t.Amount
		} else {
			expense += t.Amount // Здесь expense уже будет отрицательным
		}
	}

	// Форматируем вывод
	formatMoney := func(amount float64) string {
		return fmt.Sprintf("%.2f ₽", amount)
	}

	message := fmt.Sprintf(
		"📊 <b>Финансовая статистика</b>\n"+
			"Период: %s\n\n"+
			"💵 <b>Доходы:</b> %s\n"+
			"💸 <b>Расходы:</b> %s\n"+
			"━━━━━━━━━━━━━━\n"+
			"💰 <b>Баланс:</b> %s",
		start.Format("January 2006"),
		formatMoney(income),
		formatMoney(-expense),       // Показываем расходы как положительное число
		formatMoney(income+expense), // Складываем, т.к. expense уже отрицательный
	)

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"
	b.send(chatID, msg)
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

func (b *Bot) handleCreateSavingName(m *tgbotapi.Message) {
	name := strings.TrimSpace(m.Text)
	if name == "" {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "Название не может быть пустым. Введите название копилки:"))
		return
	}

	s := userStates[m.From.ID]
	s.TempComment = name
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

	if err := b.services.CreateSaving(s.TempComment, goal); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Копилка успешно создана!"))
	delete(userStates, m.From.ID)
	b.showSavings(m.Chat.ID)
}

func (b *Bot) sendError(chatID int64, err error) {
	log.Println("bot error:", err)
	b.send(chatID, tgbotapi.NewMessage(chatID, "⚠️ Ошибка: "+err.Error()))
}

func (b *Bot) send(chatID int64, c tgbotapi.Chattable) {
	msg, err := b.bot.Send(c)
	if err != nil {
		log.Printf("Ошибка отправки в чат %d: %v\nСообщение: %+v", chatID, err, msg)
	}
}
