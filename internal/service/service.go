package service

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
)

type FinanceService struct {
	repo *repository.SQLiteRepository
}

func NewService(repo *repository.SQLiteRepository) *FinanceService {
	return &FinanceService{repo: repo}
}

func (s *FinanceService) GetTransactionsByPeriod(start, end time.Time) ([]repository.Transaction, error) {
	return s.repo.GetTransactionsByPeriod(start, end)
}

func (s *FinanceService) GetTransactionsForPeriod(start, end time.Time) ([]repository.Transaction, error) {
	return s.repo.GetTransactionsByPeriod(start, end)
}

func (s *FinanceService) GetCategoryByID(id int) (*repository.Category, error) {
	return s.repo.GetCategoryByID(id)
}

func (s *FinanceService) CreateCategory(name, categoryType string, parentID *int) (int, error) {
	category := repository.Category{
		Name:     name,
		Type:     categoryType,
		ParentID: parentID,
	}
	return s.repo.CreateCategory(category)
}

func (s *FinanceService) GetCategories() ([]repository.Category, error) {
	return s.repo.GetCategories()
}

func (s *FinanceService) GetCategory(id int) (*repository.Category, error) {
	return s.repo.GetCategoryByID(id)
}

func (s *FinanceService) AddTransaction(amount float64, categoryID int, paymentMethod, comment string) (int, error) {
	// Проверяем, что категория существует
	category, err := s.repo.GetCategoryByID(categoryID)
	if err != nil {
		return 0, fmt.Errorf("category not found")
	}

	// Проверяем соответствие типа операции
	if (amount > 0 && category.Type != "income") || (amount < 0 && category.Type != "expense") {
		return 0, fmt.Errorf("operation type doesn't match category type")
	}

	transaction := repository.Transaction{
		Amount:        amount,
		CategoryID:    categoryID,
		Date:          time.Now(),
		PaymentMethod: paymentMethod,
		Comment:       comment,
	}
	return s.repo.AddTransaction(transaction)
}

func (s *FinanceService) GetMonthlyReport(year int, month time.Month) ([]repository.Transaction, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, -1)
	return s.repo.GetTransactionsByPeriod(start, end)
}

func (s *FinanceService) UpdateSavingAmount(id int, amount float64) error {
	return s.repo.UpdateSavingAmount(id, amount)
}

func (s *FinanceService) GetSavings() ([]repository.Saving, error) {
	return s.repo.GetSavings()
}

func (s *FinanceService) GetSaving(id int) (*repository.Saving, error) {
	return s.repo.GetSavingByID(id)
}

func (s *FinanceService) AddSaving(name, comment string) (int, error) {
	saving := repository.Saving{
		Name:    name,
		Amount:  0, // Начальный баланс 0
		Comment: comment,
	}
	return s.repo.AddSaving(saving)
}
