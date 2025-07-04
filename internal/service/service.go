package service

import (
	"fmt"
	"time"

	"github.com/IlyaMakar/finance_bot/internal/repository"
)

type FinanceService struct {
	repo   *repository.SQLiteRepository
	userID int
}

func NewService(repo *repository.SQLiteRepository, user *repository.User) *FinanceService {
	return &FinanceService{
		repo:   repo,
		userID: user.ID,
	}
}

func (s *FinanceService) DeleteCategory(id int) error {
	if _, err := s.repo.GetCategoryByID(s.userID, id); err != nil {
		return err
	}
	return s.repo.DeleteCategory(s.userID, id)
}

func (s *FinanceService) RenameCategory(id int, newName string) error {
	// Проверка принадлежности категории пользователю
	if _, err := s.repo.GetCategoryByID(s.userID, id); err != nil {
		return err
	}
	return s.repo.RenameCategory(s.userID, id, newName) // Добавлен s.userID
}

func (s *FinanceService) GetCategories() ([]repository.Category, error) {
	return s.repo.GetCategories(s.userID)
}

func (s *FinanceService) CreateCategory(name, typ string, parent *int) (int, error) {
	return s.repo.CreateCategory(s.userID, repository.Category{
		Name:     name,
		Type:     typ,
		ParentID: parent,
	})
}

func (s *FinanceService) GetCategoryByID(id int) (*repository.Category, error) {
	return s.repo.GetCategoryByID(s.userID, id)
}

func (s *FinanceService) AddTransaction(amount float64, categoryID int, method, comment string) (int, error) {
	// Проверяем, что категория принадлежит пользователю
	c, err := s.repo.GetCategoryByID(s.userID, categoryID)
	if err != nil {
		return 0, fmt.Errorf("категория не найдена")
	}

	if (amount < 0 && c.Type != "expense") || (amount > 0 && c.Type != "income") {
		return 0, fmt.Errorf("несоответствие типа и категории")
	}

	return s.repo.AddTransaction(s.userID, repository.Transaction{
		Amount:        amount,
		CategoryID:    categoryID,
		Date:          time.Now(),
		PaymentMethod: method,
		Comment:       comment,
	})
}

func (s *FinanceService) GetTransactionsForPeriod(start, end time.Time) ([]repository.Transaction, error) {
	return s.repo.GetTransactionsByPeriod(s.userID, start, end)
}

func (s *FinanceService) GetSavings() ([]repository.Saving, error) {
	return s.repo.GetSavings(s.userID)
}

func (s *FinanceService) GetSavingByID(id int) (*repository.Saving, error) {
	if id <= 0 {
		return nil, fmt.Errorf("ID копилки должен быть положительным числом")
	}

	saving, err := s.repo.GetSavingByID(s.userID, id)
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
	return s.repo.UpdateSavingAmount(s.userID, id, amount)
}

func (s *FinanceService) CreateSaving(name string, goal *float64) error {
	return s.repo.CreateSaving(s.userID, name, goal)
}
