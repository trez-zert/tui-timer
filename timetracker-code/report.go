package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Config struct {
	WeeklyTarget float64   `json:"weekly_target"`
	NoGoal       bool      `json:"no_goal"`
	ClockColor   string    `json:"clock_color"`
	ClockMode    int       `json:"clock_mode"`
}

type entry struct {
	date     time.Time
	duration time.Duration
	comment  string
}

type reportData struct {
	daily   map[string]map[string]time.Duration
	weekly  map[string]map[string]time.Duration
	monthly map[string]map[string]time.Duration
	yearly  map[string]map[string]time.Duration

	dailyKeys   []string
	weeklyKeys  []string
	monthlyKeys []string
	yearlyKeys  []string
}

type sessionState int

const (
	stateReport sessionState = iota
	stateSettings
	stateOnboarding
)

type model struct {
	state      sessionState
	data       reportData
	config     Config
	setupInput textinput.Model
	cursor     int
	err        error
}

func (m model) Init() tea.Cmd {
	if m.state == stateOnboarding {
		return textinput.Blink
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.state == stateOnboarding || m.state == stateSettings {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "esc":
				if m.state == stateSettings {
					m.state = stateReport
					return m, nil
				}
			case "n":
				if m.state == stateOnboarding || m.state == stateSettings {
					m.config.NoGoal = true
					m.config.WeeklyTarget = 0
					saveConfig(m.config)
					m.state = stateReport
					return m, nil
				}
			case "enter":
				valStr := m.setupInput.Value()
				if valStr == "" && m.state == stateSettings {
					m.state = stateReport
					return m, nil
				}
				val, err := strconv.ParseFloat(valStr, 64)
				if err != nil || val <= 0 {
					m.err = fmt.Errorf("enter a positive number")
					return m, nil
				}
				m.config.WeeklyTarget = val
				m.config.NoGoal = false
				saveConfig(m.config)
				m.state = stateReport
				m.err = nil
				return m, nil
			}
		}
		m.setupInput, cmd = m.setupInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case ",":
			m.state = stateSettings
			m.setupInput.SetValue(fmt.Sprintf("%.1f", m.config.WeeklyTarget))
			m.setupInput.Focus()
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			m.cursor++
		}
	}
	return m, nil
}

func renderProgressBar(current time.Duration, target time.Duration, width int) string {
	if target <= 0 {
		return ""
	}
	ratio := float64(current) / float64(target)
	if ratio > 1.0 {
		ratio = 1.0
	}

	filled := int(ratio * float64(width))
	empty := width - filled

	barColor := "2" // Green
	if ratio < 0.5 {
		barColor = "1" // Red
	} else if ratio < 0.9 {
		barColor = "3" // Yellow
	}

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(barColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	return "[" + barStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty)) + "]"
}

func (m model) View() string {
	if m.state == stateOnboarding {
		return lipgloss.NewStyle().Padding(1, 2).Render(
			fmt.Sprintf("Welcome! Let's set your goals.\n\nWeekly Target Hours: %s\n\n(Enter a number, 'n' for no goal, or q to quit)", m.setupInput.View()),
		)
	}

	if m.state == stateSettings {
		return lipgloss.NewStyle().Padding(1, 2).Render(
			fmt.Sprintf("Settings: Weekly Hour Target\n\nWeekly Target Hours: %s\n\n(Enter new target, 'n' to disable goals, esc to cancel)", m.setupInput.View()),
		)
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder
	title := "Time Tracking Report"
	if !m.config.NoGoal {
		title = fmt.Sprintf("Time Tracking Report (Goal: %.1fh/week)", m.config.WeeklyTarget)
	}
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render(title) + "\n\n")

	// Targets
	weeklyTarget := time.Duration(m.config.WeeklyTarget * float64(time.Hour))
	dailyTarget := weeklyTarget / 5
	monthlyTarget := weeklyTarget * 52 / 12
	yearlyTarget := weeklyTarget * 52

	// Daily
	dailyTitle := "DAILY TOTALS"
	if !m.config.NoGoal {
		dailyTitle = fmt.Sprintf("DAILY TOTALS (Target: %.1fh)", float64(dailyTarget)/float64(time.Hour))
	}
	s.WriteString(lipgloss.NewStyle().Underline(true).Render(dailyTitle) + "\n")
	for _, k := range m.data.dailyKeys {
		total := time.Duration(0)
		for _, d := range m.data.daily[k] {
			total += d
		}
		bar := ""
		if !m.config.NoGoal {
			bar = " " + renderProgressBar(total, dailyTarget, 20)
		}
		s.WriteString(fmt.Sprintf("%s: %s%s\n", k, total.Round(time.Minute), bar))
		for comment, dur := range m.data.daily[k] {
			s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur.Round(time.Minute)))
		}
	}

	// Weekly
	weeklyTitle := "WEEKLY TOTALS"
	if !m.config.NoGoal {
		weeklyTitle = fmt.Sprintf("WEEKLY TOTALS (Target: %.1fh)", float64(weeklyTarget)/float64(time.Hour))
	}
	s.WriteString("\n" + lipgloss.NewStyle().Underline(true).Render(weeklyTitle) + "\n")
	for _, k := range m.data.weeklyKeys {
		total := time.Duration(0)
		for _, d := range m.data.weekly[k] {
			total += d
		}
		bar := ""
		if !m.config.NoGoal {
			bar = " " + renderProgressBar(total, weeklyTarget, 20)
		}
		s.WriteString(fmt.Sprintf("%s: %s%s\n", k, total.Round(time.Minute), bar))
		for comment, dur := range m.data.weekly[k] {
			s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur.Round(time.Minute)))
		}
	}

	// Monthly
	monthlyTitle := "MONTHLY TOTALS"
	if !m.config.NoGoal {
		monthlyTitle = fmt.Sprintf("MONTHLY TOTALS (Target: ~%.1fh)", float64(monthlyTarget)/float64(time.Hour))
	}
	s.WriteString("\n" + lipgloss.NewStyle().Underline(true).Render(monthlyTitle) + "\n")
	for _, k := range m.data.monthlyKeys {
		total := time.Duration(0)
		for _, d := range m.data.monthly[k] {
			total += d
		}
		bar := ""
		if !m.config.NoGoal {
			bar = " " + renderProgressBar(total, monthlyTarget, 20)
		}
		s.WriteString(fmt.Sprintf("%s: %s%s\n", k, total.Round(time.Minute), bar))
		for comment, dur := range m.data.monthly[k] {
			s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur.Round(time.Minute)))
		}
	}

	// Yearly
	yearlyTitle := "YEARLY TOTALS"
	if !m.config.NoGoal {
		yearlyTitle = fmt.Sprintf("YEARLY TOTALS (Target: %.0fh)", float64(yearlyTarget)/float64(time.Hour))
	}
	s.WriteString("\n" + lipgloss.NewStyle().Underline(true).Render(yearlyTitle) + "\n")
	for _, k := range m.data.yearlyKeys {
		total := time.Duration(0)
		for _, d := range m.data.yearly[k] {
			total += d
		}
		bar := ""
		if !m.config.NoGoal {
			bar = " " + renderProgressBar(total, yearlyTarget, 20)
		}
		s.WriteString(fmt.Sprintf("%s: %s%s\n", k, total.Round(time.Minute), bar))
		for comment, dur := range m.data.yearly[k] {
			s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur.Round(time.Minute)))
		}
	}

	s.WriteString("\n(q)uit, (,) settings")

	return lipgloss.NewStyle().Padding(1, 2).Render(s.String())
}

func parseLog() (reportData, error) {
	exePath, err := os.Executable()
	if err != nil {
		return reportData{}, err
	}
	logsDir := filepath.Join(filepath.Dir(exePath), "logs")

	data := reportData{
		daily:   make(map[string]map[string]time.Duration),
		weekly:  make(map[string]map[string]time.Duration),
		monthly: make(map[string]map[string]time.Duration),
		yearly:  make(map[string]map[string]time.Duration),
	}

	err = filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			parseFile(path, &data)
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return reportData{}, err
	}

	for k := range data.daily {
		data.dailyKeys = append(data.dailyKeys, k)
	}
	sort.Strings(data.dailyKeys)

	for k := range data.weekly {
		data.weeklyKeys = append(data.weeklyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.weeklyKeys)))

	for k := range data.monthly {
		data.monthlyKeys = append(data.monthlyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.monthlyKeys)))

	for k := range data.yearly {
		data.yearlyKeys = append(data.yearlyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.yearlyKeys)))

	return data, nil
}

func parseFile(path string, data *reportData) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") || strings.Contains(line, "| Date |") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			continue
		}

		dateStr := strings.TrimSpace(parts[1])
		durStr := strings.TrimSpace(parts[4])
		comment := strings.TrimSpace(parts[5])

		date, _ := time.Parse("2006-01-02", dateStr)
		duration, _ := time.ParseDuration(durStr)

		if data.daily[dateStr] == nil {
			data.daily[dateStr] = make(map[string]time.Duration)
		}
		data.daily[dateStr][comment] += duration

		year, week := date.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", year, week)
		if data.weekly[weekKey] == nil {
			data.weekly[weekKey] = make(map[string]time.Duration)
		}
		data.weekly[weekKey][comment] += duration

		monthKey := date.Format("2006-01")
		if data.monthly[monthKey] == nil {
			data.monthly[monthKey] = make(map[string]time.Duration)
		}
		data.monthly[monthKey][comment] += duration

		yearKey := date.Format("2006")
		if data.yearly[yearKey] == nil {
			data.yearly[yearKey] = make(map[string]time.Duration)
		}
		data.yearly[yearKey][comment] += duration
	}
}

func loadConfig() (Config, bool) {
	exePath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")

	f, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, true
	}

	var cfg Config
	err = json.Unmarshal(f, &cfg)
	if err != nil {
		return Config{}, true
	}
	
	// If config exists but neither NoGoal nor WeeklyTarget are set, trigger setup
	if !cfg.NoGoal && cfg.WeeklyTarget == 0 {
		return cfg, true
	}

	return cfg, false
}

func saveConfig(cfg Config) error {
	exePath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func main() {
	cfg, needsOnboarding := loadConfig()
	
	state := stateReport
	if needsOnboarding {
		state = stateOnboarding
	}

	setupInput := textinput.New()
	setupInput.Placeholder = "e.g. 40"
	if needsOnboarding {
		setupInput.Focus()
	}

	data, errParse := parseLog()
	if errParse != nil && !os.IsNotExist(errParse) {
		log.Fatal(errParse)
	}

	p := tea.NewProgram(model{
		state:      state,
		data:       data,
		config:     cfg,
		setupInput: setupInput,
		err:        errParse,
	})

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
