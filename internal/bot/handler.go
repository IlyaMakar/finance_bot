package bot

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CallbackRenameCategory = "rename_cat_"
	CallbackDeleteCategory = "delete_cat_"
	CallbackEditCategory   = "edit_cat_"
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
		welcomeMsg := `👋 <b>Привет! Я ваш финансовый помощник!</b>

📌 <i>Вот что я умею:</i>

➕ <b>Добавить операцию</b> - учет доходов и расходов
💰 <b>Пополнить копилку</b> - пополнение ваших накоплений
📊 <b>Статистика</b> - подробные отчеты и аналитика
💵 <b>Накопления</b> - управление сберегательными целями
⚙️ <b>Настройки</b> - персонализация бота

Выберите действие кнопкой ниже:`

		msg := tgbotapi.NewMessage(m.Chat.ID, welcomeMsg)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("➕ Добавить операцию"),
				tgbotapi.NewKeyboardButton("💰 Пополнить копилку"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("📊 Статистика"),
				tgbotapi.NewKeyboardButton("💵 Накопления"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("⚙️ Настройки"),
			),
		)
		b.send(m.Chat.ID, msg)

	case "➕ Добавить операцию":
		b.startAddTransaction(m.Chat.ID)

	case "📊 Статистика":
		b.showReport(m.Chat.ID)

	case "💵 Накопления":
		b.showSavings(m.Chat.ID)

	case "💰 Пополнить копилку":
		b.startAddToSaving(m.Chat.ID)

	case "⚙️ Настройки":
		b.showSettingsMenu(m.Chat.ID)

	default:
		b.handleUserInput(m)
	}
}

func (b *Bot) showSettingsMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "⚙️ <b>Настройки</b>\n\nВыберите действие:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Управление категориями", "manage_categories"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) showCategoryManagement(chatID int64) {
	categories, err := b.services.GetCategories()
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	if len(categories) == 0 {
		b.send(chatID, tgbotapi.NewMessage(chatID, "Нет доступных категорий"))
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, cat := range categories {
		btnText := fmt.Sprintf("%s (%s)", cat.Name, cat.Type)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, CallbackEditCategory+strconv.Itoa(cat.ID)),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "settings_back"),
	))

	msg := tgbotapi.NewMessage(chatID, "📝 <b>Управление категориями</b>\n\nВыберите категорию для редактирования:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(chatID, msg)
}

func (b *Bot) showCategoryActions(chatID int64, categoryID int) {
	category, err := b.services.GetCategoryByID(categoryID)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	msgText := fmt.Sprintf("📝 <b>Категория:</b> %s\n<b>Тип:</b> %s\n\nВыберите действие:",
		category.Name, category.Type)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Переименовать", CallbackRenameCategory+strconv.Itoa(categoryID)),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Удалить", CallbackDeleteCategory+strconv.Itoa(categoryID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "manage_categories"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) handleCallback(q *tgbotapi.CallbackQuery) {

	chatID := q.From.ID
	data := q.Data

	switch {
	case data == "manage_categories":
		b.showCategoryManagement(chatID)
	case data == "settings_back":
		b.showSettingsMenu(chatID)
	case strings.HasPrefix(data, CallbackEditCategory):
		catID, _ := strconv.Atoi(data[len(CallbackEditCategory):])
		b.showCategoryActions(chatID, catID)
	case strings.HasPrefix(data, CallbackRenameCategory):
		catID, _ := strconv.Atoi(data[len(CallbackRenameCategory):])
		state := userStates[chatID]
		state.Step = "rename_category"
		state.TempCategoryID = catID
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "Введите новое название категории:"))
	case strings.HasPrefix(data, CallbackDeleteCategory):
		catID, _ := strconv.Atoi(data[len(CallbackDeleteCategory):])
		b.handleDeleteCategory(chatID, catID, q.Message.MessageID)
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

		saving, err := b.services.GetSavingByID(savingID)
		if err != nil {
			b.sendError(chatID, fmt.Errorf("не удалось найти копилку"))
			return
		}

		state := userStates[chatID]
		state.Step = "enter_saving_amount"
		state.TempCategoryID = savingID
		userStates[chatID] = state

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

func (b *Bot) handleDeleteCategory(chatID int64, categoryID int, messageID int) {
	transactions, err := b.services.GetTransactionsForPeriod(time.Now().AddDate(-10, 0, 0), time.Now())
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	hasTransactions := false
	for _, t := range transactions {
		if t.CategoryID == categoryID {
			hasTransactions = true
			break
		}
	}

	if hasTransactions {
		msg := tgbotapi.NewMessage(chatID, "⚠️ Нельзя удалить категорию, так как с ней связаны транзакции.")
		b.send(chatID, msg)
		return
	}

	err = b.services.DeleteCategory(categoryID)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	edit := tgbotapi.NewEditMessageTextAndMarkup(
		chatID,
		messageID,
		"✅ Категория успешно удалена",
		tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к категориям", "manage_categories"),
			),
		),
	)
	b.send(chatID, edit)
}

func (b *Bot) handleUserInput(m *tgbotapi.Message) {

	s, ok := userStates[m.From.ID]
	if !ok {
		b.sendMainMenu(m.Chat.ID, "Выберите действие:")
		return
	}

	switch s.Step {
	case "rename_category":
		b.handleRenameCategory(m)
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
	default:
	}
}

func (b *Bot) handleRenameCategory(m *tgbotapi.Message) {
	state := userStates[m.From.ID]
	newName := strings.TrimSpace(m.Text)

	if newName == "" {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "Название не может быть пустым. Попробуйте еще раз:"))
		return
	}

	err := b.services.RenameCategory(state.TempCategoryID, newName)
	if err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	delete(userStates, m.From.ID)
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Категория успешно переименована"))
	b.showCategoryManagement(m.Chat.ID)
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

func (b *Bot) handleSavingAmount(m *tgbotapi.Message) {
	amount, err := strconv.ParseFloat(m.Text, 64)
	if err != nil || amount <= 0 {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "Введите корректную положительную сумму:"))
		return
	}

	state := userStates[m.From.ID]
	savingID := state.TempCategoryID

	saving, err := b.services.GetSavingByID(savingID)
	if err != nil {
		b.sendError(m.Chat.ID, fmt.Errorf("не удалось получить данные копилки"))
		return
	}

	newAmount := saving.Amount + amount
	if err := b.services.UpdateSavingAmount(savingID, newAmount); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
		fmt.Sprintf("✅ Копилка '%s' пополнена на %.2f. Новый баланс: %.2f",
			saving.Name, amount, newAmount)))

	delete(userStates, m.From.ID)
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

	// Убеждаемся, что состояние пользователя сбрасывается
	delete(userStates, m.From.ID)

	// Явно отправляем главное меню после сохранения операции
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
		b.sendError(chatID, fmt.Errorf("не удалось получить статистику"))
		return
	}

	var totalIncome, totalExpense float64
	incomeDetails := make(map[string]float64)
	expenseDetails := make(map[string]float64)

	for _, t := range trans {
		c, err := b.services.GetCategoryByID(t.CategoryID)
		categoryName := "Неизвестно"
		if err == nil {
			categoryName = c.Name
		}

		if t.Amount > 0 {
			totalIncome += t.Amount
			incomeDetails[categoryName] += t.Amount
		} else {
			totalExpense += t.Amount
			expenseDetails[categoryName] += t.Amount
		}
	}

	format := func(amount float64) string {
		return fmt.Sprintf("%.2f ₽", math.Abs(amount))
	}

	var incomeDetailsStr strings.Builder
	for name, amount := range incomeDetails {
		incomeDetailsStr.WriteString(fmt.Sprintf("┣ 📈 %s: %s\n", name, format(amount)))
	}

	var expenseDetailsStr strings.Builder
	for name, amount := range expenseDetails {
		expenseDetailsStr.WriteString(fmt.Sprintf("┣ 📉 %s: %s\n", name, format(amount)))
	}

	msgText := fmt.Sprintf(
		"📊 <b>Полная финансовая статистика</b>\n"+
			"📅 Период: <i>%s</i>\n\n"+
			"💵 <b>Доходы:</b> %s\n%s\n"+
			"💸 <b>Расходы:</b> %s\n%s\n"+
			"━━━━━━━━━━━━━━━━\n"+
			"💰 <b>Итого баланс:</b> <u>%s</u>\n\n"+
			"💡 <i>Доходы/расходы по категориям</i>",
		start.Format("January 2006"),
		format(totalIncome),
		incomeDetailsStr.String(),
		format(totalExpense),
		expenseDetailsStr.String(),
		format(totalIncome+totalExpense),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
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
	s.TempComment = name          // Сохраняем название копилки во временном комментарии
	s.Step = "create_saving_goal" // Устанавливаем следующий шаг: ввод цели
	userStates[m.From.ID] = s

	// Отправляем запрос на ввод цели, прикрепляя кнопку "Пропустить"
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
	if strings.ToLower(m.Text) == "пропустить" {
		goal = nil
	} else {
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

	removeKeyboardMsg := tgbotapi.NewMessage(m.Chat.ID, "")
	removeKeyboardMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	_, err := b.bot.Send(removeKeyboardMsg)
	if err != nil {
	} else {
	}

	b.showSavings(m.Chat.ID)
}

func (b *Bot) sendMainMenu(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	menu := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Добавить операцию"),
			tgbotapi.NewKeyboardButton("💰 Пополнить копилку"),
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
	b.send(chatID, msg)
}

func (b *Bot) sendError(chatID int64, err error) {
	b.send(chatID, tgbotapi.NewMessage(chatID, "⚠️ Ошибка: "+err.Error()))
}

func (b *Bot) send(chatID int64, c tgbotapi.Chattable) {
	msg, err := b.bot.Send(c)
	if err != nil {
		log.Printf("Ошибка отправки в чат %d: %v\nСообщение: %+v", chatID, err, msg)
	}
}
