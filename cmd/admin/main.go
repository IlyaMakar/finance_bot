package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type StatsResponse struct {
	TotalUsers    int            `json:"total_users"`
	ActiveToday   int            `json:"active_today"`
	ActiveWeek    int            `json:"active_week"`
	ActiveMonth   int            `json:"active_month"`
	ButtonClicks  map[string]int `json:"button_clicks"`
	AllUsers      []UserStats    `json:"all_users"`
	FeedbackStats FeedbackStats  `json:"feedback_stats"`
	AllFeedbacks  []Feedback     `json:"all_feedbacks"`
}

type UserStats struct {
	TelegramID int64     `json:"telegram_id"`
	Username   string    `json:"username"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	LastActive time.Time `json:"last_active"`
	JoinDate   time.Time `json:"join_date"`
}

type FeedbackStats struct {
	Total        int     `json:"total"`
	RecommendYes int     `json:"recommend_yes"`
	RecommendNo  int     `json:"recommend_no"`
	YesPercent   float64 `json:"yes_percent"`
	NoPercent    float64 `json:"no_percent"`
}

type Feedback struct {
	ID           int       `json:"id"`
	TelegramID   int64     `json:"telegram_id"`
	Username     string    `json:"username"`
	WhatLikes    string    `json:"what_likes"`
	WhatMissing  string    `json:"what_missing"`
	WhatAnnoying string    `json:"what_annoying"`
	Recommend    string    `json:"recommend"`
	CreatedAt    time.Time `json:"created_at"`
}

var (
	botAPIURL string
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}

	botAPIURL = os.Getenv("BOT_API_URL")
	if botAPIURL == "" {
		botAPIURL = "http://localhost:8080"
	}

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", dashboardHandler)
	http.HandleFunc("/api/stats", apiStatsHandler)
	http.HandleFunc("/api/users", apiUsersHandler)
	http.HandleFunc("/api/feedbacks", apiFeedbacksHandler)

	port := 3000
	if envPort := os.Getenv("ADMIN_PORT"); envPort != "" {
		port, _ = strconv.Atoi(envPort)
	}

	log.Printf("🚀 Admin panel starting on http://localhost:%d", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))
	tmpl.Execute(w, nil)
}

func apiStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := fetchStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func apiUsersHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := fetchStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats.AllUsers)
}

func apiFeedbacksHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := fetchStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats.AllFeedbacks)
}

func fetchStats() (*StatsResponse, error) {
	log.Printf("🔗 Trying to connect to: %s/api/stats", botAPIURL)

	resp, err := http.Get(botAPIURL + "/api/stats")
	if err != nil {
		log.Printf("❌ Connection failed: %v", err)
		return nil, fmt.Errorf("failed to fetch stats: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("✅ Connected successfully, status: %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Failed to read response: %v", err)
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	log.Printf("📊 Response received, length: %d bytes", len(body))

	var stats StatsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		log.Printf("❌ Failed to parse JSON: %v", err)
		return nil, fmt.Errorf("failed to parse stats: %v", err)
	}

	log.Printf("✅ Stats parsed successfully: %d users, %d feedbacks", stats.TotalUsers, stats.FeedbackStats.Total)
	return &stats, nil
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Finance Bot Admin</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
    <style>
        :root {
            --primary: #6366f1;
            --primary-dark: #4f46e5;
            --secondary: #10b981;
            --danger: #ef4444;
            --warning: #f59e0b;
            --info: #3b82f6;
            --dark: #1f2937;
            --light: #f8fafc;
            --gray: #6b7280;
            --gray-light: #e5e7eb;
            --border-radius: 12px;
            --shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1);
            --transition: all 0.3s ease;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Inter', 'Segoe UI', system-ui, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            color: var(--dark);
            line-height: 1.6;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px;
        }

        .header {
            background: rgba(255, 255, 255, 0.95);
            backdrop-filter: blur(10px);
            padding: 30px 40px;
            border-radius: var(--border-radius);
            margin-bottom: 30px;
            box-shadow: var(--shadow);
            display: flex;
            justify-content: space-between;
            align-items: center;
            border: 1px solid rgba(255, 255, 255, 0.2);
        }

        .header-content h1 {
            color: var(--dark);
            font-size: 2.5em;
            font-weight: 700;
            margin-bottom: 8px;
            background: linear-gradient(135deg, var(--primary), var(--primary-dark));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .header-content p {
            color: var(--gray);
            font-size: 1.1em;
            font-weight: 500;
        }

        .header-stats {
            display: flex;
            gap: 30px;
        }

        .header-stat {
            text-align: center;
        }

        .header-stat .number {
            display: block;
            font-size: 2em;
            font-weight: 700;
            color: var(--primary);
        }

        .header-stat .label {
            font-size: 0.9em;
            color: var(--gray);
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .controls {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 30px;
            gap: 20px;
        }

        .refresh-btn {
            background: linear-gradient(135deg, var(--primary), var(--primary-dark));
            color: white;
            border: none;
            padding: 14px 28px;
            border-radius: var(--border-radius);
            cursor: pointer;
            font-size: 1em;
            font-weight: 600;
            transition: var(--transition);
            display: flex;
            align-items: center;
            gap: 10px;
            box-shadow: var(--shadow);
        }

        .refresh-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 20px 40px -10px rgba(99, 102, 241, 0.4);
        }

        .refresh-btn:active {
            transform: translateY(0);
        }

        .last-update {
            color: white;
            font-weight: 500;
            text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 25px;
            margin-bottom: 30px;
        }

        .stat-card {
            background: rgba(255, 255, 255, 0.95);
            backdrop-filter: blur(10px);
            padding: 25px;
            border-radius: var(--border-radius);
            box-shadow: var(--shadow);
            text-align: center;
            transition: var(--transition);
            border: 1px solid rgba(255, 255, 255, 0.2);
            position: relative;
            overflow: hidden;
        }

        .stat-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: 4px;
            background: linear-gradient(90deg, var(--primary), var(--secondary));
        }

        .stat-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.15);
        }

        .stat-icon {
            font-size: 2.5em;
            margin-bottom: 15px;
            opacity: 0.8;
        }

        .stat-card h3 {
            color: var(--gray);
            font-size: 0.9em;
            margin-bottom: 15px;
            text-transform: uppercase;
            letter-spacing: 1px;
            font-weight: 600;
        }

        .stat-number {
            font-size: 2.8em;
            font-weight: 800;
            color: var(--dark);
            margin-bottom: 10px;
            line-height: 1;
        }

        .stat-trend {
            font-size: 0.85em;
            padding: 6px 12px;
            border-radius: 20px;
            display: inline-block;
            font-weight: 600;
        }

        .trend-up { background: rgba(16, 185, 129, 0.1); color: var(--secondary); }
        .trend-down { background: rgba(239, 68, 68, 0.1); color: var(--danger); }

        .charts-container {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 25px;
            margin-bottom: 30px;
        }

        .chart-card {
            background: rgba(255, 255, 255, 0.95);
            backdrop-filter: blur(10px);
            padding: 30px;
            border-radius: var(--border-radius);
            box-shadow: var(--shadow);
            border: 1px solid rgba(255, 255, 255, 0.2);
        }

        .chart-card h3 {
            color: var(--dark);
            margin-bottom: 20px;
            text-align: center;
            font-size: 1.3em;
            font-weight: 700;
        }

        .tabs-container {
            background: rgba(255, 255, 255, 0.95);
            backdrop-filter: blur(10px);
            border-radius: var(--border-radius);
            box-shadow: var(--shadow);
            border: 1px solid rgba(255, 255, 255, 0.2);
            margin-bottom: 30px;
            overflow: hidden;
        }

        .tabs-header {
            display: flex;
            background: linear-gradient(135deg, var(--primary), var(--primary-dark));
            padding: 0;
        }

        .tab-btn {
            flex: 1;
            background: none;
            border: none;
            color: white;
            padding: 20px;
            font-size: 1.1em;
            font-weight: 600;
            cursor: pointer;
            transition: var(--transition);
            position: relative;
            overflow: hidden;
        }

        .tab-btn:hover {
            background: rgba(255, 255, 255, 0.1);
        }

        .tab-btn.active {
            background: rgba(255, 255, 255, 0.2);
        }

        .tab-btn.active::after {
            content: '';
            position: absolute;
            bottom: 0;
            left: 20%;
            right: 20%;
            height: 3px;
            background: white;
            border-radius: 3px 3px 0 0;
        }

        .tab-content {
            display: none;
            padding: 0;
            max-height: 600px;
            overflow-y: auto;
        }

        .tab-content.active {
            display: block;
        }

        .table-card {
            padding: 0;
        }

        .table-card h3 {
            padding: 25px 30px 0;
            margin: 0;
            color: var(--dark);
            font-size: 1.3em;
            font-weight: 700;
        }

        table {
            width: 100%;
            border-collapse: collapse;
        }

        th {
            background: var(--light);
            padding: 16px 20px;
            text-align: left;
            font-weight: 600;
            color: var(--dark);
            border-bottom: 2px solid var(--gray-light);
            position: sticky;
            top: 0;
            backdrop-filter: blur(10px);
        }

        td {
            padding: 16px 20px;
            border-bottom: 1px solid var(--gray-light);
            vertical-align: top;
        }

        tr:hover {
            background: rgba(99, 102, 241, 0.05);
        }

        .user-avatar {
            width: 40px;
            height: 40px;
            border-radius: 50%;
            background: linear-gradient(135deg, var(--primary), var(--secondary));
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-weight: 600;
            margin-right: 12px;
        }

        .user-info {
            display: flex;
            align-items: center;
        }

        .user-details {
            display: flex;
            flex-direction: column;
        }

        .user-name {
            font-weight: 600;
            color: var(--dark);
        }

        .user-username {
            color: var(--gray);
            font-size: 0.9em;
        }

        .status-badge {
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 0.8em;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .status-active { background: rgba(16, 185, 129, 0.1); color: var(--secondary); }
        .status-inactive { background: rgba(107, 114, 128, 0.1); color: var(--gray); }

        .feedback-content {
            max-width: 300px;
        }

        .feedback-text {
            margin-bottom: 8px;
            line-height: 1.5;
        }

        .feedback-label {
            font-weight: 600;
            color: var(--gray);
            font-size: 0.9em;
            margin-bottom: 4px;
        }

        .recommend-badge {
            padding: 6px 12px;
            border-radius: 20px;
            font-weight: 600;
            font-size: 0.85em;
        }

        .recommend-yes { background: rgba(16, 185, 129, 0.1); color: var(--secondary); }
        .recommend-no { background: rgba(239, 68, 68, 0.1); color: var(--danger); }

        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: var(--gray);
        }

        .empty-state i {
            font-size: 3em;
            margin-bottom: 20px;
            opacity: 0.5;
        }

        .search-box {
            background: white;
            border: 2px solid var(--gray-light);
            border-radius: var(--border-radius);
            padding: 12px 20px;
            font-size: 1em;
            width: 300px;
            transition: var(--transition);
        }

        .search-box:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1);
        }

        @media (max-width: 768px) {
            .charts-container {
                grid-template-columns: 1fr;
            }
            
            .header {
                flex-direction: column;
                gap: 20px;
                text-align: center;
            }
            
            .header-stats {
                justify-content: center;
            }
            
            .controls {
                flex-direction: column;
            }
            
            .search-box {
                width: 100%;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="header-content">
                <h1><i class="fas fa-robot"></i> Finance Bot Admin</h1>
                <p>Real-time monitoring and analytics dashboard</p>
            </div>
            <div class="header-stats">
                <div class="header-stat">
                    <span class="number" id="headerUsers">0</span>
                    <span class="label">Users</span>
                </div>
                <div class="header-stat">
                    <span class="number" id="headerActive">0</span>
                    <span class="label">Active</span>
                </div>
                <div class="header-stat">
                    <span class="number" id="headerFeedback">0</span>
                    <span class="label">Feedback</span>
                </div>
            </div>
        </div>

        <div class="controls">
            <button class="refresh-btn" onclick="loadStats()">
                <i class="fas fa-sync-alt"></i>
                Обновить данные
            </button>
            <div class="last-update" id="lastUpdate">Последнее обновление: --:--:--</div>
            <input type="text" class="search-box" id="searchInput" placeholder="Поиск пользователей и отзывов..." onkeyup="filterTables()">
        </div>

        <div class="stats-grid" id="statsGrid">
            <!-- Stats will be loaded here -->
        </div>

        <div class="charts-container">
            <div class="chart-card">
                <h3><i class="fas fa-chart-bar"></i> Активность пользователей</h3>
                <canvas id="activityChart"></canvas>
            </div>
            <div class="chart-card">
                <h3><i class="fas fa-chart-pie"></i> Статистика кнопок</h3>
                <canvas id="buttonsChart"></canvas>
            </div>
        </div>

        <div class="tabs-container">
            <div class="tabs-header">
                <button class="tab-btn active" onclick="switchTab('users')">
                    <i class="fas fa-users"></i> Все пользователи
                </button>
                <button class="tab-btn" onclick="switchTab('feedback')">
                    <i class="fas fa-comments"></i> Все отзывы
                </button>
            </div>
            
            <div class="tab-content active" id="usersTab">
                <div class="table-card">
                    <h3><i class="fas fa-list"></i> Список пользователей (<span id="usersCount">0</span>)</h3>
                    <div id="usersTable">
                        <!-- Users table will be loaded here -->
                    </div>
                </div>
            </div>
            
            <div class="tab-content" id="feedbackTab">
                <div class="table-card">
                    <h3><i class="fas fa-star"></i> Все отзывы (<span id="feedbackCount">0</span>)</h3>
                    <div id="feedbackTable">
                        <!-- Feedback table will be loaded here -->
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let activityChart, buttonsChart;
        let currentStats = null;

        async function loadStats() {
            try {
                const response = await fetch('/api/stats');
                currentStats = await response.json();
                
                updateHeaderStats(currentStats);
                updateStatsGrid(currentStats);
                updateCharts(currentStats);
                updateTables(currentStats);
                
                document.getElementById('lastUpdate').textContent = 
                    'Последнее обновление: ' + new Date().toLocaleString('ru-RU');
            } catch (error) {
                console.error('Error loading stats:', error);
                alert('Ошибка загрузки данных');
            }
        }

        function updateHeaderStats(stats) {
            document.getElementById('headerUsers').textContent = stats.total_users;
            document.getElementById('headerActive').textContent = stats.active_today;
            document.getElementById('headerFeedback').textContent = stats.feedback_stats.total;
        }

        function updateStatsGrid(stats) {
            const statsGrid = document.getElementById('statsGrid');
            statsGrid.innerHTML = 
                '<div class="stat-card">' +
                '<div class="stat-icon"><i class="fas fa-users"></i></div>' +
                '<h3>Всего пользователей</h3>' +
                '<div class="stat-number">' + stats.total_users + '</div>' +
                '<div class="stat-trend trend-up">Всего в системе</div>' +
                '</div>' +
                '<div class="stat-card">' +
                '<div class="stat-icon"><i class="fas fa-bolt"></i></div>' +
                '<h3>Активных сегодня</h3>' +
                '<div class="stat-number">' + stats.active_today + '</div>' +
                '<div class="stat-trend trend-up">За 24 часа</div>' +
                '</div>' +
                '<div class="stat-card">' +
                '<div class="stat-icon"><i class="fas fa-calendar-week"></i></div>' +
                '<h3>Активных за неделю</h3>' +
                '<div class="stat-number">' + stats.active_week + '</div>' +
                '<div class="stat-trend trend-up">За 7 дней</div>' +
                '</div>' +
                '<div class="stat-card">' +
                '<div class="stat-icon"><i class="fas fa-calendar-alt"></i></div>' +
                '<h3>Активных за месяц</h3>' +
                '<div class="stat-number">' + stats.active_month + '</div>' +
                '<div class="stat-trend trend-up">За 30 дней</div>' +
                '</div>' +
                '<div class="stat-card">' +
                '<div class="stat-icon"><i class="fas fa-comment-dots"></i></div>' +
                '<h3>Всего отзывов</h3>' +
                '<div class="stat-number">' + stats.feedback_stats.total + '</div>' +
                '<div class="stat-trend trend-up">Обратная связь</div>' +
                '</div>' +
                '<div class="stat-card">' +
                '<div class="stat-icon"><i class="fas fa-star"></i></div>' +
                '<h3>Рекомендуют</h3>' +
                '<div class="stat-number">' + stats.feedback_stats.recommend_yes + '</div>' +
                '<div class="stat-trend trend-up">' + stats.feedback_stats.yes_percent.toFixed(1) + '%</div>' +
                '</div>';
        }

        function updateCharts(stats) {
            const activityCtx = document.getElementById('activityChart').getContext('2d');
            if (activityChart) activityChart.destroy();
            
            activityChart = new Chart(activityCtx, {
                type: 'bar',
                data: {
                    labels: ['Сегодня', 'Неделя', 'Месяц'],
                    datasets: [{
                        label: 'Активные пользователи',
                        data: [stats.active_today, stats.active_week, stats.active_month],
                        backgroundColor: [
                            'rgba(99, 102, 241, 0.8)',
                            'rgba(16, 185, 129, 0.8)',
                            'rgba(245, 158, 11, 0.8)'
                        ],
                        borderColor: [
                            'rgb(99, 102, 241)',
                            'rgb(16, 185, 129)',
                            'rgb(245, 158, 11)'
                        ],
                        borderWidth: 2,
                        borderRadius: 6
                    }]
                },
                options: {
                    responsive: true,
                    plugins: {
                        legend: {
                            position: 'top',
                        },
                        title: {
                            display: true,
                            text: 'Активность по периодам'
                        }
                    },
                    scales: {
                        y: {
                            beginAtZero: true,
                            grid: {
                                color: 'rgba(0, 0, 0, 0.1)'
                            }
                        },
                        x: {
                            grid: {
                                display: false
                            }
                        }
                    }
                }
            });

            const buttonsCtx = document.getElementById('buttonsChart').getContext('2d');
            if (buttonsChart) buttonsChart.destroy();
            
            const buttonNames = Object.keys(stats.button_clicks);
            const buttonCounts = Object.values(stats.button_clicks);
            
            buttonsChart = new Chart(buttonsCtx, {
                type: 'doughnut',
                data: {
                    labels: buttonNames,
                    datasets: [{
                        data: buttonCounts,
                        backgroundColor: [
                            '#FF6384', '#36A2EB', '#FFCE56', '#4BC0C0',
                            '#9966FF', '#FF9F40', '#8c9eff', '#C9CBCF'
                        ],
                        borderWidth: 2,
                        borderColor: 'white'
                    }]
                },
                options: {
                    responsive: true,
                    plugins: {
                        legend: {
                            position: 'right',
                        },
                        title: {
                            display: true,
                            text: 'Нажатия кнопок'
                        }
                    },
                    cutout: '60%'
                }
            });
        }

        function updateTables(stats) {
            document.getElementById('usersCount').textContent = stats.all_users.length;
            const usersTable = document.getElementById('usersTable');
            
            if (stats.all_users.length === 0) {
                usersTable.innerHTML = '<div class="empty-state"><i class="fas fa-users"></i><p>Пользователи не найдены</p></div>';
                return;
            }
            
            let usersHTML = '<table><thead><tr><th>Пользователь</th><th>Telegram ID</th><th>Дата регистрации</th><th>Последняя активность</th><th>Статус</th></tr></thead><tbody>';
            
            stats.all_users.forEach(function(user) {
                const isActive = new Date(user.last_active) > new Date(Date.now() - 7 * 24 * 60 * 60 * 1000);
                const statusClass = isActive ? 'status-active' : 'status-inactive';
                const statusText = isActive ? 'Активен' : 'Неактивен';
                const userInitial = user.first_name ? user.first_name.charAt(0).toUpperCase() : 'U';
                
                usersHTML += '<tr class="user-row">' +
                    '<td>' +
                    '<div class="user-info">' +
                    '<div class="user-avatar">' + userInitial + '</div>' +
                    '<div class="user-details">' +
                    '<div class="user-name">' + (user.first_name || 'Unknown') + ' ' + (user.last_name || '') + '</div>' +
                    '<div class="user-username">@' + (user.username || 'no_username') + '</div>' +
                    '</div>' +
                    '</div>' +
                    '</td>' +
                    '<td>' + user.telegram_id + '</td>' +
                    '<td>' + new Date(user.join_date).toLocaleDateString('ru-RU') + '</td>' +
                    '<td>' + new Date(user.last_active).toLocaleDateString('ru-RU') + '</td>' +
                    '<td><span class="status-badge ' + statusClass + '">' + statusText + '</span></td>' +
                    '</tr>';
            });
            
            usersHTML += '</tbody></table>';
            usersTable.innerHTML = usersHTML;

            document.getElementById('feedbackCount').textContent = stats.all_feedbacks.length;
            const feedbackTable = document.getElementById('feedbackTable');
            
            if (stats.all_feedbacks.length === 0) {
                feedbackTable.innerHTML = '<div class="empty-state"><i class="fas fa-comments"></i><p>Отзывы не найдены</p></div>';
                return;
            }
            
            let feedbackHTML = '<table><thead><tr><th>Пользователь</th><th>Что нравится</th><th>Чего не хватает</th><th>Что раздражает</th><th>Рекомендация</th><th>Дата</th></tr></thead><tbody>';
            
            stats.all_feedbacks.forEach(function(fb) {
                const recommendClass = fb.recommend === 'yes' ? 'recommend-yes recommend-badge' : 'recommend-no recommend-badge';
                const recommendText = fb.recommend === 'yes' ? '✅ Рекомендует' : '❌ Не рекомендует';
                
                feedbackHTML += '<tr class="feedback-row">' +
                    '<td><strong>@' + (fb.username || fb.telegram_id) + '</strong></td>' +
                    '<td class="feedback-content"><div class="feedback-text">' + (fb.what_likes || '—') + '</div></td>' +
                    '<td class="feedback-content"><div class="feedback-text">' + (fb.what_missing || '—') + '</div></td>' +
                    '<td class="feedback-content"><div class="feedback-text">' + (fb.what_annoying || '—') + '</div></td>' +
                    '<td><span class="' + recommendClass + '">' + recommendText + '</span></td>' +
                    '<td>' + new Date(fb.created_at).toLocaleDateString('ru-RU') + '</td>' +
                    '</tr>';
            });
            
            feedbackHTML += '</tbody></table>';
            feedbackTable.innerHTML = feedbackHTML;
        }

        function switchTab(tabName) {
        
            document.querySelectorAll('.tab-content').forEach(tab => {
                tab.classList.remove('active');
            });
            document.querySelectorAll('.tab-btn').forEach(btn => {
                btn.classList.remove('active');
            });
            
        
            document.getElementById(tabName + 'Tab').classList.add('active');
            event.currentTarget.classList.add('active');
        }

        function filterTables() {
            const searchTerm = document.getElementById('searchInput').value.toLowerCase();
            
            document.querySelectorAll('.user-row').forEach(row => {
                const text = row.textContent.toLowerCase();
                row.style.display = text.includes(searchTerm) ? '' : 'none';
            });
            
            document.querySelectorAll('.feedback-row').forEach(row => {
                const text = row.textContent.toLowerCase();
                row.style.display = text.includes(searchTerm) ? '' : 'none';
            });
        }

        loadStats();
        setInterval(loadStats, 30000);

        window.addEventListener('resize', function() {
            if (currentStats) {
                updateCharts(currentStats);
            }
        });
    </script>
</body>
</html>`
