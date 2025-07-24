package handlers

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/IlyaMakar/finance_bot/internal/logger"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleMessage(m *tgbotapi.Message) {
	logger.LogCommand(m.From.UserName, fmt.Sprintf("Получено сообщение: %s", m.Text))

	user, err := b.repo.GetOrCreateUser(
		m.From.ID,
		m.From.UserName,
		m.From.FirstName,
		m.From.LastName,
	)
	if err != nil {
		logger.LogError(m.From.UserName, fmt.Sprintf("Ошибка получения пользователя: %v", err))
		b.sendError(m.Chat.ID, err)
		return
	}

	svc := service.NewService(b.repo, user)

	switch m.Text {
	case "/start":
		logger.LogCommand(m.From.UserName, "Команда /start")
		b.initBasicCategories(user)
		welcomeMsg := `👋 <b>Привет! Я ваш финансовый помошник!</b>

📌 <i>Вот что я умею:</i>

➕ <b>Добавить операцию</b> - учет доходов и расходов
💰 <b>Пополнить копилку</b> - пополнение ваших накоплений
📊 <b>Статистика</b> - подробные отчеты и аналитика
💵 <b>Накопления</b> - управление сберегательными целями
⚙️ <b>Настройки</b> - персонализация бота`

		msg := tgbotapi.NewMessage(m.Chat.ID, welcomeMsg)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📝 11 советов по экономии", "saving_tips"),
				tgbotapi.NewInlineKeyboardButtonData("➕ Начать учет", "start_transaction"),
			),
		)
		b.send(m.Chat.ID, msg)

	case "➕ Добавить операцию":
		logger.LogCommand(m.From.UserName, "Кнопка: Добавить операцию")
		b.startAddTransaction(m.Chat.ID)

	case "📊 Статистика":
		logger.LogCommand(m.From.UserName, "Кнопка: Статистика")
		b.showReportPeriodMenu(m.Chat.ID)

	case "⚙️ Настройки":
		logger.LogCommand(m.From.UserName, "Кнопка: Настройки")
		b.showSettingsMenu(m.Chat.ID)

	case "💵 Накопления":
		logger.LogCommand(m.From.UserName, "Кнопка: Накопления")
		b.showSavings(m.Chat.ID, svc)

	default:
		logger.LogCommand(m.From.UserName, fmt.Sprintf("Текст сообщения: %s", m.Text))
		b.handleUserInput(m, svc)
	}
}

func (b *Bot) handleUserInput(m *tgbotapi.Message, svc *service.FinanceService) {
	s, ok := userStates[m.From.ID]
	if !ok {
		b.sendMainMenu(m.Chat.ID, "🤔 Выберите действие:")
		return
	}

	switch s.Step {
	case "rename_category":
		b.handleRenameCategory(m, svc)
	case "enter_amount":
		b.handleAmount(m)
	case "enter_comment":
		b.handleComment(m, svc)
	case "enter_saving_amount":
		b.handleSavingAmount(m, svc)
	case "new_cat":
		b.handleNewCategory(m, svc)
	case "create_saving_name":
		b.handleCreateSavingName(m)
	case "create_saving_goal":
		b.handleCreateSavingGoal(m)
	case "rename_saving":
		b.handleRenameSaving(m, svc)
	case "edit_transaction_amount":
		amount, err := strconv.ParseFloat(m.Text, 64)
		if err != nil || amount <= 0 {
			b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Введите корректную сумму (например, 1500):"))
			return
		}

		state := userStates[m.From.ID]
		state.TempAmount = amount
		userStates[m.From.ID] = state

		err = svc.UpdateTransactionAmount(state.TempCategoryID, amount)
		if err != nil {
			b.sendError(m.Chat.ID, err)
			return
		}

		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Сумма обновлена!"))
		b.handleEditTransaction(m.Chat.ID, state.TempCategoryID, svc)

	case "edit_transaction_comment":
		state := userStates[m.From.ID]
		state.TempComment = m.Text
		userStates[m.From.ID] = state

		err := svc.UpdateTransactionComment(state.TempCategoryID, m.Text)
		if err != nil {
			b.sendError(m.Chat.ID, err)
			return
		}

		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Комментарий обновлен!"))
		b.handleEditTransaction(m.Chat.ID, state.TempCategoryID, svc)
	default:
		b.sendMainMenu(m.Chat.ID, "🤔 Неизвестная команда")
	}
}

func (b *Bot) handleEditTransaction(chatID int64, transactionID int, svc *service.FinanceService) {
	trans, err := svc.GetTransactionByID(transactionID)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	state := UserState{
		Step:           "edit_transaction",
		TempCategoryID: trans.ID,
		TempAmount:     math.Abs(trans.Amount),
		TempComment:    trans.Comment,
	}
	if trans.Amount < 0 {
		state.TempType = "expense"
	} else {
		state.TempType = "income"
	}
	userStates[chatID] = state

	category, err := svc.GetCategoryByID(trans.CategoryID)
	categoryName := "Неизвестно"
	if err == nil {
		categoryName = category.Name
	}

	msgText := fmt.Sprintf(
		"✏️ <b>Редактирование операции</b>\n\n"+
			"📅 Дата: %s\n"+
			"💰 Сумма: %.2f ₽\n"+
			"📂 Категория: %s\n"+
			"💬 Комментарий: %s\n\n"+
			"Выберите что изменить:",
		trans.Date.Format("02.01.2006"),
		math.Abs(trans.Amount),
		categoryName,
		trans.Comment,
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Сумма", "edit_amount"),
			tgbotapi.NewInlineKeyboardButtonData("📂 Категория", "edit_category"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💬 Комментарий", "edit_comment"),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Удалить", "delete_transaction"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "show_history"),
		),
	)
	b.send(chatID, msg)
}
func (b *Bot) handleRenameSaving(m *tgbotapi.Message, svc *service.FinanceService) {
	state := userStates[m.From.ID]
	newName := strings.TrimSpace(m.Text)

	if newName == "" {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Название не может быть пустым. Попробуйте снова:"))
		return
	}

	err := svc.RenameSaving(state.TempCategoryID, newName)
	if err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	delete(userStates, m.From.ID)
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Копилка переименована!"))
	b.showSavingsManagement(m.Chat.ID, svc)
}

func (b *Bot) handleRenameCategory(m *tgbotapi.Message, svc *service.FinanceService) {
	state := userStates[m.From.ID]
	newName := strings.TrimSpace(m.Text)

	if newName == "" {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Название не может быть пустым. Попробуйте снова:"))
		return
	}

	err := svc.RenameCategory(state.TempCategoryID, newName)
	if err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	delete(userStates, m.From.ID)
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Категория переименована!"))
	b.showCategoryManagement(m.Chat.ID, svc)
}

func (b *Bot) handleAmount(m *tgbotapi.Message) {
	a, err := strconv.ParseFloat(m.Text, 64)
	if err != nil || a <= 0 {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Введите корректную сумму (например, 1500):"))
		return
	}
	s := userStates[m.From.ID]
	s.Step = "enter_comment"
	s.TempAmount = a
	userStates[m.From.ID] = s

	msg := tgbotapi.NewMessage(m.Chat.ID, "📝 Добавьте комментарий:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Пропустить", "skip_comment"),
		),
	)
	b.send(m.Chat.ID, msg)
}

func (b *Bot) handleComment(m *tgbotapi.Message, svc *service.FinanceService) {
	s := userStates[m.From.ID]
	if m.Text != "Пропустить" {
		s.TempComment = m.Text
	} else {
		s.TempComment = ""
	}

	editMsg := tgbotapi.NewEditMessageReplyMarkup(m.Chat.ID, m.MessageID, tgbotapi.InlineKeyboardMarkup{})
	b.bot.Send(editMsg)

	c, err := svc.GetCategoryByID(s.TempCategoryID)
	if err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}
	amt := s.TempAmount
	if c.Type == "expense" {
		amt = -amt
	}
	if _, err := svc.AddTransaction(amt, s.TempCategoryID, "card", s.TempComment); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}
	label := "Доход"
	if amt < 0 {
		label = "Расход"
		amt = -amt
	}
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
		fmt.Sprintf("✅ %s: %s, %.2f ₽", label, c.Name, amt)))

	delete(userStates, m.From.ID)
	b.sendMainMenu(m.Chat.ID, "🎉 Операция добавлена! Что дальше?")
}

func (b *Bot) handleSavingAmount(m *tgbotapi.Message, svc *service.FinanceService) {
	amount, err := strconv.ParseFloat(m.Text, 64)
	if err != nil || amount <= 0 {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Введите корректную сумму (например, 500):"))
		return
	}

	state := userStates[m.From.ID]
	savingID := state.TempCategoryID

	saving, err := svc.GetSavingByID(savingID)
	if err != nil {
		b.sendError(m.Chat.ID, fmt.Errorf("не удалось найти копилку"))
		return
	}

	newAmount := saving.Amount + amount
	if err := svc.UpdateSavingAmount(savingID, newAmount); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
		fmt.Sprintf("✅ Копилка '%s' пополнена на %.2f ₽!\n💰 Новый баланс: %.2f ₽", saving.Name, amount, newAmount)))

	delete(userStates, m.From.ID)
	b.showSavings(m.Chat.ID, svc)
}

func (b *Bot) handleNewCategory(m *tgbotapi.Message, svc *service.FinanceService) {
	s := userStates[m.From.ID]
	if _, err := svc.CreateCategory(m.Text, s.TempType, nil); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}
	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "✅ Новая категория создана!"))
	delete(userStates, m.From.ID)
	b.sendMainMenu(m.Chat.ID, "🎉 Что дальше?")
}

func (b *Bot) handleCreateSavingName(m *tgbotapi.Message) {
	name := strings.TrimSpace(m.Text)
	if name == "" {
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Название копилки не может быть пустым. Попробуйте снова:"))
		return
	}

	s := userStates[m.From.ID]
	s.TempComment = name
	s.Step = "create_saving_goal"
	userStates[m.From.ID] = s

	msg := tgbotapi.NewMessage(m.Chat.ID, "🎯 Введите цель копилки (число):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Пропустить", "skip_saving_goal"),
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
			b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Введите корректное число для цели или «Пропустить»:"))
			return
		}
		goal = &value
	}

	user, err := b.repo.GetOrCreateUser(
		m.From.ID,
		m.From.UserName,
		m.From.FirstName,
		m.From.LastName,
	)
	if err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	svc := service.NewService(b.repo, user)

	if err := svc.CreateSaving(s.TempComment, goal); err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "🎉 Копилка создана!"))

	delete(userStates, m.From.ID)

	removeKeyboardMsg := tgbotapi.NewMessage(m.Chat.ID, "")
	removeKeyboardMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.bot.Send(removeKeyboardMsg)

	b.showSavings(m.Chat.ID, svc)
}
