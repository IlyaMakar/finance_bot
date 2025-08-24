package handlers

import (
	"bytes"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/IlyaMakar/finance_bot/internal/repository"
	"github.com/IlyaMakar/finance_bot/internal/service"
	"github.com/jung-kurt/gofpdf"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

type ReportGenerator struct {
	bot  *Bot
	repo *repository.SQLiteRepository
}

func NewReportGenerator(bot *Bot, repo *repository.SQLiteRepository) *ReportGenerator {
	return &ReportGenerator{
		bot:  bot,
		repo: repo,
	}
}

func (rg *ReportGenerator) GeneratePDFReport(chatID int64, start, end time.Time, svc *service.FinanceService) ([]byte, error) {
	transactions, err := svc.GetTransactionsForPeriod(start, end)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения транзакций: %v", err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	fontPath := filepath.Join("fonts", "DejaVuSans.ttf")
	pdf.AddUTF8Font("DejaVuSans", "", fontPath)
	pdf.AddUTF8Font("DejaVuSans", "B", fontPath)
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()
	pdf.SetFont("DejaVuSans", "B", 20)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(190, 10, "Финансовый отчёт", "", 1, "C", false, 0, "")
	pdf.SetFont("DejaVuSans", "", 12)
	pdf.CellFormat(190, 8, fmt.Sprintf("Период: %s – %s", start.Format("02.01.2006"), end.Format("02.01.2006")), "", 1, "C", false, 0, "")
	pdf.CellFormat(190, 8, fmt.Sprintf("Сформировано: %s", time.Now().Format("02.01.2006 15:04")), "", 1, "C", false, 0, "")
	pdf.Ln(10)

	var (
		totalIncome, totalExpense float64
		incomeDetails             = make(map[string]float64)
		expenseDetails            = make(map[string]float64)
		incomeTrend               []chart.Value
		expenseTrend              []chart.Value
		balanceTrend              []chart.Value
		balanceByDate             = make(map[string]float64)
	)

	dates := map[string]bool{}
	for _, t := range transactions {
		dateStr := t.Date.Format("2006-01-02")
		cat := removeEmoji(t.CategoryName)
		if cat == "" {
			cat = "Неизвестно"
		}
		if t.Amount > 0 {
			totalIncome += t.Amount
			incomeDetails[cat] += t.Amount
		} else {
			amount := -t.Amount
			totalExpense += amount
			expenseDetails[cat] += amount
		}
		balanceByDate[dateStr] += t.Amount
		dates[dateStr] = true
	}

	var dateList []string
	for date := range dates {
		dateList = append(dateList, date)
	}
	sort.Strings(dateList)

	var incomeSum, expenseSum, balanceSum float64
	for _, d := range dateList {
		dayBalance := balanceByDate[d]
		if dayBalance > 0 {
			incomeSum += dayBalance
		} else {
			expenseSum += -dayBalance
		}
		balanceSum += dayBalance
		incomeTrend = append(incomeTrend, chart.Value{Label: d, Value: incomeSum})
		expenseTrend = append(expenseTrend, chart.Value{Label: d, Value: expenseSum})
		balanceTrend = append(balanceTrend, chart.Value{Label: d, Value: balanceSum})
	}

	pdf.SetFont("DejaVuSans", "B", 16)
	pdf.CellFormat(190, 10, "Общая статистика", "", 1, "L", false, 0, "")
	pdf.SetFont("DejaVuSans", "", 12)
	pdf.CellFormat(190, 8, fmt.Sprintf("Общий доход: %.2f ₽", totalIncome), "", 1, "L", false, 0, "")
	pdf.CellFormat(190, 8, fmt.Sprintf("Общий расход: %.2f ₽", totalExpense), "", 1, "L", false, 0, "")
	pdf.CellFormat(190, 8, fmt.Sprintf("Баланс: %.2f ₽", totalIncome-totalExpense), "", 1, "L", false, 0, "")
	pdf.Ln(10)

	startY := pdf.GetY()
	incomeLine, _ := rg.generateLineChart(incomeTrend, drawing.ColorFromHex("5A9BD5"))
	expenseLine, _ := rg.generateLineChart(expenseTrend, drawing.ColorFromHex("ED7D31"))
	rg.addImageToPDF(pdf, incomeLine, "", 10, startY, 90, 45)
	rg.addImageToPDF(pdf, expenseLine, "", 110, startY, 90, 45)

	pdf.SetY(startY + 50)
	balanceLine, _ := rg.generateLineChart(balanceTrend, drawing.ColorFromHex("70AD47"))
	rg.addImageToPDF(pdf, balanceLine, "", 10, pdf.GetY(), 190, 45)

	pdf.SetY(pdf.GetY() + 50)
	pdf.Ln(5)

	pdf.SetFont("DejaVuSans", "B", 14)
	pdf.CellFormat(190, 10, "Распределение по категориям", "", 1, "L", false, 0, "")
	pdf.Ln(3)

	yStart := pdf.GetY()

	if len(incomeDetails) > 0 {
		incomeChart, legendIncome, colorsIncome := rg.generatePieWithLegend(incomeDetails)
		rg.addImageToPDF(pdf, incomeChart, "", 10, yStart, 90, 60)
		rg.addLegendWithColor(pdf, legendIncome, colorsIncome, 10, yStart+62)
	}

	if len(expenseDetails) > 0 {
		expenseChart, legendExpense, colorsExpense := rg.generatePieWithLegend(expenseDetails)
		rg.addImageToPDF(pdf, expenseChart, "", 110, yStart, 90, 60)
		rg.addLegendWithColor(pdf, legendExpense, colorsExpense, 110, yStart+62)
	}

	pdf.SetY(yStart + 90)
	pdf.Ln(10)

	pdf.SetFont("DejaVuSans", "B", 14)
	pdf.CellFormat(190, 10, "Детализация по категориям", "", 1, "L", false, 0, "")
	pdf.SetFont("DejaVuSans", "", 11)
	pdf.Ln(3)

	if len(incomeDetails) > 0 {
		pdf.CellFormat(190, 7, "Доходы:", "", 1, "L", false, 0, "")
		total := sum(incomeDetails)
		for _, cat := range sortedKeys(incomeDetails) {
			amount := incomeDetails[cat]
			percent := amount / total * 100
			pdf.CellFormat(190, 6, fmt.Sprintf("  • %-20s %.2f ₽ (%.0f%%)", cat, amount, percent), "", 1, "L", false, 0, "")
		}
		pdf.Ln(4)
	}

	if len(expenseDetails) > 0 {
		pdf.CellFormat(190, 7, "Расходы:", "", 1, "L", false, 0, "")
		total := sum(expenseDetails)
		for _, cat := range sortedKeys(expenseDetails) {
			amount := expenseDetails[cat]
			percent := amount / total * 100
			pdf.CellFormat(190, 6, fmt.Sprintf("  • %-20s %.2f ₽ (%.0f%%)", cat, amount, percent), "", 1, "L", false, 0, "")
		}
	}

	pdf.Ln(10)

	pdf.SetFont("DejaVuSans", "B", 14)
	pdf.CellFormat(190, 10, "Автоматический анализ", "", 1, "L", false, 0, "")
	pdf.SetFont("DejaVuSans", "", 12)
	pdf.MultiCell(190, 7, generateInsights(totalIncome, totalExpense, incomeDetails, expenseDetails), "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("ошибка генерации PDF: %v", err)
	}

	return buf.Bytes(), nil
}

func (rg *ReportGenerator) generateLineChart(data []chart.Value, color drawing.Color) ([]byte, error) {
	graph := chart.Chart{
		Width:  600,
		Height: 200,
		XAxis:  chart.XAxis{Style: chart.Style{FontSize: 8}},
		YAxis:  chart.YAxis{Style: chart.Style{FontSize: 8}},
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: extractX(data),
				YValues: extractY(data),
				Style: chart.Style{
					StrokeColor: color,
					FillColor:   drawing.ColorTransparent,
				},
			},
		},
	}
	var buf bytes.Buffer
	err := graph.Render(chart.PNG, &buf)
	return buf.Bytes(), err
}

func (rg *ReportGenerator) generatePieWithLegend(data map[string]float64) ([]byte, []string, []color.Color) {
	var values []chart.Value
	var legend []string
	var colors []color.Color

	keys := sortedKeys(data)
	total := sum(data)

	for i, k := range keys {
		val := data[k]
		percent := val / total * 100
		c := chart.GetDefaultColor(i)
		values = append(values, chart.Value{
			Value: val,
			Label: "",
			Style: chart.Style{FillColor: c},
		})
		legend = append(legend, fmt.Sprintf("%s – %.2f ₽ (%.0f%%)", k, val, percent))
		colors = append(colors, c)
	}

	if len(values) == 1 {
		values[0].Label = keys[0]
	}

	graph := chart.PieChart{
		Width:      300,
		Height:     200,
		Values:     values,
		Canvas:     chart.Style{Padding: chart.Box{Top: 2, Left: 2, Right: 2, Bottom: 2}},
		Background: chart.Style{Padding: chart.BoxZero},
	}

	var buf bytes.Buffer
	graph.Render(chart.PNG, &buf)
	return buf.Bytes(), legend, colors
}

func (rg *ReportGenerator) addLegendWithColor(pdf *gofpdf.Fpdf, items []string, colors []color.Color, x, y float64) {
	pdf.SetXY(x, y)
	pdf.SetFont("DejaVuSans", "", 9)
	for i, item := range items {
		r, g, b, _ := colors[i].RGBA()
		pdf.SetFillColor(int(r>>8), int(g>>8), int(b>>8))
		pdf.Rect(x, pdf.GetY(), 4, 4, "F")
		pdf.SetXY(x+6, pdf.GetY()-1)
		pdf.CellFormat(90, 5, item, "", 1, "L", false, 0, "")
	}
}

func (rg *ReportGenerator) addImageToPDF(pdf *gofpdf.Fpdf, img []byte, _ string, x, y, w, h float64) {
	tmpfile, err := os.CreateTemp("", "chart*.png")
	if err != nil {
		return
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Write(img)
	tmpfile.Close()
	options := gofpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptions(tmpfile.Name(), options)
	pdf.ImageOptions(tmpfile.Name(), x, y, w, h, false, options, 0, "")
}

func sortedKeys(m map[string]float64) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func extractX(data []chart.Value) []float64 {
	var xs []float64
	for i := range data {
		xs = append(xs, float64(i))
	}
	return xs
}

func extractY(data []chart.Value) []float64 {
	var ys []float64
	for _, v := range data {
		ys = append(ys, v.Value)
	}
	return ys
}

func sum(m map[string]float64) float64 {
	var total float64
	for _, v := range m {
		total += v
	}
	return total
}

func removeEmoji(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func generateInsights(income, expense float64, incomeCat, expenseCat map[string]float64) string {
	balance := income - expense
	direction := "Положительный"
	if balance < 0 {
		direction = "Отрицательный"
	}

	topIncome := topCategory(incomeCat)
	topExpense := topCategory(expenseCat)

	return fmt.Sprintf(
		"Ваш баланс за период составил: %.2f ₽ (%s).\n"+
			"Основной источник дохода: %s.\n"+
			"Основная статья расходов: %s.\n"+
			"Рекомендуем обратить внимание на контроль расходов в наиболее активной категории.",
		balance, direction, topIncome, topExpense,
	)
}

func topCategory(data map[string]float64) string {
	var max float64
	var name string
	for k, v := range data {
		if v > max {
			max = v
			name = k
		}
	}
	return name
}
