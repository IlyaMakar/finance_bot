package handlers

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/logger"
	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) startAddTransaction(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "💸 Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📈 Доход", "type_income"),
			tgbotapi.NewInlineKeyboardButtonData("📉 Расход", "type_expense"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📜 История операций", "show_history"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Отмена", "cancel"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) startAddToSaving(chatID int64, svc *service.FinanceService) {
	savings, err := svc.GetSavings()
	if err != nil || len(savings) == 0 {
		b.send(chatID, tgbotapi.NewMessage(chatID, "😔 У вас пока нет копилок для пополнения. Создайте одну в разделе «Накопления»!"))
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, s := range savings {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("💵 %s", s.Name), fmt.Sprintf("add_to_saving_%d", s.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Отмена", "cancel"),
	))

	msg := tgbotapi.NewMessage(chatID, "🎯 Выберите копилку для пополнения:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(chatID, msg)
}

func (b *Bot) createSavingsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("➕ Новая копилка", "create_saving"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("💰 Пополнить", "add_to_saving"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", "savings_stats"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", "manage_savings"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "main_menu"),
		},
	)
}

func (b *Bot) showSavingsManagement(chatID int64, svc *service.FinanceService) {
	savings, err := svc.GetSavings()
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	if len(savings) == 0 {
		b.send(chatID, tgbotapi.NewMessage(chatID, "😔 У вас пока нет копилок для редактирования."))
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, s := range savings {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(s.Name, CallbackEditSaving+strconv.Itoa(s.ID)),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "show_savings"),
	))

	msg := tgbotapi.NewMessage(chatID, "✏️ <b>Редактирование копилок</b>\n\nВыберите копилку:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(chatID, msg)
}

func (b *Bot) showSavingActions(chatID int64, savingID int, svc *service.FinanceService) {
	saving, err := svc.GetSavingByID(savingID)
	if err != nil {
		b.sendError(chatID, fmt.Errorf("не удалось получить копилку: %v", err))
		return
	}

	formattedAmount := b.formatCurrency(saving.Amount, chatID)
	msgText := fmt.Sprintf("📌 <b>%s</b>\nТекущая сумма: %s", saving.Name, formattedAmount)
	if saving.Goal != nil {
		progress := saving.Progress()
		formattedGoal := b.formatCurrency(*saving.Goal, chatID)
		msgText += fmt.Sprintf("\nЦель: %s (%.1f%%)", formattedGoal, progress)
	}
	if saving.Comment != "" {
		msgText += fmt.Sprintf("\nКомментарий: %s", saving.Comment)
	}

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Пополнить", fmt.Sprintf("saving_add_%d", savingID)),
			tgbotapi.NewInlineKeyboardButtonData("➖ Снять", fmt.Sprintf("saving_withdraw_%d", savingID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Переименовать", fmt.Sprintf("saving_rename_%d", savingID)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("saving_delete_%d", savingID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "savings_list"),
		),
	)

	if _, err := b.bot.Send(msg); err != nil {
		logger.LogError(chatID, fmt.Sprintf("Ошибка отправки сообщения showSavingActions: %v", err))
		b.sendError(chatID, err)
		return
	}
}

func (b *Bot) handleDeleteSaving(chatID int64, savingID int, messageID int, svc *service.FinanceService) {
	err := svc.DeleteSaving(savingID)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	edit := tgbotapi.NewEditMessageTextAndMarkup(
		chatID,
		messageID,
		"✅ Копилка удалена!",
		tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("◀️ К списку копилок", "manage_savings"),
			),
		),
	)
	b.send(chatID, edit)
}

func (b *Bot) handleClearSaving(chatID int64, savingID int, messageID int, svc *service.FinanceService) {
	err := svc.UpdateSavingAmount(savingID, 0)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	edit := tgbotapi.NewEditMessageTextAndMarkup(
		chatID,
		messageID,
		"✅ Копилка очищена!",
		tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("◀️ К списку копилок", "manage_savings"),
			),
		),
	)
	b.send(chatID, edit)
}

func (b *Bot) initBasicCategories(user *repository.User) {
	basicCategories := []struct{ name, typ string }{
		{"🍎 Продукты", "expense"},
		{"🚗 Транспорт", "expense"},
		{"🏠 ЖКХ", "expense"},
		{"💼 Зарплата", "income"},
		{"🎉 Развлечения", "expense"},
	}

	exists, _ := b.repo.GetCategories(user.ID)
	wrap := map[string]bool{}
	for _, c := range exists {
		wrap[c.Name] = true
	}

	for _, category := range basicCategories {
		if !wrap[category.name] {
			if _, err := b.repo.CreateCategory(user.ID, repository.Category{
				Name: category.name,
				Type: category.typ,
			}); err != nil {
				log.Println("Ошибка инициализации категории:", err)
			}
		}
	}
}

func (b *Bot) showSavingTips(chatID int64) {
	tips := `💡 <b>11 причин вести учет финансов</b>

👋 Привет! Знаю, учет финансов может звучать как что-то скучное, будто нужно сидеть с калькулятором и ворчать над каждой мелочью. 😅 Но на деле это про контроль, уверенность и путь к твоим мечтам! Вот 11 причин, почему учет финансов — это твой лучший друг:

1. 🕵️‍♂️ <b>Обнаружить "утечки" бюджета</b>
Мелкие траты — кофе, подписки, "нужные" вещички — незаметно съедают бюджет. Учет покажет, сколько ты потратил на доставку еды или спонтанные покупки. Например, 5 000 ₽ в месяц на кофе? Это пара крутых кроссовок за год! Узнай, где утекают деньги, и направь их на что-то важное. 🥐

2. 🤔 <b>Разобраться, куда уходят деньги</b>
К концу месяца кажется, что деньги просто исчезли? Учет дает ясную картину: 25% на аренду, 15% на продукты, 10% на развлечения. Ты видишь, сколько реально уходит на каждую категорию, и можешь планировать бюджет без сюрпризов. Больше никакого "где мои деньги?"! 📊

3. 🧘‍♀️ <b>Избавиться от финансовой тревоги</b>
Не знать, хватит ли денег до зарплаты, — это стресс. Учет показывает твои доходы, расходы и остаток. Зная, что у тебя есть 10 000 ₽ на две недели, ты чувствуешь себя увереннее. Это как карта в путешествии — ты всегда знаешь, где находишься. Спокойствие гарантировано! 😌

4. 🎠 <b>Предотвратить долговую спираль</b>
Кредитки и займы могут незаметно затянуть, если траты опережают доходы. Учет покажет, если ты тратишь больше, чем зарабатываешь. Например, если 30% дохода уходит на выплаты по кредитам, это сигнал пересмотреть привычки. Учет помогает жить по средствам и избегать долгов. 💳

5. 🥳 <b>Баловать себя без чувства вины</b>
Когда финансы под контролем, ты можешь выделить бюджет на удовольствия — новый гаджет, поход в кафе или спа. Учет позволяет заранее отложить 2 000 ₽ на "радости", и ты наслаждаешься ими, не переживая, что пробил дыру в бюджете. Живи ярко, но осознанно! 🎉

6. ✨ <b>Превратить мечты в реальные цели</b>
Мечтаешь о путешествии или новом ноутбуке? Учет делает мечты конкретными. Вместо "хочу на Бали" ты видишь: "Нужно 80 000 ₽, откладываю 8 000 ₽ в месяц, через 10 месяцев — чемодан в руки!" Цифры превращают желания в план, который легко выполнить. 🏝️

7. 💰 <b>Найти скрытые ресурсы для целей</b>
Учет помогает обнаружить, где можно сэкономить. Например, сократив траты на такси на 3 000 ₽ в месяц, ты можешь отложить эти деньги на новый телефон или курс обучения. Анализ трат подсказывает, как оптимизировать бюджет без лишений, чтобы быстрее достичь мечты. 🚀

8. 📈 <b>Мотивироваться своим прогрессом</b>
Видеть, как растут накопления или уменьшается долг, — это как проходить уровни в игре! Каждый месяц твоя копилка на машину увеличивается на 15 000 ₽, или долг по кредитке сокращается на 5 000 ₽. Это вдохновляет продолжать и делает финансы увлекательными. 💪

9. ⏳<b> Оценить ценность своего времени</b>
Посчитай, сколько стоит твой час работы. Если ты зарабатываешь 400 ₽ в час, а новый свитер стоит 4 000 ₽, это 10 часов труда. Стоит ли он того? Учет помогает взвешивать покупки и ценить свое время, делая решения более осознанными. 🕰️

10. ✅ <b>Принимать решения на основе фактов</b>
Покупать дорогой гаджет или подождать? Учет дает ответ: если после покупки у тебя останется всего 3 000 ₽ на месяц, лучше отложить. Цифры не врут, и ты можешь принимать решения, основанные на реальных данных, а не на эмоциях. Это как компас в мире финансов! 🧭

11. 📊 <b>Управлять нестабильным доходом</b>
Для фрилансеров, репетиторов или мастеров с плавающим доходом учет — спасение. Он показывает средний доход за месяц, выявляет сезонные спады и помогает планировать. Например, зная, что в декабре заказов меньше, ты отложишь деньги заранее. Порядок вместо хаоса! 💼

💸 <b>Начни прямо сейчас!</b> Учет — это не про скуку, а про контроль и свободу. Всего пара минут в день, и твои финансы превратятся из загадки в четкий план. Сделай первый шаг к финансовой уверенности! 🚀`

	msg := tgbotapi.NewMessage(chatID, tips)
	msg.ParseMode = "HTML"
	b.send(chatID, msg)
	b.sendMainMenu(chatID, "🎉 Что дальше?")
}

func (b *Bot) showSettingsMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "⚙️ <b>Настройки</b>\n\nВыбери, что хочешь настроить:")
	msg.ParseMode = "HTML"

	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{tgbotapi.NewInlineKeyboardButtonData("🔔 Уведомления", "notification_settings")},
		{tgbotapi.NewInlineKeyboardButtonData("📝 Категории", "manage_categories")},
		{tgbotapi.NewInlineKeyboardButtonData("📅 Период отчётов", "period_settings")},
		{tgbotapi.NewInlineKeyboardButtonData("💱 Валюта", CallbackCurrencySettings)},
		{tgbotapi.NewInlineKeyboardButtonData("🆘 Поддержка", "support")},
		{tgbotapi.NewInlineKeyboardButtonData("🧹 Очистить все данные", "confirm_clear_data")},
	}

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	b.send(chatID, msg)
}

func (b *Bot) showNotificationSettings(chatID int64) {
	user, err := b.repo.GetOrCreateUser(chatID, "", "", "")
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	svc := service.NewService(b.repo, user)
	enabled, err := svc.GetNotificationsEnabled()
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	status := "🔕 Отключены"
	if enabled {
		status = "🔔 Включены"
	}

	msg := tgbotapi.NewMessage(chatID,
		fmt.Sprintf("🔔 <b>Уведомления</b>\n\nТекущий статус: %s\n\nВыбери действие:", status))
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🔔 Включить", "enable_notifications"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🔕 Отключить", "disable_notifications"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("◀️ В меню", "settings_back"),
		},
	)
	b.send(chatID, msg)
}

func (b *Bot) showCategoryManagement(chatID int64, svc *service.FinanceService) {
	categories, err := svc.GetCategories()
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	if len(categories) == 0 {
		b.send(chatID, tgbotapi.NewMessage(chatID, "😔 У вас пока нет категорий. Создайте новую в меню!"))
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
		tgbotapi.NewInlineKeyboardButtonData("◀️ В меню", "settings_back"),
	))

	msg := tgbotapi.NewMessage(chatID, "📝 <b>Категории</b>\n\nВыбери категорию для редактирования:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(chatID, msg)
}

func (b *Bot) showSupportInfo(chatID int64) {
	supportText := `🆘 <b>Поддержка</b>

Выберите действие:`

	msg := tgbotapi.NewMessage(chatID, supportText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✉️ Написать разработчику", CallbackWriteSupport),
			tgbotapi.NewInlineKeyboardButtonData("❓ FAQ", CallbackFAQ),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "settings_back"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) showCurrencyMenu(chatID int64) {
	user, err := b.repo.GetOrCreateUser(chatID, "", "", "")
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	currentCurrency, _ := b.repo.GetUserCurrency(user.ID)

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("💱 Текущая валюта: %s\nВыберите новую валюту:", currentCurrency))

	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🇷🇺 RUB (Рубли)%s", b.getCurrencyCheckmark(currentCurrency, CurrencyRUB)), CallbackSetCurrency+CurrencyRUB)},
		{tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🇺🇸 USD (Доллары)%s", b.getCurrencyCheckmark(currentCurrency, CurrencyUSD)), CallbackSetCurrency+CurrencyUSD)},
		{tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🇪🇺 EUR (Евро)%s", b.getCurrencyCheckmark(currentCurrency, CurrencyEUR)), CallbackSetCurrency+CurrencyEUR)},
		{tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "settings_back")},
	}

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	b.send(chatID, msg)
}

func (b *Bot) formatCurrency(amount float64, chatID int64) string {
	user, err := b.repo.GetOrCreateUser(chatID, "", "", "")
	if err != nil {
		return fmt.Sprintf("%.2f ₽", amount) // По умолчанию RUB
	}

	currency, err := b.repo.GetUserCurrency(user.ID)
	if err != nil {
		return fmt.Sprintf("%.2f ₽", amount)
	}

	switch currency {
	case CurrencyRUB:
		return fmt.Sprintf("%.2f ₽", amount)
	case CurrencyUSD:
		return fmt.Sprintf("$%.2f", amount)
	case CurrencyEUR:
		return fmt.Sprintf("€%.2f", amount)
	default:
		return fmt.Sprintf("%.2f %s", amount, currency)
	}
}

func (b *Bot) getCurrencyCheckmark(current, selected string) string {
	if current == selected {
		return " ✅"
	}
	return ""
}

func (b *Bot) handleSetCurrency(chatID int64, currency string) {
	user, err := b.repo.GetOrCreateUser(chatID, "", "", "")
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	if err := b.repo.SetUserCurrency(user.ID, currency); err != nil {
		b.sendError(chatID, err)
		return
	}

	symbols := map[string]string{
		CurrencyRUB: "₽",
		CurrencyUSD: "$",
		CurrencyEUR: "€",
	}

	b.send(chatID, tgbotapi.NewMessage(chatID,
		fmt.Sprintf("✅ Валюта изменена на %s %s", currency, symbols[currency])))
	b.showSettingsMenu(chatID)
}

func (b *Bot) showCategoryActions(chatID int64, categoryID int, svc *service.FinanceService) {
	category, err := svc.GetCategoryByID(categoryID)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	msgText := fmt.Sprintf("📝 <b>Категория:</b> %s\n<b>Тип:</b> %s\n\nЧто сделать?", category.Name, category.Type)

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

func (b *Bot) showReportPeriodMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "📊 Выберите период для статистики:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("День", "stats_day"),
			tgbotapi.NewInlineKeyboardButtonData("Неделя", "stats_week"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Месяц", "stats_month"),
			tgbotapi.NewInlineKeyboardButtonData("Год", "stats_year"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "main_menu"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) showSavingsStats(chatID int64, svc *service.FinanceService) {
	savings, err := svc.GetSavings()
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	var totalSaved, totalGoal float64
	var msgText strings.Builder
	msgText.WriteString("📊 *Статистика копилок*\n\n")

	for _, s := range savings {
		if s.Goal != nil {
			totalSaved += s.Amount
			totalGoal += *s.Goal
			progress := b.renderProgressBar(s.Progress(), 10)

			formattedAmount := b.formatCurrency(s.Amount, chatID)
			formattedGoal := b.formatCurrency(*s.Goal, chatID)

			msgText.WriteString(fmt.Sprintf(
				"🔹 *%s*\n"+
					"┣ Накоплено: *%s*\n"+
					"┣ Цель: *%s*\n"+
					"┗ Прогресс: %s\n\n",
				s.Name, formattedAmount, formattedGoal, progress,
			))
		}
	}

	msg := tgbotapi.NewMessage(chatID, msgText.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к копилкам", "show_savings"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) showTransactionHistory(chatID int64, svc *service.FinanceService) {
	end := time.Now()
	start := end.AddDate(0, -1, 0)

	transactions, err := svc.GetTransactionsForPeriod(start, end)
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	var msgText strings.Builder
	msgText.WriteString("📜 <b>История операций</b>\n\n")

	for i, t := range transactions {
		formattedDate := t.Date.Format("02.01.2006")
		formattedAmount := b.formatCurrency(math.Abs(t.Amount), chatID)

		operationIcon := "📈"
		operationType := "Доход"
		if t.Amount < 0 {
			operationIcon = "📉"
			operationType = "Расход"
		}

		msgText.WriteString(fmt.Sprintf(
			"<b>%d. %s %s %s</b>\n"+
				"┣ Категория: %s\n"+
				"┣ Сумма: <code>%s</code>\n",
			i+1, formattedDate, operationIcon, operationType,
			t.CategoryName, formattedAmount))

		if t.Comment != "" {
			msgText.WriteString(fmt.Sprintf("┣ Комментарий: %s\n", t.Comment))
		}
		msgText.WriteString("\n")
	}

	msg := tgbotapi.NewMessage(chatID, msgText.String())
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "stats_back"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) createCategoryKeyboard(chatID int64, typ string, prefix string) tgbotapi.InlineKeyboardMarkup {
	user, err := b.repo.GetOrCreateUser(chatID, "", "", "")
	if err != nil {
		return tgbotapi.NewInlineKeyboardMarkup()
	}

	svc := service.NewService(b.repo, user)
	categories, err := svc.GetCategories()
	if err != nil {
		return tgbotapi.NewInlineKeyboardMarkup()
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, cat := range categories {
		if cat.Type == typ {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(cat.Name, prefix+"_"+strconv.Itoa(cat.ID)),
			))
		}
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "edit_"+strconv.Itoa(userStates[chatID].TempCategoryID)),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (b *Bot) showSavings(chatID int64, svc *service.FinanceService) {
	savings, err := svc.GetSavings()
	if err != nil {
		b.sendError(chatID, err)
		return
	}

	var msgText strings.Builder
	msgText.WriteString("💵 *Ваши копилки*\n\n")

	if len(savings) == 0 {
		msgText.WriteString("У вас пока нет копилок. Создайте первую!")
	} else {
		for _, s := range savings {
			progress := ""
			formattedAmount := b.formatCurrency(s.Amount, chatID)
			if s.Goal != nil {
				progress = b.renderProgressBar(s.Progress(), 10)
				formattedGoal := b.formatCurrency(*s.Goal, chatID)
				msgText.WriteString(fmt.Sprintf(
					"🔹 *%s*\n"+
						"┣ Накоплено: *%s*\n"+
						"┣ Цель: *%s*\n"+
						"┗ Прогресс: %s\n\n",
					s.Name, formattedAmount, formattedGoal, progress,
				))
			} else {
				msgText.WriteString(fmt.Sprintf(
					"🔹 *%s*\n"+
						"┣ Накоплено: *%s*\n\n",
					s.Name, formattedAmount,
				))
			}
		}
	}

	msg := tgbotapi.NewMessage(chatID, msgText.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = b.createSavingsKeyboard()
	b.send(chatID, msg)
}

func (b *Bot) showDailyReport(chatID int64, svc *service.FinanceService) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	b.generatePeriodReport(chatID, svc, start, end, "день")
}

func (b *Bot) showWeeklyReport(chatID int64, svc *service.FinanceService) {
	now := time.Now()
	start := now.AddDate(0, 0, -6)
	end := now
	b.generatePeriodReport(chatID, svc, start, end, "неделю")
}

func (b *Bot) showMonthlyReport(chatID int64, svc *service.FinanceService) {
	user, err := b.repo.GetOrCreateUser(chatID, "", "", "")
	if err != nil {
		b.sendError(chatID, err)
		return
	}
	now := time.Now()
	startDay := user.PeriodStartDay
	var start, end time.Time
	if startDay <= now.Day() {
		start = time.Date(now.Year(), now.Month(), startDay, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0)
	} else {
		prevMonth := now.AddDate(0, -1, 0)
		start = time.Date(prevMonth.Year(), prevMonth.Month(), startDay, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), startDay, 0, 0, 0, 0, now.Location())
	}
	b.generatePeriodReport(chatID, svc, start, end, "месяц")
}

func (b *Bot) showYearlyReport(chatID int64, svc *service.FinanceService) {
	user, err := b.repo.GetOrCreateUser(chatID, "", "", "")
	if err != nil {
		b.sendError(chatID, err)
		return
	}
	now := time.Now()
	startDay := user.PeriodStartDay
	var start, end time.Time
	if now.Month() > 1 || (now.Month() == 1 && now.Day() >= startDay) {
		start = time.Date(now.Year(), 1, startDay, 0, 0, 0, 0, now.Location())
		end = start.AddDate(1, 0, 0)
	} else {
		start = time.Date(now.Year()-1, 1, startDay, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), 1, startDay, 0, 0, 0, 0, now.Location())
	}
	b.generatePeriodReport(chatID, svc, start, end, "год")
}

func (b *Bot) generatePeriodReport(chatID int64, svc *service.FinanceService, start, end time.Time, periodName string) {
	trans, err := svc.GetTransactionsForPeriod(start, end)
	if err != nil {
		logger.LogError(chatID, fmt.Sprintf("Ошибка получения транзакций для отчета: %v", err))
		b.sendError(chatID, err)
		return
	}

	var totalIncome, totalExpense float64
	incomeDetails := make(map[string]float64)
	expenseDetails := make(map[string]float64)

	for _, t := range trans {
		catName := t.CategoryName
		if catName == "" {
			catName = "Неизвестно"
		}

		if t.Amount > 0 {
			totalIncome += t.Amount
			incomeDetails[catName] += t.Amount
		} else {
			amount := math.Abs(t.Amount)
			totalExpense += amount
			expenseDetails[catName] += amount
		}
	}

	msgText := strings.Builder{}
	msgText.WriteString(fmt.Sprintf("📊 <b>Статистика за %s</b>\n\n", periodName))

	formattedTotalIncome := b.formatCurrency(totalIncome, chatID)
	msgText.WriteString(fmt.Sprintf("📈 <b>Доходы:</b> %s\n", formattedTotalIncome))
	if len(incomeDetails) == 0 {
		msgText.WriteString("┣ Нет доходов\n")
	} else {
		for cat, amount := range incomeDetails {
			if cat == "" {
				cat = "Неизвестно"
			}
			formattedAmount := b.formatCurrency(amount, chatID)
			msgText.WriteString(fmt.Sprintf("┣ %s: %s\n", cat, formattedAmount))
		}
	}

	formattedTotalExpense := b.formatCurrency(totalExpense, chatID)
	msgText.WriteString(fmt.Sprintf("\n📉 <b>Расходы:</b> %s\n", formattedTotalExpense))
	if len(expenseDetails) == 0 {
		msgText.WriteString("┣ Нет расходов\n")
	} else {
		sortedCategories := sortCategoriesByAmount(expenseDetails)
		for _, cat := range sortedCategories {
			if cat == "" {
				cat = "Неизвестно"
			}
			amount := expenseDetails[cat]
			percentage := 0.0
			if totalExpense > 0 {
				percentage = (amount / totalExpense) * 100
			}
			formattedAmount := b.formatCurrency(amount, chatID)
			msgText.WriteString(fmt.Sprintf("┣ %s: %s (%.1f%%)\n", cat, formattedAmount, percentage))
		}
	}

	formattedBalance := b.formatCurrency(totalIncome-totalExpense, chatID)
	msgText.WriteString(fmt.Sprintf("\n💵 <b>Баланс:</b> %s", formattedBalance))

	finalMsg := msgText.String()
	if len(finalMsg) > 4096 {
		logger.LogError(chatID, "Длина сообщения статистики превышает лимит Telegram (4096 символов)")
		b.sendError(chatID, fmt.Errorf("отчет слишком длинный, попробуйте выбрать меньший период"))
		return
	}

	msg := tgbotapi.NewMessage(chatID, finalMsg)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "stats_back"),
			tgbotapi.NewInlineKeyboardButtonData("📤 Выгрузить отчет", fmt.Sprintf("export_report_%s_%s",
				start.Format("2006-01-02"),
				end.Format("2006-01-02"))),
		),
	)

	if _, err := b.bot.Send(msg); err != nil {
		logger.LogError(chatID, fmt.Sprintf("Ошибка отправки сообщения статистики: %v", err))
		b.sendError(chatID, fmt.Errorf("не удалось отправить отчет: %v", err))
		return
	}

	logger.LogButtonClickByID(chatID, fmt.Sprintf("Статистика за %s", periodName))
}

func sortCategoriesByAmount(details map[string]float64) []string {
	type kv struct {
		Key   string
		Value float64
	}

	var sorted []kv
	for k, v := range details {
		sorted = append(sorted, kv{k, v})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	result := make([]string, len(sorted))
	for i, kv := range sorted {
		result[i] = kv.Key
	}
	return result
}

func (b *Bot) SendReminder(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, `🔔 <b>Напоминание</b>

Привет! Сегодня ты не добавлял(а) транзакции. 

💡 Веди учет, чтобы лучше управлять финансами! 

➕ Нажми «Добавить операцию» или напиши сумму и комментарий, например:
<code>150 такси</code>`)
	msg.ParseMode = "HTML"
	b.send(chatID, msg)
}

func (b *Bot) showWriteSupport(chatID int64) {
	writeText := `✉️ <b>Написать разработчику</b>

Если у вас возникли вопросы или проблемы, напишите напрямую: @LONEl1st

Или оставьте issue на GitHub: https://github.com/IlyaMakar/finance_bot`

	msg := tgbotapi.NewMessage(chatID, writeText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к поддержке", "support"),
		),
	)
	b.send(chatID, msg)
}

func (b *Bot) showFAQ(chatID int64) {
	faqText := `❓ <b>FAQ (Часто задаваемые вопросы)</b>

Вот основная информация о боте. Если чего-то не хватает, напишите в поддержку!

1. <b>Как добавить операцию?</b>
   - Нажмите "➕ Добавить операцию" в главном меню.
   - Выберите тип (Доход или Расход).
   - Выберите категорию или создайте новую.
   - Введите сумму и комментарий (опционально).

2. <b>Как управлять копилками?</b>
   - Перейдите в "💵 Накопления".
   - Создайте новую копилку, укажите имя и цель (опционально).
   - Пополняйте, редактируйте или удаляйте копилки.

3. <b>Как посмотреть статистику?</b>
   - Нажмите "📊 Статистика".
   - Выберите период: день, неделя, месяц, год.
   - Бот покажет доходы, расходы, баланс и детали по категориям.

4. <b>Как включить/отключить уведомления?</b>
   - В "⚙️ Настройки" выберите "🔔 Уведомления".
   - Включите или отключите. Уведомления приходят в 16:00, если нет транзакций за день.

5. <b>Как изменить валюту?</b>
   - В "⚙️ Настройки" выберите "💱 Валюта".
   - Выберите RUB, USD или EUR.

6. <b>Как очистить все данные?</b>
   - В "⚙️ Настройки" нажмите "🧹 Очистить все данные".
   - Подтвердите — это удалит транзакции, категории и копилки.

7. <b>Что делать, если бот не отвечает?</b>
   - Проверьте интернет. Если проблема persists, напишите @LONEl1st.

8. <b>Обновления бота</b>
   - Бот уведомит вас о новых версиях. Читайте описания для новых фич.

Если вопрос не покрыт, напишите разработчику!`

	msg := tgbotapi.NewMessage(chatID, faqText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад к поддержке", "support"),
		),
	)
	b.send(chatID, msg)
}
