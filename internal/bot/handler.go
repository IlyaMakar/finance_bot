package bot

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/logger"
	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CallbackRenameCategory = "rename_cat_"
	CallbackDeleteCategory = "delete_cat_"
	CallbackEditCategory   = "edit_cat_"
)

const (
	CallbackToggleNotifications = "toggle_notifs_"
)

type Bot struct {
	bot  *tgbotapi.BotAPI
	repo *repository.SQLiteRepository
}

type UserState struct {
	Step           string
	TempCategoryID int
	TempAmount     float64
	TempComment    string
	TempType       string
}

var userStates = make(map[int64]UserState)

func NewBot(token string, repo *repository.SQLiteRepository) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{bot: botAPI, repo: repo}, nil
}

func (b *Bot) startAddTransaction(chatID int64) {
	keyb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📈 Доход", "type_income"),
			tgbotapi.NewInlineKeyboardButtonData("📉 Расход", "type_expense"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Отмена", "cancel"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "💸 Выберите тип операции:")
	msg.ReplyMarkup = keyb
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
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Новая копилка", "create_saving"),
			tgbotapi.NewInlineKeyboardButtonData("💰 Пополнить", "add_to_saving"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", "savings_stats"),
			tgbotapi.NewInlineKeyboardButtonData("◀️ Назад", "main_menu"),
		),
	)
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

func (b *Bot) handleMessage(m *tgbotapi.Message) {
	userID := m.From.ID
	logger.LogCommand(userID, fmt.Sprintf("Получено сообщение: %s", m.Text))

	user, err := b.repo.GetOrCreateUser(
		m.From.ID,
		m.From.UserName,
		m.From.FirstName,
		m.From.LastName,
	)
	if err != nil {
		logger.LogError(userID, fmt.Sprintf("Ошибка получения пользователя: %v", err))
		b.sendError(m.Chat.ID, err)
		return
	}

	svc := service.NewService(b.repo, user)

	switch m.Text {
	case "/start":
		logger.LogCommand(userID, "Команда /start")
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
		logger.LogCommand(userID, "Кнопка: Добавить операцию")
		b.startAddTransaction(m.Chat.ID)

	case "📊 Статистика":
		logger.LogCommand(userID, "Кнопка: Статистика")
		b.showReport(m.Chat.ID, svc)

	case "⚙️ Настройки":
		logger.LogCommand(userID, "Кнопка: Настройки")
		b.showSettingsMenu(m.Chat.ID)

	case "💵 Накопления":
		logger.LogCommand(userID, "Кнопка: Накопления")
		b.showSavings(m.Chat.ID, svc)

	default:
		logger.LogCommand(userID, fmt.Sprintf("Текст сообщения: %s", m.Text))
		b.handleUserInput(m, svc)
	}
}

func (b *Bot) showSettingsMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "⚙️ <b>Настройки</b>\n\nВыбери, что хочешь настроить:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔔 Уведомления", "notification_settings"),
			tgbotapi.NewInlineKeyboardButtonData("📝 Категории", "manage_categories"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧹 Очистить все данные", "confirm_clear_data"),
		),
	)
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
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔔 Включить", "enable_notifications"),
			tgbotapi.NewInlineKeyboardButtonData("🔕 Отключить", "disable_notifications"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ В меню", "settings_back"),
		),
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
		logger.LogError(chatID, fmt.Sprintf("Ошибка получения пользователя: %v", err))
		b.sendError(chatID, err)
		return
	}

	svc := service.NewService(b.repo, user)

	logger.LogButtonClick(chatID, data)

	switch {
	case data == "cancel":
		b.sendMainMenu(chatID, "🚫 Действие отменено. Что дальше?")

	case data == "saving_tips":
		b.showSavingTips(chatID)

	case data == "start_transaction":
		b.startAddTransaction(chatID)

	case data == "manage_categories":
		b.showCategoryManagement(chatID, svc)

	case data == "settings_back":
		b.showSettingsMenu(chatID)

	case strings.HasPrefix(data, CallbackEditCategory):
		catID, _ := strconv.Atoi(data[len(CallbackEditCategory):])
		b.showCategoryActions(chatID, catID, svc)

	case data == "add_to_saving":
		b.startAddToSaving(chatID, svc)

	case data == "savings_stats":
		b.showSavingsStats(chatID, svc)

	case data == "show_savings":
		b.showSavings(chatID, svc)

	case data == "main_menu":
		b.sendMainMenu(chatID, "Главное меню")

	case strings.HasPrefix(data, CallbackRenameCategory):
		catID, _ := strconv.Atoi(data[len(CallbackRenameCategory):])
		state := userStates[chatID]
		state.Step = "rename_category"
		state.TempCategoryID = catID
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "✏️ Введите новое название категории:"))

	case data == "skip_comment":
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

	case data == "skip_saving_goal":
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

	case strings.HasPrefix(data, CallbackDeleteCategory):
		catID, _ := strconv.Atoi(data[len(CallbackDeleteCategory):])
		b.handleDeleteCategory(chatID, catID, q.Message.MessageID, svc)

	case data == "type_income" || data == "type_expense":
		b.handleTypeSelect(chatID, q.Message.MessageID, data, svc)

	case strings.HasPrefix(data, "add_to_saving_"):
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

	case strings.HasPrefix(data, "cat_"):
		catID, err := strconv.Atoi(data[4:])
		if err != nil {
			b.sendError(chatID, fmt.Errorf("ошибка обработки ID категории"))
			return
		}
		b.handleCatSelect(int(chatID), catID)

	case data == "notification_settings":
		b.showNotificationSettings(chatID)

	case data == "enable_notifications":
		b.handleToggleNotifications(chatID, true, q.From)

	case data == "disable_notifications":
		b.handleToggleNotifications(chatID, false, q.From)

	case data == "other_cat":
		state := userStates[chatID]
		state.Step = "new_cat"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "📝 Введите название новой категории:"))

	case data == "create_saving":
		state := userStates[chatID]
		state.Step = "create_saving_name"
		userStates[chatID] = state
		b.send(chatID, tgbotapi.NewMessage(chatID, "💸 Введите название копилки:"))

	case data == "confirm_clear_data":
		msg := tgbotapi.NewMessage(chatID, "⚠️ <b>Внимание!</b>\n\nВы действительно хотите удалить ВСЕ свои данные? Это действие нельзя отменить!\n\nВсе транзакции, категории и копилки будут удалены.")
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить все", "clear_data"),
				tgbotapi.NewInlineKeyboardButtonData("❌ Нет, отменить", "settings_back"),
			),
		)
		b.send(chatID, msg)

	case data == "clear_data":
		err := svc.ClearUserData()
		if err != nil {
			logger.LogError(chatID, fmt.Sprintf("Ошибка очистки данных: %v", err))
			b.sendError(chatID, err)
			return
		}

		b.initBasicCategories(user)

		b.send(chatID, tgbotapi.NewMessage(chatID, "🧹 Все данные успешно удалены! Бот сброшен к начальному состоянию."))
		b.sendMainMenu(chatID, "🔄 Вы можете начать заново!")

	default:
		b.bot.Send(tgbotapi.NewCallback(q.ID, ""))
	}
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

			msgText.WriteString(fmt.Sprintf(
				"🔹 *%s*\n"+
					"┣ Накоплено: *%.2f ₽*\n"+
					"┣ Цель: *%.2f ₽*\n"+
					"┗ Прогресс: %s\n\n",
				s.Name, s.Amount, *s.Goal, progress,
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
	default:
		b.sendMainMenu(m.Chat.ID, "🤔 Неизвестная команда")
	}
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

func (b *Bot) showReport(chatID int64, svc *service.FinanceService) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)

	trans, err := svc.GetTransactionsForPeriod(start, end)
	if err != nil {
		b.sendError(chatID, fmt.Errorf("не удалось получить статистику"))
		return
	}

	var totalIncome, totalExpense float64
	incomeDetails := make(map[string]float64)
	expenseDetails := make(map[string]float64)

	for _, t := range trans {
		c, err := svc.GetCategoryByID(t.CategoryID)
		categoryName := "Неизвестно"
		if err == nil {
			categoryName = c.Name
		}

		if t.Amount > 0 {
			totalIncome += t.Amount
			incomeDetails[categoryName] += t.Amount
		} else {
			totalExpense += math.Abs(t.Amount)
			expenseDetails[categoryName] += math.Abs(t.Amount)
		}
	}

	format := func(amount float64) string {
		return fmt.Sprintf("%.2f ₽", amount)
	}

	var incomeDetailsStr strings.Builder
	for name, amount := range incomeDetails {
		incomeDetailsStr.WriteString(fmt.Sprintf("┣  %s: %s\n", name, format(amount)))
	}

	var expenseDetailsStr strings.Builder
	for name, amount := range expenseDetails {
		expenseDetailsStr.WriteString(fmt.Sprintf("┣  %s: %s\n", name, format(amount)))
	}

	balance := totalIncome - totalExpense

	msgText := fmt.Sprintf(
		"📊 <b>Финансовая статистика</b>\n📅 Период: <i>%s</i>\n\n"+
			"📈 <b>Доходы:</b> %s\n%s\n"+
			"📉 <b>Расходы:</b> %s\n%s\n"+
			"━━━━━━━━━━━━━━━\n"+
			"💸 <b>Баланс:</b> <u>%s</u>",
		start.Format("January 2006"),
		format(totalIncome),
		incomeDetailsStr.String(),
		format(totalExpense),
		expenseDetailsStr.String(),
		format(balance),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
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

func (b *Bot) SendReminder(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, `🔔 <b>Напоминание</b>

Привет! Сегодня ты не добавлял(а) транзакции. 

💡 Веди учет, чтобы лучше управлять финансами! 

➕ Нажми «Добавить операцию» или напиши сумму и комментарий, например:
<code>150 такси</code>`)
	msg.ParseMode = "HTML"
	b.send(chatID, msg)
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
			if s.Goal != nil {
				progress = b.renderProgressBar(s.Progress(), 10)
			}

			msgText.WriteString(fmt.Sprintf(
				"🔹 *%s*\n"+
					"┣ Накоплено: *%.2f ₽*\n",
				s.Name, s.Amount,
			))

			if s.Goal != nil {
				msgText.WriteString(fmt.Sprintf(
					"┣ Цель: *%.2f ₽*\n"+
						"┗ Прогресс: %s\n\n",
					*s.Goal, progress,
				))
			} else {
				msgText.WriteString("\n")
			}
		}
	}

	msg := tgbotapi.NewMessage(chatID, msgText.String())
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = b.createSavingsKeyboard()
	b.send(chatID, msg)
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

func (b *Bot) sendError(chatID int64, err error) {
	b.send(chatID, tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ Ошибка: %s", err.Error())))
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
