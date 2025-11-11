package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "finance.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`
        SELECT uf.id, u.telegram_id, u.username, uf.what_likes, uf.what_missing, uf.what_annoying, uf.recommend, uf.created_at
        FROM user_feedback uf
        JOIN users u ON uf.user_id = u.id
        ORDER BY uf.created_at DESC
    `)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("📊 ОТЗЫВЫ ПОЛЬЗОВАТЕЛЕЙ")
	fmt.Println("=======================")

	count := 0
	for rows.Next() {
		var (
			id           int
			telegramID   int64
			username     sql.NullString
			whatLikes    sql.NullString
			whatMissing  sql.NullString
			whatAnnoying sql.NullString
			recommend    sql.NullString
			createdAt    string
		)

		err := rows.Scan(&id, &telegramID, &username, &whatLikes, &whatMissing, &whatAnnoying, &recommend, &createdAt)
		if err != nil {
			log.Fatal(err)
		}

		count++
		fmt.Printf("\n📝 Отзыв #%d\n", count)
		fmt.Printf("👤 Пользователь: %s (ID: %d)\n", getString(username), telegramID)
		fmt.Printf("📅 Дата: %s\n", createdAt)
		fmt.Printf("✅ Что нравится: %s\n", getString(whatLikes))
		fmt.Printf("❌ Чего не хватает: %s\n", getString(whatMissing))
		fmt.Printf("😠 Что раздражает: %s\n", getString(whatAnnoying))
		fmt.Printf("⭐ Рекомендация: %s\n", getRecommendation(recommend))
		fmt.Println("────────────────────")
	}

	fmt.Printf("\nВсего отзывов: %d\n", count)
}

func getString(s sql.NullString) string {
	if s.Valid && s.String != "" {
		return s.String
	}
	return "(не указано)"
}

func getRecommendation(s sql.NullString) string {
	if !s.Valid {
		return "(не указано)"
	}
	if s.String == "yes" {
		return "✅ Да"
	}
	return "❌ Нет"
}
