package handlers

import (
	"bytes"
	"fmt"
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

	var incomeSum, expenseSum float64
	for _, d := range dateList {
		balance := balanceByDate[d]
		if balance > 0 {
			incomeSum += balance
			incomeTrend = append(incomeTrend, chart.Value{Label: d, Value: incomeSum})
		} else {
			expenseSum += -balance
			expenseTrend = append(expenseTrend, chart.Value{Label: d, Value: expenseSum})
		}
	}

	pdf.SetFont("DejaVuSans", "B", 16)
	pdf.CellFormat(190, 10, "Общая статистика", "", 1, "L", false, 0, "")
	pdf.SetFont("DejaVuSans", "", 12)
	pdf.CellFormat(190, 8, fmt.Sprintf("Общий доход: %.2f ₽", totalIncome), "", 1, "L", false, 0, "")
	pdf.CellFormat(190, 8, fmt.Sprintf("Общий расход: %.2f ₽", totalExpense), "", 1, "L", false, 0, "")
	pdf.CellFormat(190, 8, fmt.Sprintf("Баланс: %.2f ₽", totalIncome-totalExpense), "", 1, "L", false, 0, "")
	pdf.Ln(10)

	startY := pdf.GetY()
	incomeLine, _ := rg.generateLineChart(incomeTrend, "Доходы", drawing.ColorFromHex("5A9BD5"))
	expenseLine, _ := rg.generateLineChart(expenseTrend, "Расходы", drawing.ColorFromHex("ED7D31"))
	rg.addImageToPDF(pdf, incomeLine, "", 10, startY, 90, 50)
	rg.addImageToPDF(pdf, expenseLine, "", 110, startY, 90, 50)
	pdf.SetY(startY + 55)
	pdf.Ln(5)

	pdf.SetFont("DejaVuSans", "B", 14)
	pdf.CellFormat(190, 10, "Распределение по категориям", "", 1, "L", false, 0, "")
	pdf.Ln(3)

	if len(incomeDetails) > 0 {
		incomeChart, legend := rg.generatePieWithLegend(incomeDetails, drawing.ColorFromHex("5A9BD5"))
		rg.addImageToPDF(pdf, incomeChart, "Доходы", 10, pdf.GetY(), 90, 60)
		rg.addLegend(pdf, legend, 10, pdf.GetY()+62)
	}

	if len(expenseDetails) > 0 {
		expenseChart, legend := rg.generatePieWithLegend(expenseDetails, drawing.ColorFromHex("ED7D31"))
		rg.addImageToPDF(pdf, expenseChart, "Расходы", 110, pdf.GetY()-62, 90, 60)
		rg.addLegend(pdf, legend, 110, pdf.GetY()+2)
	}
	pdf.Ln(70)

	pdf.SetFont("DejaVuSans", "B", 14)
	pdf.CellFormat(190, 10, "Детализация по категориям", "", 1, "L", false, 0, "")
	pdf.SetFont("DejaVuSans", "", 12)

	if len(incomeDetails) > 0 {
		pdf.CellFormat(190, 8, "Доходы:", "", 1, "L", false, 0, "")
		for _, cat := range sortedKeys(incomeDetails) {
			pdf.CellFormat(190, 6, fmt.Sprintf("- %s: %.2f ₽", cat, incomeDetails[cat]), "", 1, "L", false, 0, "")
		}
		pdf.Ln(5)
	}
	if len(expenseDetails) > 0 {
		pdf.CellFormat(190, 8, "Расходы:", "", 1, "L", false, 0, "")
		for _, cat := range sortedKeys(expenseDetails) {
			pdf.CellFormat(190, 6, fmt.Sprintf("- %s: %.2f ₽", cat, expenseDetails[cat]), "", 1, "L", false, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("ошибка генерации PDF: %v", err)
	}
	return buf.Bytes(), nil
}

func (rg *ReportGenerator) generateLineChart(data []chart.Value, title string, lineColor drawing.Color) ([]byte, error) {
	graph := chart.Chart{
		Width:  600,
		Height: 200,
		XAxis:  chart.XAxis{Style: chart.Style{FontSize: 8}},
		YAxis:  chart.YAxis{Style: chart.Style{FontSize: 8}},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    title,
				XValues: extractX(data),
				YValues: extractY(data),
				Style: chart.Style{
					StrokeColor: lineColor,
					FillColor:   drawing.ColorTransparent,
				},
			},
		},
	}
	var buf bytes.Buffer
	err := graph.Render(chart.PNG, &buf)
	return buf.Bytes(), err
}

func (rg *ReportGenerator) generatePieWithLegend(data map[string]float64, baseColor drawing.Color) ([]byte, []string) {
	var values []chart.Value
	var legend []string
	keys := sortedKeys(data)
	total := 0.0
	for _, v := range data {
		total += v
	}
	for i, k := range keys {
		val := data[k]
		percent := val / total * 100
		legend = append(legend, fmt.Sprintf("%s – %.2f ₽ (%.0f%%)", k, val, percent))
		values = append(values, chart.Value{
			Value: val,
			Label: "",
			Style: chart.Style{FillColor: chart.GetDefaultColor(i)},
		})
	}
	graph := chart.PieChart{
		Width:  300,
		Height: 200,
		Values: values,
	}
	var buf bytes.Buffer
	graph.Render(chart.PNG, &buf)
	return buf.Bytes(), legend
}

func (rg *ReportGenerator) addLegend(pdf *gofpdf.Fpdf, items []string, x, y float64) {
	pdf.SetXY(x, y)
	pdf.SetFont("DejaVuSans", "", 10)
	for _, item := range items {
		pdf.CellFormat(90, 5, item, "", 1, "L", false, 0, "")
	}
}

func (rg *ReportGenerator) addImageToPDF(pdf *gofpdf.Fpdf, img []byte, title string, x, y, w, h float64) {
	tmpfile, err := os.CreateTemp("", "chart*.png")
	if err != nil {
		return
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Write(img)
	tmpfile.Close()
	if title != "" {
		pdf.SetFont("DejaVuSans", "B", 12)
		pdf.SetXY(x, y-6)
		pdf.CellFormat(w, 6, title, "", 0, "C", false, 0, "")
	}
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

func removeEmoji(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
