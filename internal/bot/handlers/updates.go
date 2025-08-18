package handlers

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const currentVersion = "1.0.0"

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
		"1.0.0": `🚀 Горячее обновление! 🚀

✨ *Что нового в версии 1.0.0:*
- 📊 *Супер-статистика!* Теперь просмотр финансов за любой период стал ещё удобнее и нагляднее!
- 📜 *История транзакций!* Погружайтесь в детали своих операций с новой функцией просмотра истории.
- ⏰ *Точное время!* Исправили ошибку с отправкой сообщений — теперь всё работает как часы!
- 🆘 *Техподдержка!* Если возникли какие-то проблемы, то просто нажми на кнопку и задай вопрос
- 💰 *Копилки на высоте!* Управление копилками стало проще:
  - 🗑️ Удаляйте копилки одним движением.
  - ✏️ Редактируйте их с лёгкостью.
  - 🧹 Очищайте данные, когда захотите!
- 🔔 *Будьте в курсе!* Теперь вы получите яркое уведомление о каждом обновлении бота.

🚀 *Совет от бота:* Ведите учет доходов и расходов, чтобы ваши финансы всегда были под контролем! 💸`,
	}

	if desc, ok := descriptions[version]; ok {
		return desc
	}
	return "🎉 Обновление бота! Откройте новые функции и станьте ближе к финансовой свободе! 🚀"
}
