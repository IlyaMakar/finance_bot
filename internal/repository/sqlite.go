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

type Category struct {
	ID       int
	Name     string
	Type     string
	ParentID *int
}

type Transaction struct {
	ID            int
	Amount        float64
	CategoryID    int
	Date          time.Time
	PaymentMethod string
	Comment       string
}

type Saving struct {
	ID      int
	Name    string
	Amount  float64
	Goal    *float64
	Comment string
}

// NewSQLiteDB создает новое подключение к базе данных SQLite
func NewSQLiteDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// InitDB инициализирует структуру базы данных
func InitDB(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL CHECK(type IN ('income', 'expense', 'saving')),
		parent_id INTEGER,
		FOREIGN KEY(parent_id) REFERENCES categories(id)
	);
	
	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		amount REAL NOT NULL,
		category_id INTEGER NOT NULL,
		date TEXT NOT NULL,
		payment_method TEXT CHECK(payment_method IN ('cash', 'card')),
		comment TEXT,
		FOREIGN KEY(category_id) REFERENCES categories(id)
	);
	
	CREATE TABLE IF NOT EXISTS savings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		amount REAL NOT NULL DEFAULT 0,
		goal REAL,
		comment TEXT
	);

	CREATE TABLE IF NOT EXISTS savings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    amount REAL NOT NULL DEFAULT 0,
    comment TEXT
);
	
	CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
	CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_id);`

	_, err := db.Exec(query)
	return err
}

// NewRepository создает новый экземпляр репозитория
func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// Методы для работы с категориями
func (r *SQLiteRepository) CreateCategory(category Category) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO categories (name, type, parent_id) VALUES (?, ?, ?)",
		category.Name, category.Type, category.ParentID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create category: %w", err)
	}

	id, err := res.LastInsertId()
	return int(id), err
}

func (r *SQLiteRepository) GetCategories() ([]Category, error) {
	// Добавляем DISTINCT для исключения дубликатов
	rows, err := r.db.Query("SELECT DISTINCT id, name, type, parent_id FROM categories ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer rows.Close()

	var categories []Category
	existingNames := make(map[string]bool) // Для проверки уникальности

	for rows.Next() {
		var c Category
		var parentID *int
		err := rows.Scan(&c.ID, &c.Name, &c.Type, &parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		// Проверяем, не встречалось ли уже такое имя
		if !existingNames[c.Name] {
			c.ParentID = parentID
			categories = append(categories, c)
			existingNames[c.Name] = true
		}
	}

	return categories, nil
}

func (r *SQLiteRepository) GetCategoryByID(id int) (*Category, error) {
	var c Category
	var parentID *int

	err := r.db.QueryRow(
		"SELECT id, name, type, parent_id FROM categories WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Name, &c.Type, &parentID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	c.ParentID = parentID
	return &c, nil
}

func (r *SQLiteRepository) CleanDuplicateCategories() error {
	// Создаем временную таблицу без дубликатов
	_, err := r.db.Exec(`
        CREATE TEMPORARY TABLE temp_categories AS 
        SELECT MIN(id) as id, name, type, parent_id 
        FROM categories 
        GROUP BY name, type, parent_id;
        
        DELETE FROM categories;
        
        INSERT INTO categories 
        SELECT * FROM temp_categories;
        
        DROP TABLE temp_categories;
    `)
	return err
}

// Методы для работы с транзакциями
func (r *SQLiteRepository) AddTransaction(t Transaction) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO transactions (amount, category_id, date, payment_method, comment) VALUES (?, ?, ?, ?, ?)",
		t.Amount, t.CategoryID, t.Date.Format(time.RFC3339), t.PaymentMethod, t.Comment,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add transaction: %w", err)
	}

	id, err := res.LastInsertId()
	return int(id), err
}

func (r *SQLiteRepository) GetTransactionsByPeriod(start, end time.Time) ([]Transaction, error) {
	rows, err := r.db.Query(
		`SELECT id, amount, category_id, date, payment_method, comment 
		 FROM transactions 
		 WHERE date BETWEEN ? AND ?
		 ORDER BY date DESC`,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.CategoryID, &dateStr, &t.PaymentMethod, &t.Comment)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		t.Date, err = time.Parse(time.RFC3339, dateStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse date: %w", err)
		}

		transactions = append(transactions, t)
	}

	return transactions, nil
}

// Методы для работы с накоплениями
// Изменяем метод AddSaving
func (r *SQLiteRepository) AddSaving(s Saving) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO savings (name, amount, comment) VALUES (?, ?, ?)",
		s.Name, s.Amount, s.Comment,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add saving: %w", err)
	}

	id, err := res.LastInsertId()
	return int(id), err
}

func (r *SQLiteRepository) UpdateSaving(s Saving) error {
	_, err := r.db.Exec(
		"UPDATE savings SET amount = ?, goal = ?, comment = ? WHERE id = ?",
		s.Amount, s.Goal, s.Comment, s.ID,
	)
	return err
}

func (r *SQLiteRepository) GetSavings() ([]Saving, error) {
	rows, err := r.db.Query("SELECT id, name, amount, goal, comment FROM savings ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to get savings: %w", err)
	}
	defer rows.Close()

	var savings []Saving
	for rows.Next() {
		var s Saving
		var goal *float64
		err := rows.Scan(&s.ID, &s.Name, &s.Amount, &goal, &s.Comment)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saving: %w", err)
		}
		s.Goal = goal
		savings = append(savings, s)
	}

	return savings, nil
}

func (r *SQLiteRepository) AddSavings(name, comment string) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO savings (name, amount, comment) VALUES (?, 0, ?)",
		name, comment,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add saving: %w", err)
	}

	id, err := res.LastInsertId()
	return int(id), err
}

// UpdateSavingAmount обновляет сумму в копилке
func (r *SQLiteRepository) UpdateSavingAmount(id int, amount float64) error {
	_, err := r.db.Exec(
		"UPDATE savings SET amount = ? WHERE id = ?",
		amount, id,
	)
	return err
}

// GetSavings возвращает список всех копилок
func (r *SQLiteRepository) GetSaving() ([]Saving, error) {
	rows, err := r.db.Query("SELECT id, name, amount, comment FROM savings ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to get savings: %w", err)
	}
	defer rows.Close()

	var savings []Saving
	for rows.Next() {
		var s Saving
		err := rows.Scan(&s.ID, &s.Name, &s.Amount, &s.Comment)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saving: %w", err)
		}
		savings = append(savings, s)
	}

	return savings, nil
}

// GetSavingByID возвращает копилку по ID
func (r *SQLiteRepository) GetSavingByID(id int) (*Saving, error) {
	var s Saving
	err := r.db.QueryRow(
		"SELECT id, name, amount, comment FROM savings WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.Amount, &s.Comment)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("saving not found")
		}
		return nil, fmt.Errorf("failed to get saving: %w", err)
	}

	return &s, nil
}
