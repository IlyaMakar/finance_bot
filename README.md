# 💰 Spendy – Telegram-бот для учёта финансов

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Telegram Bot](https://img.shields.io/badge/Telegram-%40trackfinans__bot-blue)](https://t.me/trackfinans_bot)

Простой и удобный Telegram-бот на Go для управления личными финансами с аналитикой и автоматическими напоминаниями.

## 🌟 Основные возможности

### 📊 Учет операций

- ➕ Добавление доходов/расходов простыми командами
- ✏️ Редактирование и удаление транзакций
- 📝 История операций с фильтрацией

### 📈 Аналитика

- Статистика за день/неделю/месяц
- Визуализация данных (графики)
- Экспорт отчетов в PDF

### ⚙️ Удобство

- 🔔 Автонапоминания о транзакциях
- 💾 Локальное хранение в SQLite
- 👥 Мультипользовательский режим

## 🚀 Быстрый старт (для пользователей)

1. Откройте бота в Telegram: [@trackfinans_bot](https://t.me/trackfinans_bot)
2. Нажмите `/start`
3. Используйте интуитивное меню:

1500 продукты # Добавить расход 1500 ₽ на продукты
5000 зарплата # Добавить доход 5000 ₽ (зарплата)
🛠 Установка (для разработчиков)
Требования
Go 1.21+

SQLite3



# 1. Клонируйте репозиторий

git clone https://github.com/IlyaMakar/finance_bot.git
cd finance_bot

# 2. Настройте окружение

cp .env.example .env
nano .env # задайте TELEGRAM_TOKEN

# 3. Запустите бота

go run cmd/bot/main.go
Конфигурация (.env)
ini
TELEGRAM_TOKEN=your_bot_token
TIMEZONE=Europe/Moscow # Часовой пояс для напоминаний
TEST_MODE=false # Режим тестирования


## 🏗 Архитектура проекта
```text
finance_bot/
├── cmd/
│ └── bot/ # Точка входа (main.go)
├── internal/
│ ├── bot/ # Логика Telegram-бота
│ │ ├── handlers/ # Обработчики команд
│ │ └── reports/ # Генерация отчетов
│ ├── repository/ # Работа с SQLite
│ ├── service/ # Бизнес-логика
│ └── logger/ # Система логирования
├── go.mod # Зависимости
└── README.md # Документация
``` 
## 📌 Примеры использования
Добавление операций


Просмотр статистики
📊 Статистика за месяц:
Доходы: 85,000 ₽
Расходы: 42,500 ₽
Баланс: +42,500 ₽

Топ расходов:

1. Продукты: 15,000 ₽
2. Транспорт: 8,200 ₽

## 🤝 Как помочь проекту
# Приветствуются contributions! Вот что можно улучшить:
- Интеграция с банковскими API
- Кастомные категории расходов
- Экспорт в Excel/Google Sheets

# Порядок внесения изменений:
- Создайте issue для обсуждения
- Сделайте fork репозитория
- Отправьте pull request

## 📜 Лицензия
MIT License - подробности в файле LICENSE

## 📩 Контакты
- Автор: Илья Макаров
- Telegram: @LONEl1st
- Issues: https://github.com/IlyaMakar/finance_bot/issues
