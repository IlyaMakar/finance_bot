package handlers

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const currentVersion = "1.1.0"

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
		"1.1.0": `🚀 Горячее обновление! Версия 1.1.1 🚀

✨ *Что нового в этой версии:*
- 📂  *Выгрузка статистики в таблицу!* Теперь можно детально анализировать финансы — просто экспортируйте данные в удобном формате. 
- 🎨 *Красивый редизайн кнопок!* Больше никаких сокращений — весь текст теперь виден полностью.
- 📅 *Гибкий выбор даты!* Укажите любой промежуток времени для анализа — статистика подстроится под ваш запрос.
- 💱 *Выбор валюты!* Теперь можно просматривать баланс и операции в предпочитаемой валюте.
- ❓*FAQ (Часто задаваемые вопросы)* Если не знаешь что происходит перейди в "Настройки" -> "Поддержка" -> "FAQ"
🔧 Также улучшено:
- Оптимизирована работа бота — стал ещё быстрее и стабильнее.
- Мелкие исправления и улучшения интерфейс

💡 *Совет от бота:* Используйте выгрузку таблиц, чтобы глубже анализировать свои финансы и принимать взвешенные решения! 💰`,
	}

	if desc, ok := descriptions[version]; ok {
		return desc
	}
	return "🎉 Обновление бота! Откройте новые функции и станьте ближе к финансовой свободе! 🚀"
}
