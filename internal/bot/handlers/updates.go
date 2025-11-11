package handlers

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const currentVersion = "1.2.2"

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
		"1.2.2": `❗ Важно! Версия 1.2.2 ❗

✨ *Добавили обратную связь:*
- 📝 **Система обратной связи!** Добавлена возможность оставить отзыв о работе бота. Ваше мнение поможет нам стать лучше!
- ⚙️ **Новый раздел в настройках:** Теперь в настройках есть кнопка "Обратная связь", где вы можете рассказать о своих впечатлениях
- 🗳️ **Опрос пользователей:** Ответьте на 4 простых вопроса:
  1. Что нравится в боте?
  2. Чего не хватает?
  3. Что раздражает?
  4. Порекомендуете ли друзьям?

💡 *Почему это важно:*
Ваши отзывы - это бесценная информация, которая поможет нам:
- Исправить ошибки и неудобства
- Добавить нужные функции
- Сделать бота еще удобнее и полезнее

📊 *Как оставить отзыв:*
Зайдите в ⚙️ Настройки → 📝 Обратная связь /feedback

🎯 *Наша цель:* Сделать лучшего финансового помощника для вас!

*Пожалуйста, найдите минутку и поделитесь своим мнением - это действительно поможет улучшить бота для всех пользователей!* 

Спасибо за вашу поддержку! 💙`,
	}

	if desc, ok := descriptions[version]; ok {
		return desc
	}
	return "🎉 Обновление бота! Откройте новые функции и станьте ближе к финансовой свободе! 🚀"
}
