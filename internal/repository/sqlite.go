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

func NewSQLiteDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("can't open database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("can't connect to database: %w", err)
	}

	return db, nil
}

func InitDB(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL CHECK(type IN ('income', 'expense')),
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
	);`

	_, err := db.Exec(query)
	return err
}

func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) CreateCategory(category Category) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO categories (name, type, parent_id) VALUES (?, ?, ?)",
		category.Name, category.Type, category.ParentID,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return int(id), err
}

func (r *SQLiteRepository) GetCategories() ([]Category, error) {
	rows, err := r.db.Query("SELECT id, name, type, parent_id FROM categories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		var parentID *int
		err := rows.Scan(&c.ID, &c.Name, &c.Type, &parentID)
		if err != nil {
			return nil, err
		}
		c.ParentID = parentID
		categories = append(categories, c)
	}

	return categories, nil
}

func (r *SQLiteRepository) GetCategoryByID(id int) (*Category, error) {
	var c Category
	err := r.db.QueryRow(
		"SELECT id, name, type, parent_id FROM categories WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Name, &c.Type, &c.ParentID)

	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *SQLiteRepository) AddTransaction(t Transaction) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO transactions (amount, category_id, date, payment_method, comment) VALUES (?, ?, ?, ?, ?)",
		t.Amount, t.CategoryID, t.Date.Format(time.RFC3339), t.PaymentMethod, t.Comment,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return int(id), err
}

func (r *SQLiteRepository) GetTransactionsByPeriod(start, end time.Time) ([]Transaction, error) {
	rows, err := r.db.Query(
		`SELECT id, amount, category_id, date, payment_method, comment 
		 FROM transactions 
		 WHERE date BETWEEN ? AND ?`,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.CategoryID, &dateStr, &t.PaymentMethod, &t.Comment)
		if err != nil {
			return nil, err
		}

		t.Date, err = time.Parse(time.RFC3339, dateStr)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, t)
	}

	return transactions, nil
}
