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
CREATE TABLE IF NOT EXISTS categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  type TEXT NOT NULL CHECK(type IN ('income','expense','saving')),
  parent_id INTEGER,
  FOREIGN KEY(parent_id) REFERENCES categories(id)
);

CREATE TABLE IF NOT EXISTS transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  amount REAL NOT NULL,
  category_id INTEGER NOT NULL,
  date TEXT NOT NULL,
  payment_method TEXT CHECK(payment_method IN ('cash','card')),
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

CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_id);
`
	_, err := db.Exec(schema)
	return err
}

func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) CreateCategory(c Category) (int, error) {
	res, err := r.db.Exec("INSERT INTO categories(name,type,parent_id) VALUES(?,?,?)", c.Name, c.Type, c.ParentID)
	if err != nil {
		return 0, fmt.Errorf("create category: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (r *SQLiteRepository) GetCategories() ([]Category, error) {
	rows, err := r.db.Query("SELECT id, name, type, parent_id FROM categories ORDER BY name")
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
		cats = append(cats, c)
	}
	return cats, nil
}

func (r *SQLiteRepository) CheckCategoryUsage(categoryID int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM transactions WHERE category_id = ?",
		categoryID,
	).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("ошибка проверки использования категории: %w", err)
	}

	return count > 0, nil
}

func (r *SQLiteRepository) IsCategoryNameUnique(name string, excludeID int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM categories WHERE name = ? AND id != ?",
		name, excludeID,
	).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("ошибка проверки уникальности имени: %w", err)
	}

	return count == 0, nil
}

func (r *SQLiteRepository) GetCategoryByID(id int) (*Category, error) {
	var c Category
	var pid *int
	row := r.db.QueryRow("SELECT id, name, type, parent_id FROM categories WHERE id = ?", id)
	if err := row.Scan(&c.ID, &c.Name, &c.Type, &pid); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("категория не найдена")
		}
		return nil, fmt.Errorf("scan category: %w", err)
	}
	c.ParentID = pid
	return &c, nil
}

func (r *SQLiteRepository) AddTransaction(t Transaction) (int, error) {
	res, err := r.db.Exec(
		"INSERT INTO transactions(amount,category_id,date,payment_method,comment) VALUES(?,?,?,?,?)",
		t.Amount, t.CategoryID, t.Date.Format(time.RFC3339), t.PaymentMethod, t.Comment,
	)
	if err != nil {
		return 0, fmt.Errorf("insert trans: %w", err)
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}
func (r *SQLiteRepository) DeleteCategory(id int) error {
	_, err := r.db.Exec("DELETE FROM categories WHERE id = ?", id)
	return err
}

func (r *SQLiteRepository) RenameCategory(id int, newName string) error {
	_, err := r.db.Exec("UPDATE categories SET name = ? WHERE id = ?", newName, id)
	return err
}

func (r *SQLiteRepository) GetTransactionsByPeriod(start, end time.Time) ([]Transaction, error) {
	rows, err := r.db.Query(
		"SELECT id,amount,category_id,date,payment_method,comment FROM transactions WHERE date >= ? AND date < ? ORDER BY date DESC",
		start.Format(time.RFC3339), end.Format(time.RFC3339),
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
		res = append(res, t)
	}
	return res, nil
}

func (r *SQLiteRepository) GetSavings() ([]Saving, error) {
	rows, err := r.db.Query("SELECT id,name,amount,goal,comment FROM savings ORDER BY name")
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
		list = append(list, s)
	}
	return list, nil
}

func (r *SQLiteRepository) GetSavingByID(id int) (*Saving, error) {
	var s Saving
	var goal sql.NullFloat64
	var comment sql.NullString

	err := r.db.QueryRow(
		"SELECT id, name, amount, goal, comment FROM savings WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.Amount, &goal, &comment)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Возвращаем nil вместо ошибки
		}
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	if goal.Valid {
		s.Goal = &goal.Float64
	}
	if comment.Valid {
		s.Comment = comment.String
	}

	return &s, nil
}

func (r *SQLiteRepository) UpdateSavingAmount(id int, amount float64) error {
	_, err := r.db.Exec("UPDATE savings SET amount = ? WHERE id = ?", amount, id)
	return err
}

func (r *SQLiteRepository) CreateSaving(name string, goal *float64) error {
	_, err := r.db.Exec("INSERT INTO savings (name, amount, goal) VALUES (?, 0, ?)", name, goal)
	return err
}
