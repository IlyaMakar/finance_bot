package service

import (
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
)

type FinanceService struct {
	repo *repository.SQLiteRepository
}

func NewService(repo *repository.SQLiteRepository) *FinanceService {
	return &FinanceService{repo: repo}
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
