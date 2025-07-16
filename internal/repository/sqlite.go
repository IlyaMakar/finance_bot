package repository

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteRepository struct {
	db *sql.DB
}

type User struct {
	ID                   int
	TelegramID           int64
	Username             string
	FirstName            string
	LastName             string
	CreatedAt            time.Time
	NotificationsEnabled bool
}

type Category struct {
	ID       int
	UserID   int
	Name     string
	Type     string
	ParentID *int
}

type Transaction struct {
	ID            int
	UserID        int
	Amount        float64
	CategoryID    int
	Date          time.Time
	PaymentMethod string
	Comment       string
}

type Saving struct {
	ID      int
	UserID  int
	Name    string
	Amount  float64
	Goal    *float64
	Comment string
}

func NewSQLiteDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open DB: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping DB: %w", err)
	}
	return db, nil
}

func InitDB(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        telegram_id INTEGER NOT NULL UNIQUE,
        username TEXT,
        first_name TEXT,
        last_name TEXT,
        created_at TEXT NOT NULL,
        notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE
    );

CREATE TABLE IF NOT EXISTS categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	type TEXT NOT NULL CHECK(type IN ('income','expense','saving')),
	parent_id INTEGER,
	FOREIGN KEY(parent_id) REFERENCES categories(id),
	FOREIGN KEY(user_id) REFERENCES users(id),
	UNIQUE(user_id, name)
);

CREATE TABLE IF NOT EXISTS transactions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	amount REAL NOT NULL,
	category_id INTEGER NOT NULL,
	date TEXT NOT NULL,
	payment_method TEXT CHECK(payment_method IN ('cash','card')),
	comment TEXT,
	FOREIGN KEY(category_id) REFERENCES categories(id),
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS savings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	amount REAL NOT NULL DEFAULT 0,
	goal REAL,
	comment TEXT,
	FOREIGN KEY(user_id) REFERENCES users(id),
	UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_id);
CREATE INDEX IF NOT EXISTS idx_transactions_user ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_categories_user ON categories(user_id);
CREATE INDEX IF NOT EXISTS idx_savings_user ON savings(user_id);
`
	_, err := db.Exec(schema)
	return err
}

func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) UpdateUserNotifications(userID int, enabled bool) error {
	_, err := r.db.Exec(
		"UPDATE users SET notifications_enabled = ? WHERE id = ?",
		enabled, userID,
	)
	return err
}

func (r *SQLiteRepository) GetUserNotificationsEnabled(userID int) (bool, error) {
	var enabled bool
	err := r.db.QueryRow(
		"SELECT notifications_enabled FROM users WHERE id = ?",
		userID,
	).Scan(&enabled)
	return enabled, err
}
func (r *SQLiteRepository) HasTransactionsToday(userID int) (bool, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)

	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM transactions WHERE user_id = ? AND date >= ? AND date < ?",
		userID, start.Format(time.RFC3339), end.Format(time.RFC3339),
	).Scan(&count)

	return count > 0, err
}

func (r *SQLiteRepository) GetAllUsers() ([]User, error) {
	rows, err := r.db.Query("SELECT id, telegram_id FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TelegramID); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *SQLiteRepository) GetOrCreateUser(telegramID int64, username, firstName, lastName string) (*User, error) {
	var user User
	var createdAt string

	err := r.db.QueryRow(
		"SELECT id, telegram_id, username, first_name, last_name, created_at FROM users WHERE telegram_id = ?",
		telegramID,
	).Scan(&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName, &createdAt)

	if err == nil {
		user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		return &user, nil
	}

	if err == sql.ErrNoRows {
		res, err := r.db.Exec(
			"INSERT INTO users (telegram_id, username, first_name, last_name, created_at) VALUES (?, ?, ?, ?, ?)",
			telegramID, username, firstName, lastName, time.Now().Format(time.RFC3339),
		)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}

		id, _ := res.LastInsertId()
		return &User{
			ID:         int(id),
			TelegramID: telegramID,
			Username:   username,
			FirstName:  firstName,
			LastName:   lastName,
			CreatedAt:  time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("get user: %w", err)
}

func (r *SQLiteRepository) GetCategories(userID int) ([]Category, error) {
	rows, err := r.db.Query(
		"SELECT id, name, type, parent_id FROM categories WHERE user_id = ? ORDER BY name",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var c Category
		var pid *int
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &pid); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		c.ParentID = pid
		c.UserID = userID
		cats = append(cats, c)
	}
	return cats, nil
}

func (r *SQLiteRepository) GetCategoryByID(userID, categoryID int) (*Category, error) {
	var c Category
	var pid *int
	row := r.db.QueryRow(
		"SELECT id, name, type, parent_id FROM categories WHERE id = ? AND user_id = ?",
		categoryID, userID,
	)
	if err := row.Scan(&c.ID, &c.Name, &c.Type, &pid); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("категория не найдена")
		}
		return nil, fmt.Errorf("scan category: %w", err)
	}
	c.ParentID = pid
	c.UserID = userID
	return &c, nil
}

func (r *SQLiteRepository) CreateCategory(userID int, c Category) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO categories(user_id, name, type, parent_id) VALUES(?, ?, ?, ?)",
		userID, c.Name, c.Type, c.ParentID,
	)
	if err != nil {
		return 0, fmt.Errorf("create category: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (r *SQLiteRepository) RenameCategory(userID, id int, newName string) error {
	if _, err := r.GetCategoryByID(userID, id); err != nil {
		return err
	}

	_, err := r.db.Exec(
		"UPDATE categories SET name = ? WHERE id = ? AND user_id = ?",
		newName, id, userID,
	)
	return err
}

func (r *SQLiteRepository) DeleteCategory(userID, id int) error {
	if _, err := r.GetCategoryByID(userID, id); err != nil {
		return err
	}

	_, err := r.db.Exec(
		"DELETE FROM categories WHERE id = ? AND user_id = ?",
		id, userID,
	)
	return err
}

func (r *SQLiteRepository) AddTransaction(userID int, t Transaction) (int, error) {
	if _, err := r.GetCategoryByID(userID, t.CategoryID); err != nil {
		return 0, err
	}

	res, err := r.db.Exec(
		"INSERT INTO transactions(user_id, amount, category_id, date, payment_method, comment) VALUES(?, ?, ?, ?, ?, ?)",
		userID, t.Amount, t.CategoryID, t.Date.Format(time.RFC3339), t.PaymentMethod, t.Comment,
	)
	if err != nil {
		return 0, fmt.Errorf("insert trans: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (r *SQLiteRepository) GetTransactionsByPeriod(userID int, start, end time.Time) ([]Transaction, error) {
	rows, err := r.db.Query(
		"SELECT id, amount, category_id, date, payment_method, comment FROM transactions WHERE user_id = ? AND date >= ? AND date < ? ORDER BY date DESC",
		userID, start.Format(time.RFC3339), end.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query trans: %w", err)
	}
	defer rows.Close()

	var res []Transaction
	for rows.Next() {
		var t Transaction
		var ds string
		if err := rows.Scan(&t.ID, &t.Amount, &t.CategoryID, &ds, &t.PaymentMethod, &t.Comment); err != nil {
			return nil, fmt.Errorf("scan trans: %w", err)
		}
		t.Date, _ = time.Parse(time.RFC3339, ds)
		t.UserID = userID
		res = append(res, t)
	}
	return res, nil
}

func (r *SQLiteRepository) GetSavings(userID int) ([]Saving, error) {
	rows, err := r.db.Query(
		"SELECT id, name, amount, goal, comment FROM savings WHERE user_id = ? ORDER BY name",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get savings: %w", err)
	}
	defer rows.Close()

	var list []Saving
	for rows.Next() {
		var s Saving
		var goal sql.NullFloat64
		var comment sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.Amount, &goal, &comment); err != nil {
			return nil, fmt.Errorf("scan saving: %w", err)
		}
		if goal.Valid {
			s.Goal = &goal.Float64
		}
		if comment.Valid {
			s.Comment = comment.String
		}
		s.UserID = userID
		list = append(list, s)
	}
	return list, nil
}

func (r *SQLiteRepository) GetSavingByID(userID, id int) (*Saving, error) {
	var s Saving
	var goal sql.NullFloat64
	var comment sql.NullString

	err := r.db.QueryRow(
		"SELECT id, name, amount, goal, comment FROM savings WHERE id = ? AND user_id = ?",
		id, userID,
	).Scan(&s.ID, &s.Name, &s.Amount, &goal, &comment)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	if goal.Valid {
		s.Goal = &goal.Float64
	}
	if comment.Valid {
		s.Comment = comment.String
	}
	s.UserID = userID

	return &s, nil
}

func (r *SQLiteRepository) UpdateSavingAmount(userID, id int, amount float64) error {
	if _, err := r.GetSavingByID(userID, id); err != nil {
		return err
	}

	_, err := r.db.Exec(
		"UPDATE savings SET amount = ? WHERE id = ? AND user_id = ?",
		amount, id, userID,
	)
	return err
}

func (r *SQLiteRepository) CreateSaving(userID int, name string, goal *float64) error {
	_, err := r.db.Exec(
		"INSERT INTO savings (user_id, name, amount, goal) VALUES (?, ?, 0, ?)",
		userID, name, goal,
	)
	return err
}

func (s *Saving) Progress() float64 {
	if s.Goal == nil || *s.Goal == 0 {
		return 0
	}
	return (s.Amount / *s.Goal) * 100
}

func (r *SQLiteRepository) ClearUserData(userID int) error {
	_, err := r.db.Exec("DELETE FROM transactions WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("ошибка удаления транзакций: %w", err)
	}

	_, err = r.db.Exec("DELETE FROM savings WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("ошибка удаления копилок: %w", err)
	}

	_, err = r.db.Exec("DELETE FROM categories WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("ошибка удаления категорий: %w", err)
	}

	_, err = r.db.Exec("UPDATE users SET notifications_enabled = TRUE WHERE id = ?", userID)
	if err != nil {
		return fmt.Errorf("ошибка сброса настроек: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) GetTransactionByID(userID, id int) (*Transaction, error) {
	var t Transaction
	var ds string

	err := r.db.QueryRow(
		"SELECT id, amount, category_id, date, comment FROM transactions WHERE id = ? AND user_id = ?",
		id, userID,
	).Scan(&t.ID, &t.Amount, &t.CategoryID, &ds, &t.Comment)

	if err != nil {
		return nil, err
	}

	t.Date, _ = time.Parse(time.RFC3339, ds)
	t.UserID = userID
	return &t, nil
}

func (r *SQLiteRepository) UpdateTransactionAmount(userID, id int, amount float64) error {
	_, err := r.db.Exec(
		"UPDATE transactions SET amount = ? WHERE id = ? AND user_id = ?",
		amount, id, userID,
	)
	return err
}

func (r *SQLiteRepository) UpdateTransactionComment(userID, id int, comment string) error {
	_, err := r.db.Exec(
		"UPDATE transactions SET comment = ? WHERE id = ? AND user_id = ?",
		comment, id, userID,
	)
	return err
}

func (r *SQLiteRepository) DeleteTransaction(userID, id int) error {
	_, err := r.db.Exec(
		"DELETE FROM transactions WHERE id = ? AND user_id = ?",
		id, userID,
	)
	return err
}
