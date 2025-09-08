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

		welcomeMsg := `👋 <b>Привет! Я твой финансовый помощник! 🎯</b>

💰 <i>Со мной ты сможешь:</i>
• 📊 Следить за доходами и расходами
• 💵 Копить на мечты и цели  
• 📈 Анализировать свои финансы
• 🔔 Получать полезные напоминания

🎉 <b>Давай наведем порядок в финансах вместе!</b>`

		msg := tgbotapi.NewMessage(m.Chat.ID, welcomeMsg)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💡 Советы по экономии", "saving_tips"),
				tgbotapi.NewInlineKeyboardButtonData("🚀 Начать учет", "start_transaction"),
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

	case "enter_period_start_day":
		user, err := b.repo.GetOrCreateUser(m.Chat.ID, m.From.UserName, m.From.FirstName, m.From.LastName)
		if err != nil {
			b.sendError(m.Chat.ID, err)
			return
		}
		day, err := strconv.ParseInt(m.Text, 10, 32)
		if err != nil || day < 1 || day > 31 {
			b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "⚠️ Введите число от 1 до 31:"))
			return
		}
		svc := service.NewService(b.repo, user)
		err = svc.SetPeriodStartDay(int(day))
		if err != nil {
			b.sendError(m.Chat.ID, err)
			return
		}
		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ Начало периода установлено на %d-е число.", day)))
		delete(userStates, m.From.ID)
		b.showSettingsMenu(m.Chat.ID)

	case "enter_saving_withdraw_amount":
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

		if saving.Amount < amount {
			b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "❌ Недостаточно средств в копилке!"))
			return
		}

		newAmount := saving.Amount - amount
		if err := svc.UpdateSavingAmount(savingID, newAmount); err != nil {
			b.sendError(m.Chat.ID, err)
			return
		}

		formattedAmount := b.formatCurrency(amount, m.Chat.ID)
		formattedNewAmount := b.formatCurrency(newAmount, m.Chat.ID)

		b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
			fmt.Sprintf("✅ Снято %s из копилки '%s'!\n💰 Новый баланс: %s",
				formattedAmount, saving.Name, formattedNewAmount)))

		delete(userStates, m.From.ID)
		b.showSavingActions(m.Chat.ID, savingID, svc)
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

	formattedAmount := b.formatCurrency(math.Abs(trans.Amount), chatID)

	msgText := fmt.Sprintf(
		"✏️ <b>Редактирование операции</b>\n\n"+
			"📅 Дата: %s\n"+
			"💰 Сумма: %s\n"+
			"📂 Категория: %s\n"+
			"💬 Комментарий: %s\n\n"+
			"Выберите что изменить:",
		trans.Date.Format("02.01.2006"),
		formattedAmount,
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
	state, ok := userStates[m.From.ID]
	if !ok || state.TempCategoryID == 0 {
		b.sendError(m.Chat.ID, fmt.Errorf("не выбрана категория"))
		return
	}

	if m.Text != "Пропустить" {
		state.TempComment = m.Text
	} else {
		state.TempComment = ""
	}

	editMsg := tgbotapi.NewEditMessageReplyMarkup(m.Chat.ID, m.MessageID, tgbotapi.InlineKeyboardMarkup{})
	b.bot.Send(editMsg)

	amount := state.TempAmount
	if state.TempType == "expense" {
		amount = -amount
	}

	_, err := svc.AddTransaction(amount, state.TempCategoryID, "card", state.TempComment)
	if err != nil {
		b.sendError(m.Chat.ID, err)
		return
	}

	category, err := svc.GetCategoryByID(state.TempCategoryID)
	categoryName := "Неизвестно"
	if err == nil && category != nil {
		categoryName = category.Name
	}

	operationType := "Доход"
	if amount < 0 {
		operationType = "Расход"
		amount = -amount
	}

	formattedAmount := b.formatCurrency(amount, m.Chat.ID)

	// Логирование транзакции
	logger.LogTransaction(m.From.ID, amount, categoryName)

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
		fmt.Sprintf("✅ %s: %s, %s", operationType, categoryName, formattedAmount)))

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

	formattedAmount := b.formatCurrency(amount, m.Chat.ID)
	formattedNewAmount := b.formatCurrency(newAmount, m.Chat.ID)

	// Логирование операции с копилкой
	logger.LogSaving(m.From.ID, "Пополнение", amount, saving.Name)

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID,
		fmt.Sprintf("✅ Копилка '%s' пополнена на %s!\n💰 Новый баланс: %s", saving.Name, formattedAmount, formattedNewAmount)))

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

	// Логирование создания копилки
	logger.LogSaving(m.From.ID, "Создание", 0, s.TempComment)

	b.send(m.Chat.ID, tgbotapi.NewMessage(m.Chat.ID, "🎉 Копилка создана!"))

	delete(userStates, m.From.ID)

	removeKeyboardMsg := tgbotapi.NewMessage(m.Chat.ID, "")
	removeKeyboardMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.bot.Send(removeKeyboardMsg)

	b.showSavings(m.Chat.ID, svc)
}
