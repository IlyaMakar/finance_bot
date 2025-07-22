package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/logger"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCallback(q *tgbotapi.CallbackQuery) {
	_, _ = b.bot.Request(tgbotapi.NewCallback(q.ID, ""))
	chatID := q.From.ID
	data := q.Data

	user, err := b.repo.GetOrCreateUser(
		q.From.ID,
		q.From.UserName,
		q.From.FirstName,
		q.From.LastName,
	)
	if err != nil {
		logger.LogError(q.From.UserName, fmt.Sprintf("Ошибка: %v", err))
		b.sendError(chatID, err)
		return
	}

	svc := service.NewService(b.repo, user)

	logger.LogButtonClick(q.From.UserName, data)
	if data == CallbackManageSavings {
		b.showSavingsManagement(q.From.ID, svc)
		return
	}

	if strings.HasPrefix(data, CallbackEditSaving) {
		savingID, _ := strconv.Atoi(data[len(CallbackEditSaving):])
		b.showSavingActions(q.From.ID, savingID, q.Message.MessageID, svc)
		return
	}

	if strings.HasPrefix(data, CallbackDeleteSaving) {
		savingID, _ := strconv.Atoi(data[len(CallbackDeleteSaving):])
		b.handleDeleteSaving(q.From.ID, savingID, q.Message.MessageID, svc)
		return
	}

	if strings.HasPrefix(data, CallbackRenameSaving) {
		savingID, _ := strconv.Atoi(data[len(CallbackRenameSaving):])
		state := userStates[q.From.ID]
		state.Step = "rename_saving"
		state.TempCategoryID = savingID
		userStates[q.From.ID] = state
		b.send(q.From.ID, tgbotapi.NewMessage(q.From.ID, "✏️ Введите новое название копилки:"))
		return
	}

	if strings.HasPrefix(data, CallbackClearSaving) {
		savingID, _ := strconv.Atoi(data[len(CallbackClearSaving):])
		b.handleClearSaving(q.From.ID, savingID, q.Message.MessageID, svc)
		return
	}
	if strings.HasPrefix(data, CallbackEditCategory) {
		catID, _ := strconv.Atoi(data[len(CallbackEditCategory):])
		b.showCategoryActions(chatID, catID, svc)
		return
	}

	if strings.HasPrefix(data, CallbackRenameCategory) {
		catID, _ := strconv.Atoi(data[len(CallbackRenameCategory):])
		state := userStates[chatID]
		state.Step = "rename_category"
		state.TempCategoryID = catID
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "✏️ Введите новое название категории:"))
		return
	}

	if strings.HasPrefix(data, CallbackDeleteCategory) {
		catID, _ := strconv.Atoi(data[len(CallbackDeleteCategory):])
		b.handleDeleteCategory(chatID, catID, q.Message.MessageID, svc)
		return
	}

	if strings.HasPrefix(data, "add_to_saving_") {
		parts := strings.Split(data, "_")
		if len(parts) < 4 {
			b.sendError(chatID, fmt.Errorf("неверный формат ID копилки"))
			return
		}

		savingID, err := strconv.Atoi(parts[3])
		if err != nil {
			b.sendError(chatID, fmt.Errorf("ошибка преобразования ID копилки"))
			return
		}

		saving, err := svc.GetSavingByID(savingID)
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

		b.send(chatID, tgbotapi.NewMessage(chatID, fmt.Sprintf("💵 Вы выбрали копилку: %s\nВведите сумму для пополнения:", saving.Name)))
		return
	}

	if strings.HasPrefix(data, "cat_") {
		catID, err := strconv.Atoi(data[4:])
		if err != nil {
			b.sendError(chatID, fmt.Errorf("ошибка обработки ID категории"))
			return
		}
		b.handleCatSelect(int(chatID), catID)
		return
	}

	if strings.HasPrefix(data, "edit_") {
		transID, err := strconv.Atoi(data[5:])
		if err != nil {
			b.sendError(chatID, fmt.Errorf("неверный ID транзакции"))
			return
		}
		b.handleEditTransaction(chatID, transID, svc)
		return
	}

	switch data {
	case "cancel":
		b.sendMainMenu(chatID, "🚫 Действие отменено. Что дальше?")

	case "saving_tips":
		b.showSavingTips(chatID)

	case "start_transaction":
		b.startAddTransaction(chatID)

	case "manage_categories":
		b.showCategoryManagement(chatID, svc)

	case "settings_back":
		b.showSettingsMenu(chatID)

	case "add_to_saving":
		b.startAddToSaving(chatID, svc)

	case "savings_stats":
		b.showSavingsStats(chatID, svc)

	case "show_savings":
		b.showSavings(chatID, svc)

	case "main_menu":
		b.sendMainMenu(chatID, "Главное меню")
	case "support":
		b.showSupportInfo(chatID)

	case "skip_comment":
		editMsg := tgbotapi.NewEditMessageReplyMarkup(chatID, q.Message.MessageID, tgbotapi.InlineKeyboardMarkup{})
		b.bot.Send(editMsg)
		s := userStates[chatID]
		s.TempComment = ""
		userStates[chatID] = s
		b.handleComment(&tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			From: q.From,
			Text: "Пропустить",
		}, svc)

	case "skip_saving_goal":
		editMsg := tgbotapi.NewEditMessageReplyMarkup(chatID, q.Message.MessageID, tgbotapi.InlineKeyboardMarkup{})
		b.bot.Send(editMsg)
		s := userStates[chatID]
		s.TempAmount = 0
		userStates[chatID] = s
		b.handleCreateSavingGoal(&tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			From: q.From,
			Text: "Пропустить",
		})

	case "type_income", "type_expense":
		b.handleTypeSelect(chatID, q.Message.MessageID, data, svc)

	case "notification_settings":
		b.showNotificationSettings(chatID)

	case "enable_notifications":
		b.handleToggleNotifications(chatID, true, q.From)

	case "disable_notifications":
		b.handleToggleNotifications(chatID, false, q.From)

	case "other_cat":
		state := userStates[chatID]
		state.Step = "new_cat"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "📝 Введите название новой категории:"))

	case "create_saving":
		state := userStates[chatID]
		state.Step = "create_saving_name"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "💸 Введите название копилки:"))

	case "confirm_clear_data":
		msg := tgbotapi.NewMessage(chatID, "⚠️ <b>Внимание!</b>\n\nВы действительно хотите удалить ВСЕ свои данные? Это действие нельзя отменить!\n\nВсе транзакции, категории и копилки будут удалены.")
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить все", "clear_data"),
				tgbotapi.NewInlineKeyboardButtonData("❌ Нет, отменить", "settings_back"),
			),
		)
		b.send(chatID, msg)

	case "clear_data":
		err := svc.ClearUserData()
		if err != nil {
			logger.LogError(fmt.Sprintf("user_%d", chatID), fmt.Sprintf("Ошибка очистки данных: %v", err))
			b.sendError(chatID, err)
			return
		}

		b.initBasicCategories(user)

		b.send(chatID, tgbotapi.NewMessage(chatID, "🧹 Все данные успешно удалены! Бот сброшен к начальному состоянию."))
		b.sendMainMenu(chatID, "🔄 Вы можете начать заново!")

	case "stats_day":
		b.showDailyReport(chatID, svc)
	case "stats_week":
		b.showWeeklyReport(chatID, svc)
	case "stats_month":
		b.showMonthlyReport(chatID, svc)
	case "stats_year":
		b.showYearlyReport(chatID, svc)
	case "stats_back":
		b.showReportPeriodMenu(chatID)

	case "show_history":
		b.showTransactionHistory(chatID, svc)

	case "edit_amount":
		state := userStates[chatID]
		state.Step = "edit_transaction_amount"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "💰 Введите новую сумму:"))

	case "edit_category":
		state := userStates[chatID]
		msg := tgbotapi.NewMessage(chatID, "📂 Выберите новую категорию:")
		msg.ReplyMarkup = b.createCategoryKeyboard(chatID, state.TempType, "change_category")
		b.send(chatID, msg)

	case "edit_comment":
		state := userStates[chatID]
		state.Step = "edit_transaction_comment"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "💬 Введите новый комментарий:"))

	case "delete_transaction":
		err := svc.DeleteTransaction(userStates[chatID].TempCategoryID)
		if err != nil {
			b.sendError(chatID, err)
			return
		}
		delete(userStates, chatID)
		b.send(chatID, tgbotapi.NewMessage(chatID, "✅ Операция удалена!"))
		b.showTransactionHistory(chatID, svc)

	default:
		b.bot.Send(tgbotapi.NewCallback(q.ID, ""))
	}
}

func (b *Bot) handleTypeSelect(chatID int64, msgID int, data string, svc *service.FinanceService) {
	u := UserState{Step: "select_cat", TempType: data[5:]}
	userStates[chatID] = u

	cats, _ := svc.GetCategories()
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range cats {
		if c.Type == u.TempType {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(c.Name, "cat_"+strconv.Itoa(c.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✨ Новая категория", "other_cat"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Отмена", "cancel"),
	))
	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msgID, "📂 Выберите категорию:", tgbotapi.NewInlineKeyboardMarkup(rows...))
	b.send(chatID, edit)
}

func (b *Bot) handleCatSelect(chatID int, catID int) {
	s := userStates[int64(chatID)]
	s.Step = "enter_amount"
	s.TempCategoryID = catID
	userStates[int64(chatID)] = s

	msg := tgbotapi.NewMessage(int64(chatID), "💸 Введите сумму (например, 1500):")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.send(int64(chatID), msg)
}

func (b *Bot) handleToggleNotifications(chatID int64, enable bool, user *tgbotapi.User) {
	dbUser, err := b.repo.GetOrCreateUser(user.ID, user.UserName, user.FirstName, user.LastName)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	svc := service.NewService(b.repo, dbUser)
	err = svc.SetNotificationsEnabled(enable)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	status := "🔔 Включены"
	if !enable {
		status = "🔕 Отключены"
	}

	b.send(chatID, tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Уведомления %s", status)))
	b.showNotificationSettings(chatID)
}

func (b *Bot) handleDeleteCategory(chatID int64, categoryID int, messageID int, svc *service.FinanceService) {
	transactions, err := svc.GetTransactionsForPeriod(time.Now().AddDate(-10, 0, 0), time.Now())
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
		msg := tgbotapi.NewMessage(chatID, "⚠️ Нельзя удалить категорию, связанную с транзакциями!")
		b.send(chatID, msg)
		return
	}

	err = svc.DeleteCategory(categoryID)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	edit := tgbotapi.NewEditMessageTextAndMarkup(
		chatID,
		messageID,
		"✅ Категория удалена!",
		tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("◀️ К категориям", "manage_categories"),
			),
		),
	)
	b.send(chatID, edit)
}
