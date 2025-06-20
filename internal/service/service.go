package service

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
)

type FinanceService struct {
	repo *repository.SQLiteRepository
}

func NewService(r *repository.SQLiteRepository) *FinanceService {
	return &FinanceService{repo: r}
}

func (s *FinanceService) GetCategories() ([]repository.Category, error) {
	return s.repo.GetCategories()
}

func (s *FinanceService) CreateCategory(name, typ string, parent *int) (int, error) {
	return s.repo.CreateCategory(repository.Category{Name: name, Type: typ, ParentID: parent})
}

func (s *FinanceService) GetCategoryByID(id int) (*repository.Category, error) {
	return s.repo.GetCategoryByID(id)
}

func (s *FinanceService) AddTransaction(amount float64, categoryID int, method, comment string) (int, error) {
	c, err := s.repo.GetCategoryByID(categoryID)
	if err != nil {
		return 0, fmt.Errorf("категория не найдена")
	}
	if (amount < 0 && c.Type != "expense") || (amount > 0 && c.Type != "income") {
		return 0, fmt.Errorf("несоответствие типа и категории")
	}
	return s.repo.AddTransaction(repository.Transaction{
		Amount:        amount,
		CategoryID:    categoryID,
		Date:          time.Now(),
		PaymentMethod: method,
		Comment:       comment,
	})
}

func (s *FinanceService) GetTransactionsForPeriod(start, end time.Time) ([]repository.Transaction, error) {
	return s.repo.GetTransactionsByPeriod(start, end)
}

func (s *FinanceService) GetSavings() ([]repository.Saving, error) {
	return s.repo.GetSavings()
}

func (s *FinanceService) GetSavingByID(id int) (*repository.Saving, error) {
	if id <= 0 {
		return nil, fmt.Errorf("ID копилки должен быть положительным числом")
	}

	saving, err := s.repo.GetSavingByID(id)
	if err != nil {
		return nil, fmt.Errorf("ошибка базы данных: %v", err)
	}

	if saving == nil {
		return nil, fmt.Errorf("копилка не найдена")
	}

	return saving, nil
}

func (s *FinanceService) UpdateSavingAmount(id int, amount float64) error {
	if id <= 0 {
		return fmt.Errorf("неверный ID копилки")
	}
	if amount < 0 {
		return fmt.Errorf("сумма не может быть отрицательной")
	}
	return s.repo.UpdateSavingAmount(id, amount)
}

func (s *FinanceService) CreateSaving(name string, goal *float64) error {
	return s.repo.CreateSaving(name, goal)
}
