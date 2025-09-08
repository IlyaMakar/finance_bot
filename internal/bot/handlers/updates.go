package handlers

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const currentVersion = "1.2.0"

func (b *Bot) CheckForUpdates() {
	latestVersion, err := b.repo.GetLatestVersion()
	if err != nil {
		svc := service.NewService(b.repo, &repository.User{ID: 0})
		svc.AddVersion(currentVersion, getVersionDescription(currentVersion))
		return
	}

	if latestVersion == nil || latestVersion.Version != currentVersion {
		svc := service.NewService(b.repo, &repository.User{ID: 0})
		description := getVersionDescription(currentVersion)
		svc.AddVersion(currentVersion, description)
	}
}

func (b *Bot) NotifyUsersAboutUpdate() {
	latestVersion, err := b.repo.GetLatestVersion()
	if err != nil || latestVersion == nil {
		return
	}

	users, err := b.repo.GetAllUsers()
	if err != nil {
		return
	}

	for _, user := range users {
		svc := service.NewService(b.repo, &user)

		hasRead, err := svc.HasUserReadVersion(latestVersion.ID)
		if err != nil || hasRead {
			continue
		}

		msg := tgbotapi.NewMessage(
			user.TelegramID,
			fmt.Sprintf("🎉 *Обновление бота до v%s!*\n\n%s\n\n_Спасибо, что используете нашего бота!_",
				latestVersion.Version,
				latestVersion.Description),
		)
		msg.ParseMode = "Markdown"
		b.SendMessage(msg)

		svc.MarkVersionAsRead(latestVersion.ID)

		time.Sleep(100 * time.Millisecond)
	}
}

func getVersionDescription(version string) string {
	descriptions := map[string]string{
		"1.2.0": `🚀 Горячее обновление! Версия 1.2.0 🚀

✨ *Что нового в этой версии:*
- 🎛️  *Новый интерфейс!* ТПолностью убрали нижнюю клавиатуру. Управление ботом стало еще удобнее и интуитивнее — все взаимодействие теперь происходит через аккуратные всплывающие меню. 
- 💬 *Умные сообщения!* Реализовано исчезновение старых сообщений при новом взаимодействии. Лента переписки теперь чистая и аккуратная, ничто не отвлекает от анализа финансов.
- 📅 *Еще более гибкий выбор дат!* Переработали и значительно улучшили механизм изменения периода для статистики. Теперь устанавливать нужные даты стало проще и быстрее.
- 🐖 *Исправление копилок!* Решена проблема с редактированием копилок. Теперь ваши цели накопления можно легко изменять без ошибок..
- ⚡ *Повышена производительность!* Провели глубокую оптимизацию кода. Бот стал заметно шустрее и стабильнее, чтобы моментально реагировать на ваши запросы.

🔧  А также:
- Мелкие исправления интерфейса для еще большего комфорта. 

💡 *Совет от бота:* Используйте обновленный интерфейс, чтобы управлять финансами еще эффективнее! Начните с команды /start`,
	}

	if desc, ok := descriptions[version]; ok {
		return desc
	}
	return "🎉 Обновление бота! Откройте новые функции и станьте ближе к финансовой свободе! 🚀"
}
