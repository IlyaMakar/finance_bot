package service

import (
	"fmt"
	"log"
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
	return s.repo.DeleteCategory(s.userID, id)
}

func (s *FinanceService) RenameCategory(id int, newName string) error {
	return s.repo.RenameCategory(s.userID, id, newName)
}

func (s *FinanceService) GetCategories() ([]repository.Category, error) {
	cats, err := s.repo.GetCategories(s.userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить категории: %v", err)
	}

	if len(cats) == 0 {
		log.Printf("Для пользователя %d категории не найдены, создаем базовые", s.userID)
	} else {
		log.Printf("Найдено %d категорий для пользователя %d", len(cats), s.userID)
	}

	return cats, nil
}

func (s *FinanceService) CreateCategory(name, typ string, parent *int) (int, error) {
	return s.repo.CreateCategory(s.userID, repository.Category{
		Name:     name,
		Type:     typ,
		ParentID: parent,
	})
}

func (s *FinanceService) GetCategoryByID(id int) (*repository.Category, error) {
	if id <= 0 {
		return nil, fmt.Errorf("неверный ID категории")
	}

	cat, err := s.repo.GetCategoryByID(s.userID, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка базы данных: %v", err)
	}

	if cat == nil {
		return nil, fmt.Errorf("категория не найдена или не принадлежит пользователю")
	}

	return cat, nil
}

func (s *FinanceService) GetCategoryWithTypeCheck(id int, expectedType string) (*repository.Category, error) {
	cat, err := s.GetCategoryByID(id)
	if err != nil {
		return nil, fmt.Errorf("категория не найдена: %v", err)
	}

	if cat.Type != expectedType {
		return nil, fmt.Errorf("несоответствие типа категории: ожидается %s, получено %s", expectedType, cat.Type)
	}

	return cat, nil
}

func (s *FinanceService) AddTransaction(amount float64, categoryID int, method, comment string) (int, error) {
	cat, err := s.GetCategoryByID(categoryID)
	if err != nil {
		return 0, fmt.Errorf("ошибка категории: %v", err)
	}

	expectedType := "income"
	if amount < 0 {
		expectedType = "expense"
	}

	if cat.Type != expectedType {
		return 0, fmt.Errorf("несоответствие типа: категория %s, операция %s", cat.Type, expectedType)
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
	transactions, err := s.repo.GetTransactionsByPeriod(s.userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить транзакции: %v", err)
	}

	for i := range transactions {
		if cat, err := s.repo.GetCategoryByID(s.userID, transactions[i].CategoryID); err == nil {
			transactions[i].CategoryName = cat.Name
		} else {
			transactions[i].CategoryName = "Неизвестно"
		}
	}

	return transactions, nil
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

func (s *FinanceService) SetNotificationsEnabled(enabled bool) error {
	return s.repo.UpdateUserNotifications(s.userID, enabled)
}

func (s *FinanceService) GetNotificationsEnabled() (bool, error) {
	return s.repo.GetUserNotificationsEnabled(s.userID)
}

func (s *FinanceService) ClearUserData() error {
	return s.repo.ClearUserData(s.userID)
}

func (s *FinanceService) GetTransactionByID(id int) (*repository.Transaction, error) {
	return s.repo.GetTransactionByID(s.userID, id)
}

func (s *FinanceService) UpdateTransactionAmount(id int, amount float64) error {
	return s.repo.UpdateTransactionAmount(s.userID, id, amount)
}

func (s *FinanceService) UpdateTransactionComment(id int, comment string) error {
	return s.repo.UpdateTransactionComment(s.userID, id, comment)
}

func (s *FinanceService) DeleteTransaction(id int) error {
	return s.repo.DeleteTransaction(s.userID, id)
}

func (s *FinanceService) DeleteSaving(id int) error {
	if id <= 0 {
		return fmt.Errorf("неверный ID копилки")
	}
	return s.repo.DeleteSaving(s.userID, id)
}

func (s *FinanceService) RenameSaving(id int, newName string) error {
	if id <= 0 {
		return fmt.Errorf("неверный ID копилки")
	}
	if newName == "" {
		return fmt.Errorf("название не может быть пустым")
	}
	return s.repo.RenameSaving(s.userID, id, newName)
}

func (s *FinanceService) AddVersion(version, description string) error {
	return s.repo.AddVersion(version, description)
}

func (s *FinanceService) GetLatestVersion() (*repository.Version, error) {
	return s.repo.GetLatestVersion()
}

func (s *FinanceService) MarkVersionAsRead(versionID int) error {
	return s.repo.MarkVersionAsRead(s.userID, versionID)
}

func (s *FinanceService) HasUserReadVersion(versionID int) (bool, error) {
	return s.repo.HasUserReadVersion(s.userID, versionID)
}

func (s *FinanceService) SetPeriodStartDay(day int) error {
	return s.repo.UpdateUserPeriodStartDay(s.userID, day)
}
