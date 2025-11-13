package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/logger"
	"github.com/IlyaMakar/finance_bot/internal/repository"
)

type StatsAPI struct {
	repo *repository.SQLiteRepository
}

func NewStatsAPI(repo *repository.SQLiteRepository) *StatsAPI {
	return &StatsAPI{repo: repo}
}

type StatsResponse struct {
	TotalUsers    int            `json:"total_users"`
	ActiveToday   int            `json:"active_today"`
	ActiveWeek    int            `json:"active_week"`
	ActiveMonth   int            `json:"active_month"`
	ButtonClicks  map[string]int `json:"button_clicks"`
	AllUsers      []UserStats    `json:"all_users"`
	FeedbackStats FeedbackStats  `json:"feedback_stats"`
	AllFeedbacks  []Feedback     `json:"all_feedbacks"`
}

type UserStats struct {
	TelegramID int64     `json:"telegram_id"`
	Username   string    `json:"username"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	LastActive time.Time `json:"last_active"`
	JoinDate   time.Time `json:"join_date"`
}

type FeedbackStats struct {
	Total        int     `json:"total"`
	RecommendYes int     `json:"recommend_yes"`
	RecommendNo  int     `json:"recommend_no"`
	YesPercent   float64 `json:"yes_percent"`
	NoPercent    float64 `json:"no_percent"`
}

type Feedback struct {
	ID           int       `json:"id"`
	TelegramID   int64     `json:"telegram_id"`
	Username     string    `json:"username"`
	WhatLikes    string    `json:"what_likes"`
	WhatMissing  string    `json:"what_missing"`
	WhatAnnoying string    `json:"what_annoying"`
	Recommend    string    `json:"recommend"`
	CreatedAt    time.Time `json:"created_at"`
}

func translateButtonName(buttonName string) string {
	translations := map[string]string{

		"start_transaction": "💸 Добавить операцию",
		"show_stats":        "📊 Статистика",
		"show_savings":      "💰 Накопления",
		"show_settings":     "⚙️ Настройки",

		"stats_day":    "📅 День",
		"stats_week":   "📆 Неделя",
		"stats_month":  "📈 Месяц",
		"stats_year":   "🎯 Год",
		"stats_back":   "◀️ Назад",
		"show_history": "📜 История операций",

		"create_saving":  "➕ Новая копилка",
		"add_to_saving":  "💰 Пополнить",
		"savings_stats":  "📊 Статистика",
		"manage_savings": "✏️ Редактировать",

		"notification_settings": "🔔 Уведомления",
		"manage_categories":     "📝 Категории",
		"settings_back":         "◀️ В меню",
		"enable_notifications":  "🔔 Включить",
		"disable_notifications": "🔕 Отключить",
		"confirm_clear_data":    "🧹 Очистить все данные",
		"clear_data":            "✅ Да, удалить все",

		"other_cat": "✨ Новая категория",
		"cancel":    "◀️ Отмена",

		"type_income":  "📈 Доход",
		"type_expense": "📉 Расход",

		"skip_comment":     "Пропустить",
		"skip_saving_goal": "Пропустить",
		"main_menu":        "🏠 Главное меню",
		"support":          "🆘 Поддержка",

		"edit_amount":        "✏️ Сумма",
		"edit_category":      "📂 Категория",
		"edit_comment":       "💬 Комментарий",
		"delete_transaction": "🗑️ Удалить",

		"currency_settings": "💱 Валюта",
		"set_currency_RUB":  "🇷🇺 RUB (Рубли)",
		"set_currency_USD":  "🇺🇸 USD (Доллары)",
		"set_currency_EUR":  "🇪🇺 EUR (Евро)",

		"set_period_start": "📅 Период отчётов",

		"write_support":          "✉️ Написать разработчику",
		"faq":                    "❓ FAQ",
		"feedback":               "📝 Обратная связь",
		"feedback_submit":        "✅ Отправить отзыв",
		"feedback_cancel":        "🚫 Отмена",
		"feedback_recommend_yes": "✅ Да",
		"feedback_recommend_no":  "❌ Нет",

		"rename_cat_": "✏️ Переименовать",
		"delete_cat_": "🗑️ Удалить",
		"edit_cat_":   "✏️ Редактировать",

		"edit_saving_":   "✏️ Редактировать",
		"delete_saving_": "🗑️ Удалить",
		"rename_saving_": "✏️ Переименовать",
		"clear_saving_":  "🧹 Очистить",

		"add_to_saving_":   "➕ Пополнить",
		"saving_add_":      "➕ Пополнить",
		"saving_withdraw_": "➖ Снять",
		"saving_rename_":   "✏️ Переименовать",
		"saving_delete_":   "🗑️ Удалить",

		"cat_": "📂 Категория: ",

		"edit_":           "✏️ Редактировать: ",
		"change_category": "📂 Сменить категорию: ",
	}

	if translated, exists := translations[buttonName]; exists {
		return translated
	}

	for prefix, translation := range translations {
		if len(buttonName) > len(prefix) && buttonName[:len(prefix)] == prefix {

			if prefix == "cat_" || prefix == "edit_" || prefix == "change_category_" {
				return translation
			}

			return translation + buttonName[len(prefix):]
		}
	}

	return buttonName
}

func (s *StatsAPI) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := StatsResponse{
		ButtonClicks:  make(map[string]int),
		AllUsers:      []UserStats{},
		FeedbackStats: FeedbackStats{},
		AllFeedbacks:  []Feedback{},
	}

	defer func() {
		if err := recover(); err != nil {
			logger.Error("Panic in GetStats", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	users, err := s.repo.GetAllUsers()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting users: %v", err), http.StatusInternalServerError)
		return
	}
	stats.TotalUsers = len(users)

	today := time.Now().Add(-24 * time.Hour)
	activeToday, err := s.repo.GetActiveUsersCount(today)
	if err == nil {
		stats.ActiveToday = activeToday
	}

	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	activeWeek, err := s.repo.GetActiveUsersCount(weekAgo)
	if err == nil {
		stats.ActiveWeek = activeWeek
	}

	monthAgo := time.Now().Add(-30 * 24 * time.Hour)
	activeMonth, err := s.repo.GetActiveUsersCount(monthAgo)
	if err == nil {
		stats.ActiveMonth = activeMonth
	}

	buttonClicks, err := s.repo.GetButtonClicksCount(weekAgo)
	if err == nil {
		translatedButtonClicks := make(map[string]int)
		for buttonName, count := range buttonClicks {
			translatedName := translateButtonName(buttonName)
			translatedButtonClicks[translatedName] = count
		}
		stats.ButtonClicks = translatedButtonClicks
	}

	stats.AllUsers = s.getAllUsers(users)

	feedbackStats, err := s.repo.GetFeedbackStats()
	if err == nil && feedbackStats != nil {
		// Безопасное извлечение данных с проверкой типов
		if total, ok := feedbackStats["total_feedbacks"].(int); ok {
			stats.FeedbackStats.Total = total
		}
		if yes, ok := feedbackStats["recommend_yes"].(int); ok {
			stats.FeedbackStats.RecommendYes = yes
		}
		if no, ok := feedbackStats["recommend_no"].(int); ok {
			stats.FeedbackStats.RecommendNo = no
		}
		if yesPercent, ok := feedbackStats["recommend_yes_percent"].(float64); ok {
			stats.FeedbackStats.YesPercent = yesPercent
		}
		if noPercent, ok := feedbackStats["recommend_no_percent"].(float64); ok {
			stats.FeedbackStats.NoPercent = noPercent
		}
	}

	feedbacks, err := s.repo.GetAllFeedback()
	if err == nil {
		stats.AllFeedbacks = s.getAllFeedbacks(feedbacks)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *StatsAPI) getAllUsers(users []repository.User) []UserStats {
	var allUsers []UserStats
	for _, user := range users {
		lastActive, err := s.repo.GetUserActivity(user.ID)
		if err != nil {
			lastActive = user.CreatedAt
		}

		if lastActive.IsZero() {
			lastActive = user.CreatedAt
		}

		allUsers = append(allUsers, UserStats{
			TelegramID: user.TelegramID,
			Username:   user.Username,
			FirstName:  user.FirstName,
			LastName:   user.LastName,
			LastActive: lastActive,
			JoinDate:   user.CreatedAt,
		})
	}
	return allUsers
}

func (s *StatsAPI) getAllFeedbacks(feedbacks []map[string]interface{}) []Feedback {
	var allFeedbacks []Feedback
	for _, fb := range feedbacks {
		id, _ := fb["id"].(int)
		telegramID, _ := fb["telegram_id"].(int64)
		username, _ := fb["username"].(string)
		whatLikes, _ := fb["what_likes"].(string)
		whatMissing, _ := fb["what_missing"].(string)
		whatAnnoying, _ := fb["what_annoying"].(string)
		recommend, _ := fb["recommend"].(string)

		createdAtStr, _ := fb["created_at"].(string)
		createdAt, _ := time.Parse(time.RFC3339, createdAtStr)

		allFeedbacks = append(allFeedbacks, Feedback{
			ID:           id,
			TelegramID:   telegramID,
			Username:     username,
			WhatLikes:    whatLikes,
			WhatMissing:  whatMissing,
			WhatAnnoying: whatAnnoying,
			Recommend:    recommend,
			CreatedAt:    createdAt,
		})
	}
	return allFeedbacks
}
