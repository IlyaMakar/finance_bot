package repository

import (
	"database/sql"
	"fmt"
	"strings"
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
	PeriodStartDay       int
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
	CategoryName  string
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

type GlobalCategory struct {
	ID   int
	Name string
	Type string
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
    notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    period_start_day INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS global_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('income', 'expense', 'saving')),
    UNIQUE(name, type)
);

CREATE TABLE IF NOT EXISTS user_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    global_category_id INTEGER NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id),
    FOREIGN KEY(global_category_id) REFERENCES global_categories(id),
    UNIQUE(user_id, global_category_id)
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

CREATE TABLE IF NOT EXISTS user_currency_settings (
    user_id INTEGER PRIMARY KEY,
    currency TEXT NOT NULL DEFAULT 'RUB',
    FOREIGN KEY (user_id) REFERENCES users(id)
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

CREATE TABLE IF NOT EXISTS user_activity (
    user_id INTEGER PRIMARY KEY,
    last_active TEXT,
    join_date TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS button_clicks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    button_name TEXT,
    click_time TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_id);
CREATE INDEX IF NOT EXISTS idx_transactions_user ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_categories_user ON categories(user_id);
CREATE INDEX IF NOT EXISTS idx_savings_user ON savings(user_id);

CREATE TABLE IF NOT EXISTS versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version TEXT NOT NULL,
    release_date TEXT NOT NULL,
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_version_read (
    user_id INTEGER NOT NULL,
    version_id INTEGER NOT NULL,
    read_at TEXT NOT NULL,
    PRIMARY KEY (user_id, version_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (version_id) REFERENCES versions(id)
);
`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("ошибка создания схемы базы данных: %w", err)
	}

	var columnExists int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM pragma_table_info('users') 
		WHERE name = 'period_start_day'
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("ошибка проверки столбца period_start_day: %w", err)
	}

	if columnExists == 0 {
		_, err = db.Exec("ALTER TABLE users ADD COLUMN period_start_day INTEGER NOT NULL DEFAULT 1")
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("ошибка добавления столбца period_start_day: %w", err)
		}
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM global_categories").Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка проверки глобальных категорий: %w", err)
	}
	if count == 0 {
		initialCategories := []struct {
			name string
			typ  string
		}{
			{"🍎 Продукты", "expense"},
			{"🚗 Транспорт", "expense"},
			{"🏠 ЖКХ", "expense"},
			{"💼 Зарплата", "income"},
			{"🎉 Развлечения", "expense"},
		}
		for _, cat := range initialCategories {
			_, err = db.Exec("INSERT INTO global_categories (name, type) VALUES (?, ?)", cat.name, cat.typ)
			if err != nil {
				return fmt.Errorf("ошибка добавления начальной категории %s: %w", cat.name, err)
			}
		}
	}

	return nil
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
		"SELECT id, telegram_id, username, first_name, last_name, created_at, notifications_enabled, period_start_day FROM users WHERE telegram_id = ?",
		telegramID,
	).Scan(&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName, &createdAt, &user.NotificationsEnabled, &user.PeriodStartDay)

	if err == nil {
		user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		r.UpdateUserActivity(user.ID, time.Now())
		return &user, nil
	}

	if err == sql.ErrNoRows {
		res, err := r.db.Exec(
			"INSERT INTO users (telegram_id, username, first_name, last_name, created_at, notifications_enabled, period_start_day) VALUES (?, ?, ?, ?, ?, TRUE, 1)",
			telegramID, username, firstName, lastName, time.Now().Format(time.RFC3339),
		)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}

		id, _ := res.LastInsertId()
		user = User{
			ID:         int(id),
			TelegramID: telegramID,
			Username:   username,
			FirstName:  firstName,
			LastName:   lastName,
			CreatedAt:  time.Now(),
		}
		r.UpdateUserActivity(user.ID, time.Now())
		return &user, nil
	}

	return nil, fmt.Errorf("get user: %w", err)
}

func (r *SQLiteRepository) UpdateUserActivity(userID int, activeTime time.Time) error {
	var exists int
	err := r.db.QueryRow("SELECT COUNT(*) FROM user_activity WHERE user_id = ?", userID).Scan(&exists)
	if err != nil {
		return err
	}

	if exists == 0 {
		_, err = r.db.Exec(
			"INSERT INTO user_activity (user_id, last_active, join_date) VALUES (?, ?, ?)",
			userID, activeTime.Format(time.RFC3339), activeTime.Format(time.RFC3339),
		)
	} else {
		_, err = r.db.Exec(
			"UPDATE user_activity SET last_active = ? WHERE user_id = ?",
			activeTime.Format(time.RFC3339), userID,
		)
	}
	return err
}

func (r *SQLiteRepository) RecordButtonClick(userID int, buttonName string) error {
	_, err := r.db.Exec(
		"INSERT INTO button_clicks (user_id, button_name, click_time) VALUES (?, ?, ?)",
		userID, buttonName, time.Now().Format(time.RFC3339),
	)
	return err
}

func (r *SQLiteRepository) GetActiveUsersCount(since time.Time, count *int) error {
	query := "SELECT COUNT(*) FROM user_activity WHERE last_active >= ?"
	return r.db.QueryRow(query, since.Format(time.RFC3339)).Scan(count)
}

func (r *SQLiteRepository) GetActiveUsersCountForPeriod(start, end time.Time, count *int) error {
	query := "SELECT COUNT(*) FROM user_activity WHERE last_active >= ? AND last_active < ?"
	return r.db.QueryRow(query, start.Format(time.RFC3339), end.Format(time.RFC3339)).Scan(count)
}

func (r *SQLiteRepository) GetButtonClicksCount(since time.Time, counts *map[string]int) error {
	*counts = make(map[string]int)
	rows, err := r.db.Query("SELECT button_name, COUNT(*) FROM button_clicks WHERE click_time >= ? GROUP BY button_name", since.Format(time.RFC3339))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var buttonName string
		var count int
		if err := rows.Scan(&buttonName, &count); err != nil {
			return err
		}
		(*counts)[buttonName] = count
	}
	return rows.Err()
}

func (r *SQLiteRepository) GetButtonClicksCountForPeriod(start, end time.Time, counts *map[string]int) error {
	*counts = make(map[string]int)
	rows, err := r.db.Query("SELECT button_name, COUNT(*) FROM button_clicks WHERE click_time >= ? AND click_time < ? GROUP BY button_name", start.Format(time.RFC3339), end.Format(time.RFC3339))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var buttonName string
		var count int
		if err := rows.Scan(&buttonName, &count); err != nil {
			return err
		}
		(*counts)[buttonName] = count
	}
	return rows.Err()
}

func (r *SQLiteRepository) GetCategories(userID int) ([]Category, error) {
	rows, err := r.db.Query(
		"SELECT id, name, type, parent_id FROM categories WHERE user_id = ?",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		var pid *int
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &pid); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		c.ParentID = pid
		c.UserID = userID
		categories = append(categories, c)
	}

	if len(categories) == 0 {
		basicCats := []struct{ name, typ string }{
			{"🍎 Продукты", "expense"},
			{"🚗 Транспорт", "expense"},
			{"🏠 ЖКХ", "expense"},
			{"💼 Зарплата", "income"},
			{"🎉 Развлечения", "expense"},
		}

		for _, cat := range basicCats {
			newCat := Category{
				UserID: userID,
				Name:   cat.name,
				Type:   cat.typ,
			}
			if _, err := r.CreateCategory(userID, newCat); err != nil {
				return nil, err
			}
		}

		return r.GetCategories(userID)
	}

	return categories, nil
}

func (r *SQLiteRepository) GetCategoryByID(userID, id int) (*Category, error) {
	var c Category
	row := r.db.QueryRow(`
        SELECT id, user_id, name, type 
        FROM categories 
        WHERE id = ? AND user_id = ?`,
		id, userID)
	if err := row.Scan(&c.ID, &c.UserID, &c.Name, &c.Type); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка сканирования категории: %w", err)
	}
	return &c, nil
}

func (r *SQLiteRepository) CreateCategory(userID int, c Category) (int, error) {
	var globalID int
	err := r.db.QueryRow("SELECT id FROM global_categories WHERE name = ? AND type = ?", c.Name, c.Type).Scan(&globalID)
	if err != nil {
		if err == sql.ErrNoRows {
			res, err := r.db.Exec("INSERT INTO global_categories (name, type) VALUES (?, ?)", c.Name, c.Type)
			if err != nil {
				return 0, fmt.Errorf("create global category: %w", err)
			}
			id, _ := res.LastInsertId()
			globalID = int(id)
		} else {
			return 0, fmt.Errorf("check global category: %w", err)
		}
	}

	var exists int
	err = r.db.QueryRow("SELECT COUNT(*) FROM user_categories WHERE user_id = ? AND global_category_id = ?", userID, globalID).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if exists == 0 {
		_, err = r.db.Exec("INSERT INTO user_categories (user_id, global_category_id) VALUES (?, ?)", userID, globalID)
		if err != nil {
			return 0, fmt.Errorf("create user category: %w", err)
		}
	}

	res, err := r.db.Exec("INSERT INTO categories (user_id, name, type, parent_id) VALUES (?, ?, ?, ?)", userID, c.Name, c.Type, c.ParentID)
	if err != nil {
		return 0, fmt.Errorf("create category: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (r *SQLiteRepository) RenameCategory(userID, id int, newName string) error {
	var globalID int
	err := r.db.QueryRow(`
        SELECT gc.id 
        FROM user_categories uc
        JOIN global_categories gc ON uc.global_category_id = gc.id
        JOIN categories c ON c.user_id = uc.user_id AND c.name = gc.name
        WHERE c.id = ? AND uc.user_id = ?
    `, id, userID).Scan(&globalID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec("UPDATE global_categories SET name = ? WHERE id = ?", newName, globalID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec("UPDATE categories SET name = ? WHERE id = ? AND user_id = ?", newName, id, userID)
	return err
}

func (r *SQLiteRepository) DeleteCategory(userID, id int) error {
	var globalID int
	err := r.db.QueryRow(`
        SELECT gc.id 
        FROM user_categories uc
        JOIN global_categories gc ON uc.global_category_id = gc.id
        JOIN categories c ON c.user_id = uc.user_id AND c.name = gc.name
        WHERE c.id = ? AND uc.user_id = ?
    `, id, userID).Scan(&globalID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec("DELETE FROM user_categories WHERE user_id = ? AND global_category_id = ?", userID, globalID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec("DELETE FROM categories WHERE id = ? AND user_id = ?", id, userID)
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
	r.UpdateUserActivity(userID, time.Now())
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

func (r *SQLiteRepository) DeleteSaving(userID, id int) error {
	_, err := r.db.Exec(
		"DELETE FROM savings WHERE id = ? AND user_id = ?",
		id, userID,
	)
	return err
}

func (r *SQLiteRepository) RenameSaving(userID, id int, newName string) error {
	_, err := r.db.Exec(
		"UPDATE savings SET name = ? WHERE id = ? AND user_id = ?",
		newName, id, userID,
	)
	return err
}

func (r *SQLiteRepository) UpdateUserPeriodStartDay(userID int, day int) error {
	_, err := r.db.Exec("UPDATE users SET period_start_day = ? WHERE id = ?", day, userID)
	return err
}

type Version struct {
	ID          int
	Version     string
	ReleaseDate time.Time
	Description string
}

func (r *SQLiteRepository) AddVersion(version, description string) error {
	_, err := r.db.Exec(
		"INSERT INTO versions (version, release_date, description) VALUES (?, ?, ?)",
		version, time.Now().Format(time.RFC3339), description,
	)
	return err
}

func (r *SQLiteRepository) GetLatestVersion() (*Version, error) {
	var v Version
	var dateStr string

	err := r.db.QueryRow(
		"SELECT id, version, release_date, description FROM versions ORDER BY release_date DESC LIMIT 1",
	).Scan(&v.ID, &v.Version, &dateStr, &v.Description)

	if err != nil {
		return nil, err
	}

	v.ReleaseDate, _ = time.Parse(time.RFC3339, dateStr)
	return &v, nil
}

func (r *SQLiteRepository) MarkVersionAsRead(userID, versionID int) error {
	_, err := r.db.Exec(
		"INSERT INTO user_version_read (user_id, version_id, read_at) VALUES (?, ?, ?)",
		userID, versionID, time.Now().Format(time.RFC3339),
	)
	return err
}

func (r *SQLiteRepository) HasUserReadVersion(userID, versionID int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM user_version_read WHERE user_id = ? AND version_id = ?",
		userID, versionID,
	).Scan(&count)

	return count > 0, err
}

func (r *SQLiteRepository) SetUserCurrency(userID int, currency string) error {
	_, err := r.db.Exec(`
        INSERT OR REPLACE INTO user_currency_settings (user_id, currency) 
        VALUES (?, ?)`,
		userID, currency,
	)
	return err
}

func (r *SQLiteRepository) GetUserCurrency(userID int) (string, error) {
	var currency string
	err := r.db.QueryRow(`
        SELECT currency FROM user_currency_settings 
        WHERE user_id = ?`,
		userID,
	).Scan(&currency)

	if err == sql.ErrNoRows {
		return "RUB", nil
	}
	return currency, err
}
