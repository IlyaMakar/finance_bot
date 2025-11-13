package handlers

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const currentVersion = "1.2.3"

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
		"1.2.3": `❗ Важно! Версия 1.2.3 ❗

💙 *Спасибо, что вы с нами! И извините за неудобства.*

Нам очень важно ваше мнение, и мы были очень расстроены, что после анонса система обратной связи временно работала с ошибками.

Мы хотели, чтобы ваш опыт был безупречным, но, к сожалению, подвели технические моменты. Проблема решена, и мы с удвоенным вниманием ждём ваших отзывов.

🎯 *Напомним, как оставить отзыв:*
Зайдите в ⚙️ Настройки → 📝 Обратная связь. Или нажмите сюда -> /feedback

Ваши идеи — это топливо для нашего развития. Спасибо за понимание!`,
	}

	if desc, ok := descriptions[version]; ok {
		return desc
	}
	return "🎉 Обновление бота! Откройте новые функции и станьте ближе к финансовой свободе! 🚀"
}
