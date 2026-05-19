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

// --- Types ---

type AppView int

const (
	viewSetup AppView = iota
	viewTimer
	viewReports
	viewDay
	viewDayAdd
	viewSettings
	viewOnboarding
)

type ClockMode int

const (
	ClockPlain ClockMode = iota
	ClockSmall
	ClockLarge
)

type Config struct {
	WeeklyTarget float64   `json:"weekly_target"`
	NoGoal       bool      `json:"no_goal"`
	ClockColor   string    `json:"clock_color"` // Lipgloss color string (e.g. "6")
	ClockMode    ClockMode `json:"clock_mode"`
}

type sessionState int

const (
	stateSetup sessionState = iota
	stateTracking
	stateSettings
	stateChangeTask
)

type entry struct {
	date     time.Time
	start    time.Time
	end      time.Time
	duration time.Duration
	comment  string
	original string // The raw line from the markdown file
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

type model struct {
	view  AppView
	state sessionState

	// Navigation
	menuIndex  int // 0: Timer, 1: Reports, 2: Day View, 3: Settings
	focusIndex int // -1: Menu, 0: Start, 1: End, 2: Comment

	// Setup / Manual Entry
	focusIndexManual int // Helper for manual add focus
	startTimeInput   textinput.Model
	endTimeInput     textinput.Model
	commentInput     textinput.Model
	changeTaskInput  textinput.Model

	// Timer
	startTime      time.Time
	endTime        time.Time
	pausedDuration time.Duration
	pauseStart     time.Time
	isPaused       bool

	// Suggestions
	suggestions     []string
	filtered        []string
	suggestionIndex int

	// Feedback / Toast
	toastMsg string

	// Reports
	reportData reportData
	repCursor  int

	// Day View
	selectedDate time.Time
	dayEntries   []entry
	dayCursor    int
	colCursor    int // 0: Start, 1: End, 2: Comment
	isEditing    bool
	editInput    textinput.Model

	confirmNegative bool
	config          Config
	err             error
}

// --- Globals ---

var colors = []string{"1", "2", "3", "4", "5", "6", "7"} // Red, Green, Yellow, Blue, Magenta, Cyan, White
var menuItems = []string{"Reports", "Day View", "Settings", "Quit"}

// --- Messages ---

type tickMsg time.Time
type clearToastMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func clearToast() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return clearToastMsg{}
	})
}

// --- Persistence ---

func loadConfig() (Config, bool) {
	exePath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")
	f, err := os.ReadFile(configPath)
	if err != nil {
		return Config{WeeklyTarget: 40, ClockColor: "6", ClockMode: ClockLarge}, true
	}
	var cfg Config
	json.Unmarshal(f, &cfg)
	if cfg.ClockColor == "" {
		cfg.ClockColor = "6"
	}
	needsOnboarding := !cfg.NoGoal && cfg.WeeklyTarget == 0
	return cfg, needsOnboarding
}

func saveConfig(cfg Config) {
	exePath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func getRecentComments() []string {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	cachePath := filepath.Join(filepath.Dir(exePath), "recent_comments.json")
	f, err := os.ReadFile(cachePath)
	if err != nil {
		defaults := []string{"Coding", "Meeting", "Research", "Documentation", "Testing", "Debugging", "Planning", "Support", "Review", "Design"}
		saveRecentComments(defaults)
		return defaults
	}
	var comments []string
	if err := json.Unmarshal(f, &comments); err != nil {
		return nil
	}
	return comments
}

func saveRecentComments(comments []string) {
	exePath, _ := os.Executable()
	cachePath := filepath.Join(filepath.Dir(exePath), "recent_comments.json")
	if comments == nil {
		comments = []string{}
	}
	data, _ := json.MarshalIndent(comments, "", "  ")
	os.WriteFile(cachePath, data, 0644)
}

func updateRecentComments(newComment string) {
	comments := getRecentComments()
	var updated []string
	updated = append(updated, newComment)
	for _, c := range comments {
		if c != newComment {
			updated = append(updated, c)
		}
	}
	if len(updated) > 50 {
		updated = updated[:50]
	}
	saveRecentComments(updated)
}

// --- Helpers ---

func parseTime(val string) (time.Time, error) {
	now := time.Now()
	t, err := time.Parse("15:04", val)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid format, use HH:MM")
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), nil
}

func validateTimeFormat(val string) bool {
	if val == "" {
		return true
	}
	_, err := time.Parse("15:04", val)
	return err == nil
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
	barColor := "2"
	if ratio < 0.5 {
		barColor = "1"
	} else if ratio < 0.9 {
		barColor = "3"
	}
	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(barColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return "[" + barStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty)) + "]"
}

// --- Core Logic ---

func (m *model) logToFile() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exeDir := filepath.Dir(exePath)
	yearStr := m.startTime.Format("2006")
	monthStr := m.startTime.Format("01-January")
	logDir := filepath.Join(exeDir, "logs", yearStr)
	os.MkdirAll(logDir, 0755)
	filename := filepath.Join(logDir, monthStr+".md")

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil && info.Size() == 0 {
		f.WriteString("| Date | Start | End | Duration | Comment |\n")
		f.WriteString("| --- | --- | --- | --- | --- |\n")
	}

	duration := m.endTime.Sub(m.startTime) - m.pausedDuration
	comment := m.commentInput.Value()
	if comment == "" {
		comment = "work"
	}
	updateRecentComments(comment)

	dateStr := m.startTime.Format("2006-01-02")
	startStr := m.startTime.Format("15:04:05")
	endStr := m.endTime.Format("15:04:05")
	row := fmt.Sprintf("| %s | %s | %s | %s | %s |\n", dateStr, startStr, endStr, duration.Round(time.Second), comment)
	f.WriteString(row)
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
	filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			parseFile(path, &data)
		}
		return nil
	})
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

// --- Initial Model ---

func initialModel() model {
	cfg, onboarding := loadConfig()
	s := textinput.New()
	s.Placeholder = "Start Time (HH:MM or blank for now)"
	s.Focus()
	e := textinput.New()
	e.Placeholder = "End Time (HH:MM for manual log)"
	c := textinput.New()
	c.Placeholder = "Comment (defaults to 'work')"
	ct := textinput.New()
	ct.Placeholder = "New task description"
	suggestions := getRecentComments()
	m := model{
		view:            viewSetup,
		focusIndex:      0,
		startTimeInput:  s,
		endTimeInput:    e,
		commentInput:    c,
		changeTaskInput: ct,
		suggestions:     suggestions,
		suggestionIndex: -1,
		config:          cfg,
		selectedDate:    time.Now(),
	}
	if onboarding {
		m.view = viewOnboarding
		m.startTimeInput.Placeholder = "e.g. 40"
		m.startTimeInput.SetValue("")
	}
	m.updateFiltered("")
	return m
}

func (m *model) updateFiltered(val string) {
	m.suggestionIndex = -1
	if val == "" {
		limit := 10
		if len(m.suggestions) < limit {
			limit = len(m.suggestions)
		}
		m.filtered = m.suggestions[:limit]
	} else {
		m.filtered = nil
		for _, s := range m.suggestions {
			if strings.HasPrefix(strings.ToLower(s), strings.ToLower(val)) {
				m.filtered = append(m.filtered, s)
			}
		}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.ClearScreen)
}

// --- Update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	switch msg := msg.(type) {
	case reportData:
		m.reportData = msg
		m.repCursor = 0
		return m, nil
	case []entry:
		m.dayEntries = msg
		if m.dayCursor >= len(m.dayEntries) {
			m.dayCursor = 0
		}
		if len(m.dayEntries) == 0 && m.dayCursor != -1 {
			m.dayCursor = 0
		}
		return m, nil
	case error:
		m.err = msg
		return m, nil
	case clearToastMsg:
		m.toastMsg = ""
		return m, nil
	}

	switch m.view {
	case viewOnboarding:
		return m.updateOnboarding(msg)
	case viewSetup:
		return m.updateSetup(msg)
	case viewTimer:
		return m.updateTimer(msg)
	case viewReports:
		return m.updateReports(msg)
	case viewDay, viewDayAdd:
		return m.updateDay(msg)
	case viewSettings:
		return m.updateSettings(msg)
	}
	return m, nil
}

func (m model) updateOnboarding(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			m.config.NoGoal = true
			m.config.WeeklyTarget = 0
			saveConfig(m.config)
			m.view = viewSetup
			m.startTimeInput.Placeholder = "Start Time (HH:MM or blank for now)"
			m.startTimeInput.SetValue("")
			return m, nil
		case "enter":
			val, err := strconv.ParseFloat(m.startTimeInput.Value(), 64)
			if err != nil || val <= 0 {
				m.err = fmt.Errorf("enter a positive number")
				return m, nil
			}
			m.config.WeeklyTarget = val
			m.config.NoGoal = false
			saveConfig(m.config)
			m.view = viewSetup
			m.startTimeInput.Placeholder = "Start Time (HH:MM or blank for now)"
			m.startTimeInput.SetValue("")
			m.err = nil
			return m, nil
		}
	}
	m.startTimeInput, cmd = m.startTimeInput.Update(msg)
	return m, cmd
}

func (m model) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.focusIndex != -1 {
				m.focusIndex = -1
				m.updateFocus()
				return m, nil
			}
			return m, tea.Quit
		case "left":
			if m.focusIndex == -1 {
				m.menuIndex--
				if m.menuIndex < 0 {
					m.menuIndex = len(menuItems) - 1
				}
				return m, nil
			}
		case "right":
			if m.focusIndex == -1 {
				m.menuIndex = (m.menuIndex + 1) % len(menuItems)
				return m, nil
			}
		case "up":
			if m.focusIndex == 2 && len(m.filtered) > 0 {
				if m.suggestionIndex <= 0 {
					m.focusIndex--
					m.updateFocus()
					return m, nil
				}
				m.suggestionIndex--
				m.commentInput.SetValue(m.filtered[m.suggestionIndex])
				return m, nil
			}
			m.focusIndex--
			if m.focusIndex < -1 {
				m.focusIndex = 2
			}
			m.updateFocus()
			return m, nil
		case "down", "tab":
			if m.focusIndex == 2 && len(m.filtered) > 0 {
				if msg.String() == "tab" {
					m.commentInput.SetValue(m.filtered[m.suggestionIndex])
					m.updateFiltered("")
					return m, nil
				}
				m.suggestionIndex = (m.suggestionIndex + 1) % len(m.filtered)
				return m, nil
			}
			m.focusIndex++
			if m.focusIndex > 2 {
				m.focusIndex = -1
			}
			m.updateFocus()
			return m, nil
		case "enter":
			if m.focusIndex == -1 {
				switch m.menuIndex {
				case 0: // Reports
					m.view = viewReports
					return m, m.loadReports()
				case 1: // Day View
					m.view = viewDay
					m.dayCursor = -1
					m.selectedDate = time.Now()
					return m, m.loadDayEntries()
				case 2: // Settings
					m.view = viewSettings
					m.state = stateSettings
					return m, nil
				case 3: // Quit
					return m, tea.Quit
				}
			}
			if m.focusIndex < 2 {
				val := ""
				if m.focusIndex == 0 {
					val = m.startTimeInput.Value()
				} else {
					val = m.endTimeInput.Value()
				}
				if !validateTimeFormat(val) {
					m.err = fmt.Errorf("invalid format")
					return m, nil
				}
				m.err = nil
				m.focusIndex++
				m.updateFocus()
				return m, nil
			}
			m.err = nil
			valStart := m.startTimeInput.Value()
			if !validateTimeFormat(valStart) {
				m.err = fmt.Errorf("invalid start time")
				m.focusIndex = 0
				m.updateFocus()
				return m, nil
			}
			if valStart == "" {
				m.startTime = time.Now()
			} else {
				m.startTime, _ = parseTime(valStart)
			}
			valEnd := m.endTimeInput.Value()
			if valEnd != "" {
				if !validateTimeFormat(valEnd) {
					m.err = fmt.Errorf("invalid end time")
					m.focusIndex = 1
					m.updateFocus()
					return m, nil
				}
				m.endTime, _ = parseTime(valEnd)
				if m.endTime.Before(m.startTime) && !m.confirmNegative {
					m.confirmNegative = true
					return m, nil
				}
				m.logToFile()
				m.toastMsg = "Saved!"
				m.startTimeInput.SetValue("")
				m.endTimeInput.SetValue("")
				m.commentInput.SetValue("")
				m.focusIndex = 0
				m.confirmNegative = false
				m.updateFocus()
				return m, clearToast()
			}
			if m.startTime.After(time.Now()) && !m.confirmNegative {
				m.confirmNegative = true
				return m, nil
			}
			m.view = viewTimer
			m.state = stateTracking
			m.confirmNegative = false
			return m, tick()
		}
	}
	switch m.focusIndex {
	case 0:
		m.startTimeInput, cmd = m.startTimeInput.Update(msg)
	case 1:
		m.endTimeInput, cmd = m.endTimeInput.Update(msg)
	case 2:
		prevVal := m.commentInput.Value()
		m.commentInput, cmd = m.commentInput.Update(msg)
		if m.commentInput.Value() != prevVal {
			m.updateFiltered(m.commentInput.Value())
		}
	}
	return m, cmd
}

func (m *model) updateFocus() {
	m.startTimeInput.Blur()
	m.endTimeInput.Blur()
	m.commentInput.Blur()
	switch m.focusIndex {
	case 0:
		m.startTimeInput.Focus()
	case 1:
		m.endTimeInput.Focus()
	case 2:
		m.commentInput.Focus()
		m.updateFiltered("")
	}
}

func (m model) updateTimer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state == stateSettings {
		return m.updateSettings(msg)
	}
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.state == stateChangeTask {
				m.state = stateTracking
				return m, tick()
			}
		case "t":
			if m.state == stateTracking {
				m.state = stateChangeTask
				if m.isPaused {
					m.endTime = m.pauseStart
				} else {
					m.endTime = time.Now()
				}
				m.changeTaskInput.SetValue("")
				m.changeTaskInput.Focus()
				m.updateFiltered("")
				return m, nil
			}
		case ",":
			m.state = stateSettings
			return m, tick()
		case "p":
			if !m.isPaused {
				m.isPaused = true
				m.pauseStart = time.Now()
			} else {
				m.isPaused = false
				m.pausedDuration += time.Since(m.pauseStart)
				return m, tick()
			}
		case "s":
			m.endTime = time.Now()
			if m.isPaused {
				m.pausedDuration += time.Since(m.pauseStart)
			}
			m.logToFile()
			m.view = viewSetup
			m.startTimeInput.SetValue("")
			m.endTimeInput.SetValue("")
			m.commentInput.SetValue("")
			m.focusIndex = 0
			m.isPaused = false
			m.pausedDuration = 0
			m.updateFocus()
			return m, nil
		case "enter":
			if m.state == stateChangeTask {
				newComment := m.changeTaskInput.Value()
				if newComment == "" {
					newComment = "work"
				}
				m.logToFile()
				m.startTime = m.endTime
				m.pausedDuration = 0
				m.isPaused = false
				m.commentInput.SetValue(newComment)
				m.state = stateTracking
				return m, tick()
			}
		}
	case tickMsg:
		if !m.isPaused {
			return m, tick()
		}
	}
	if m.state == stateChangeTask {
		prevVal := m.changeTaskInput.Value()
		m.changeTaskInput, cmd = m.changeTaskInput.Update(msg)
		if m.changeTaskInput.Value() != prevVal {
			m.updateFiltered(m.changeTaskInput.Value())
		}
	}
	return m, cmd
}

func (m model) updateReports(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.view = viewSetup
			return m, nil
		case "up", "k":
			if m.repCursor > 0 {
				m.repCursor--
			}
		case "down", "j":
			m.repCursor++
		case ",":
			m.view = viewSettings
			m.state = stateSettings
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if !m.isPaused {
			return m, tick()
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", ",":
			if m.view == viewTimer {
				m.state = stateTracking
				saveConfig(m.config)
				return m, tick()
			}
			m.view = viewSetup
			saveConfig(m.config)
			return m, nil
		case "c":
			for i, c := range colors {
				if c == m.config.ClockColor {
					m.config.ClockColor = colors[(i+1)%len(colors)]
					break
				}
			}
		case "m":
			m.config.ClockMode = (m.config.ClockMode + 1) % 3
		}
	}
	return m, nil
}

func (m model) updateDay(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.view == viewDayAdd {
		return m.updateDayAdd(msg)
	}
	if m.isEditing {
		var cmd tea.Cmd
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.isEditing = false
				m.confirmNegative = false
				return m, nil
			case "enter":
				newVal := m.editInput.Value()
				if m.colCursor < 2 && !validateTimeFormat(newVal) {
					m.err = fmt.Errorf("invalid format")
					return m, nil
				}
				e := m.dayEntries[m.dayCursor]
				if m.colCursor == 0 {
					e.start, _ = time.Parse("15:04:05", newVal+":00")
				} else if m.colCursor == 1 {
					e.end, _ = time.Parse("15:04:05", newVal+":00")
				} else {
					e.comment = newVal
				}
				if e.end.Before(e.start) && !m.confirmNegative {
					m.confirmNegative = true
					return m, nil
				}
				e.duration = e.end.Sub(e.start)
				m.createDayBackup()
				m.dayEntries[m.dayCursor] = e
				m.saveDayChanges()
				m.isEditing = false
				m.err = nil
				m.confirmNegative = false
				return m, m.loadDayEntries()
			}
		}
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.view = viewSetup
			return m, nil
		case "u":
			m.restoreDayBackup()
			m.toastMsg = "Undone!"
			return m, tea.Batch(m.loadDayEntries(), clearToast())
		case "a":
			m.view = viewDayAdd
			m.focusIndex = 0
			m.startTimeInput.SetValue("")
			m.startTimeInput.Placeholder = "Start Time (HH:MM for manual log)"
			m.endTimeInput.SetValue("")
			m.commentInput.SetValue("")
			m.updateFiltered("")
			m.updateFocus()
			return m, nil
		case "left":
			if m.dayCursor == -1 {
				m.selectedDate = m.selectedDate.AddDate(0, 0, -1)
				return m, m.loadDayEntries()
			}
			if m.colCursor > 0 {
				m.colCursor--
			}
		case "right":
			if m.dayCursor == -1 {
				m.selectedDate = m.selectedDate.AddDate(0, 0, 1)
				return m, m.loadDayEntries()
			}
			if m.colCursor < 2 {
				m.colCursor++
			}
		case "up", "k":
			if m.dayCursor > -1 {
				m.dayCursor--
			}
		case "down", "j":
			if m.dayCursor < len(m.dayEntries)-1 {
				m.dayCursor++
			}
		case "enter":
			if m.dayCursor > -1 && len(m.dayEntries) > 0 {
				m.isEditing = true
				m.editInput = textinput.New()
				e := m.dayEntries[m.dayCursor]
				val := ""
				if m.colCursor == 0 {
					val = e.start.Format("15:04")
				} else if m.colCursor == 1 {
					val = e.end.Format("15:04")
				} else {
					val = e.comment
				}
				m.editInput.SetValue(val)
				m.editInput.Focus()
				return m, nil
			}
		case "delete", "backspace":
			if m.dayCursor > -1 && len(m.dayEntries) > 0 {
				m.createDayBackup()
				m.dayEntries = append(m.dayEntries[:m.dayCursor], m.dayEntries[m.dayCursor+1:]...)
				m.saveDayChanges()
				if m.dayCursor >= len(m.dayEntries) && m.dayCursor > 0 {
					m.dayCursor--
				}
				return m, m.loadDayEntries()
			}
		}
	}
	return m, nil
}

func (m model) updateDayAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.view = viewDay
			m.confirmNegative = false
			m.startTimeInput.Placeholder = "Start Time (HH:MM or blank for now)"
			return m, nil
		case "enter":
			if m.focusIndex < 2 {
				m.focusIndex++
				m.updateFocus()
				return m, nil
			}
			valStart := m.startTimeInput.Value()
			valEnd := m.endTimeInput.Value()
			if valEnd == "" {
				m.err = fmt.Errorf("end time required for manual add")
				m.focusIndex = 1
				m.updateFocus()
				return m, nil
			}
			if !validateTimeFormat(valStart) || !validateTimeFormat(valEnd) {
				m.err = fmt.Errorf("invalid time format")
				return m, nil
			}
			st, _ := parseTime(valStart)
			et, _ := parseTime(valEnd)
			st = time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), st.Hour(), st.Minute(), 0, 0, st.Location())
			et = time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), et.Hour(), et.Minute(), 0, 0, et.Location())
			if et.Before(st) && !m.confirmNegative {
				m.confirmNegative = true
				return m, nil
			}
			m.createDayBackup()
			newEntry := entry{
				date:     m.selectedDate,
				start:    st,
				end:      et,
				duration: et.Sub(st),
				comment:  m.commentInput.Value(),
			}
			if newEntry.comment == "" {
				newEntry.comment = "work"
			}
			m.dayEntries = append(m.dayEntries, newEntry)
			m.saveDayChanges()
			m.view = viewDay
			m.err = nil
			m.confirmNegative = false
			m.startTimeInput.Placeholder = "Start Time (HH:MM or blank for now)"
			return m, m.loadDayEntries()
		case "tab", "down":
			if m.focusIndex == 2 && len(m.filtered) > 0 {
				if msg.String() == "tab" {
					m.commentInput.SetValue(m.filtered[m.suggestionIndex])
					m.updateFiltered("")
					return m, nil
				}
				m.suggestionIndex = (m.suggestionIndex + 1) % len(m.filtered)
				return m, nil
			}
			m.focusIndex++
			if m.focusIndex > 2 {
				m.focusIndex = 0
			}
			m.updateFocus()
			return m, nil
		case "up":
			if m.focusIndex == 2 && len(m.filtered) > 0 {
				if m.suggestionIndex <= 0 {
					m.focusIndex--
					m.updateFocus()
					return m, nil
				}
				m.suggestionIndex--
				return m, nil
			}
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = 2
			}
			m.updateFocus()
			return m, nil
		}
	}
	switch m.focusIndex {
	case 0:
		m.startTimeInput, cmd = m.startTimeInput.Update(msg)
	case 1:
		m.endTimeInput, cmd = m.endTimeInput.Update(msg)
	case 2:
		prevVal := m.commentInput.Value()
		m.commentInput, cmd = m.commentInput.Update(msg)
		if m.commentInput.Value() != prevVal {
			m.updateFiltered(m.commentInput.Value())
		}
	}
	return m, cmd
}

// --- View ---

func (m model) View() string {
	switch m.view {
	case viewOnboarding:
		return m.viewOnboarding()
	case viewSetup:
		return m.viewSetup()
	case viewTimer:
		return m.viewTimer()
	case viewReports:
		return m.viewReports()
	case viewDay, viewDayAdd:
		return m.viewDay()
	case viewSettings:
		return m.viewSettings()
	}
	return "Unknown view"
}

func (m model) viewOnboarding() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(
		fmt.Sprintf("Welcome! Let's set your goals.\n\nWeekly Target Hours: %s\n\n(Enter a number, 'n' for no goal, or ctrl+c to quit)", m.startTimeInput.View()),
	)
}

func (m model) viewSetup() string {
	var s strings.Builder
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	s.WriteString(titleStyle.Render("tuitime") + "\n\n")

	// Menu
	for i, item := range menuItems {
		style := lipgloss.NewStyle().Padding(0, 1)
		if m.focusIndex == -1 && i == m.menuIndex {
			style = style.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("5")).Bold(true)
		} else if i == m.menuIndex {
			style = style.Foreground(lipgloss.Color("5")).Underline(true)
		}
		s.WriteString(style.Render(item) + "  ")
	}
	s.WriteString("\n\n")

	modeText := "TIMER"
	if m.endTimeInput.Value() != "" {
		modeText = "MANUAL LOG"
	}
	
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Session Setup ["+modeText+"]") + "\n\n")
	s.WriteString(fmt.Sprintf("Start Time: %s\n", m.startTimeInput.View()))
	s.WriteString(fmt.Sprintf("End Time:   %s\n", m.endTimeInput.View()))
	s.WriteString(fmt.Sprintf("Comment:    %s\n", m.commentInput.View()))

	if m.focusIndex == 2 && len(m.filtered) > 0 {
		s.WriteString("\nSuggestions:\n")
		for i, f := range m.filtered {
			style := lipgloss.NewStyle()
			if i == m.suggestionIndex {
				style = style.Foreground(lipgloss.Color("205")).Bold(true)
			}
			s.WriteString(style.Render(fmt.Sprintf("  • %s", f)) + "\n")
		}
	}

	s.WriteString("\n(tab/arrows) navigate  (enter) select/start  (q)uit")
	if m.err != nil {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("\n\nError: %v", m.err)))
	}
	if m.confirmNegative {
		warn := "Warning: Start time is after end time (or in the future).\nPress Enter again to confirm."
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render("\n\n" + warn))
	}
	if m.toastMsg != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Render("\n\n" + m.toastMsg))
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(s.String())
}

func (m model) viewTimer() string {
	if m.state == stateSettings {
		return m.viewSettings()
	}
	if m.state == stateChangeTask {
		var s strings.Builder
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Render("Change Task") + "\n\n")
		s.WriteString(fmt.Sprintf("Current Task: %s\n", m.commentInput.Value()))
		s.WriteString(fmt.Sprintf("New Task:     %s\n", m.changeTaskInput.View()))
		if len(m.filtered) > 0 {
			s.WriteString("\nSuggestions:\n")
			for i, f := range m.filtered {
				style := lipgloss.NewStyle()
				if i == m.suggestionIndex {
					style = style.Foreground(lipgloss.Color("205")).Bold(true)
				}
				s.WriteString(style.Render(fmt.Sprintf("  • %s", f)) + "\n")
			}
		}
		s.WriteString("\n(enter) confirm  (esc) cancel")
		return lipgloss.NewStyle().Padding(1, 2).Render(s.String())
	}
	elapsed := time.Since(m.startTime) - m.pausedDuration
	if m.isPaused {
		elapsed = m.pauseStart.Sub(m.startTime) - m.pausedDuration
	}
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("RUNNING")
	if m.isPaused {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("PAUSED")
	}
	displayComment := m.commentInput.Value()
	if displayComment == "" {
		displayComment = "work"
	}
	h := int(elapsed.Hours())
	min := int(elapsed.Minutes()) % 60
	sec := int(elapsed.Seconds()) % 60
	timeStr := fmt.Sprintf("%02d:%02d:%02d", h, min, sec)
	clockStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.config.ClockColor)).Bold(true)
	clock := clockStyle.Render(renderClock(timeStr, m.config.ClockMode))
	s := fmt.Sprintf("Activity: %s\nStatus: %s\n\n%s\n\n(p)ause  (s)top  (t)ask  (,) settings", displayComment, status, clock)
	return lipgloss.NewStyle().Padding(1, 2).Render(s)
}

func (m model) viewReports() string {
	var s strings.Builder
	title := "Time Tracking Report"
	if !m.config.NoGoal {
		title = fmt.Sprintf("Time Tracking Report (Goal: %.1fh/week)", m.config.WeeklyTarget)
	}
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render(title) + "\n\n")
	weeklyTarget := time.Duration(m.config.WeeklyTarget * float64(time.Hour))
	dailyTarget := weeklyTarget / 5
	monthlyTarget := weeklyTarget * 52 / 12
	yearlyTarget := weeklyTarget * 52
	renderSection := func(title string, target time.Duration, keys []string, data map[string]map[string]time.Duration) {
		s.WriteString(lipgloss.NewStyle().Underline(true).Render(title) + "\n")
		for _, k := range keys {
			total := time.Duration(0)
			for _, d := range data[k] {
				total += d
			}
			bar := ""
			if !m.config.NoGoal {
				bar = " " + renderProgressBar(total, target, 20)
			}
			s.WriteString(fmt.Sprintf("%s: %s%s\n", k, total.Round(time.Minute), bar))
			for comment, dur := range data[k] {
				s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur.Round(time.Minute)))
			}
		}
		s.WriteString("\n")
	}
	renderSection("DAILY TOTALS", dailyTarget, m.reportData.dailyKeys, m.reportData.daily)
	renderSection("WEEKLY TOTALS", weeklyTarget, m.reportData.weeklyKeys, m.reportData.weekly)
	renderSection("MONTHLY TOTALS", monthlyTarget, m.reportData.monthlyKeys, m.reportData.monthly)
	renderSection("YEARLY TOTALS", yearlyTarget, m.reportData.yearlyKeys, m.reportData.yearly)
	s.WriteString("(esc) back  (,) settings")
	return lipgloss.NewStyle().Padding(1, 2).Render(s.String())
}

func (m model) viewSettings() string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Render("Settings") + "\n\n"
	modeName := "Large ASCII"
	if m.config.ClockMode == ClockPlain {
		modeName = "Plain Text"
	} else if m.config.ClockMode == ClockSmall {
		modeName = "Small ASCII"
	}
	s += fmt.Sprintf("(m) Clock Mode:  %s\n", modeName)
	s += fmt.Sprintf("(c) Clock Color: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color(m.config.ClockColor)).Render("██████"))
	s += "\n(esc) back"
	return lipgloss.NewStyle().Padding(1, 2).Render(s)
}

func (m model) viewDay() string {
	var s strings.Builder
	
	// Date Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	dateText := "Day View: " + m.selectedDate.Format("2006-01-02")
	if m.dayCursor == -1 {
		headerStyle = headerStyle.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("5"))
		dateText = "<< " + dateText + " >>"
	}
	s.WriteString(headerStyle.Render(dateText) + "\n\n")

	if m.view == viewDayAdd {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Add Entry for "+m.selectedDate.Format("2006-01-02")) + "\n\n")
		s.WriteString(fmt.Sprintf("Start Time: %s\n", m.startTimeInput.View()))
		s.WriteString(fmt.Sprintf("End Time:   %s\n", m.endTimeInput.View()))
		s.WriteString(fmt.Sprintf("Comment:    %s\n", m.commentInput.View()))
		
		if m.focusIndex == 2 && len(m.filtered) > 0 {
			s.WriteString("\nSuggestions:\n")
			for i, f := range m.filtered {
				style := lipgloss.NewStyle()
				if i == m.suggestionIndex {
					style = style.Foreground(lipgloss.Color("205")).Bold(true)
				}
				s.WriteString(style.Render(fmt.Sprintf("  • %s", f)) + "\n")
			}
		}

		s.WriteString("\n(enter) save  (esc) cancel")
		if m.err != nil {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("\n\nError: %v", m.err)))
		}
		if m.confirmNegative {
			warn := "Warning: Start time is after end time (or in the future).\nPress Enter again to confirm negative/future entry."
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render("\n\n" + warn))
		}
		return lipgloss.NewStyle().Padding(1, 2).Render(s.String())
	}
	if len(m.dayEntries) == 0 {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("No entries for this day.") + "\n")
	} else {
		for i, e := range m.dayEntries {
			style := lipgloss.NewStyle()
			if i == m.dayCursor {
				style = style.Background(lipgloss.Color("235"))
			}
			startStr := e.start.Format("15:04")
			endStr := e.end.Format("15:04")
			commStr := e.comment
			if i == m.dayCursor && m.isEditing {
				if m.colCursor == 0 {
					startStr = m.editInput.View()
				} else if m.colCursor == 1 {
					endStr = m.editInput.View()
				} else {
					commStr = m.editInput.View()
				}
			} else if i == m.dayCursor {
				selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
				if m.colCursor == 0 {
					startStr = selStyle.Render(startStr)
				} else if m.colCursor == 1 {
					endStr = selStyle.Render(endStr)
				} else {
					commStr = selStyle.Render(commStr)
				}
			}
			durStr := e.duration.Round(time.Second).String()
			s.WriteString(style.Render(fmt.Sprintf("%s - %s | %s | %s", startStr, endStr, durStr, commStr)) + "\n")
		}
	}
	if m.err != nil {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("\nError: %v", m.err)))
	}
	if m.toastMsg == "Undone!" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("\nUndone!") + "\n")
	}

	if m.isEditing && m.confirmNegative {
		warn := "Warning: Start time is after end time (or in the future).\nPress Enter again to confirm negative/future entry."
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render("\n\n" + warn))
	}

	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	s.WriteString("\n" + hintStyle.Render("Hint: Navigate to the header above to change days with left/right arrows."))

	s.WriteString("\n\n(enter) edit  (a)dd  (del) remove  (u)ndo  (esc) back")
	s.WriteString("\n(up/down) select entry  (left/right) navigate days (on header) or fields")
	return lipgloss.NewStyle().Padding(1, 2).Render(s.String())
}

func (m model) loadReports() tea.Cmd {
	return func() tea.Msg {
		data, err := parseLog()
		if err != nil {
			return err
		}
		return data
	}
}

func (m model) loadDayEntries() tea.Cmd {
	return func() tea.Msg {
		dateStr := m.selectedDate.Format("2006-01-02")
		exePath, _ := os.Executable()
		yearStr := m.selectedDate.Format("2006")
		monthStr := m.selectedDate.Format("01-January")
		logPath := filepath.Join(filepath.Dir(exePath), "logs", yearStr, monthStr+".md")
		f, err := os.Open(logPath)
		if err != nil {
			return []entry{}
		}
		defer f.Close()
		var entries []entry
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(line, "| "+dateStr+" |") {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) < 6 {
				continue
			}
			startStr := strings.TrimSpace(parts[2])
			endStr := strings.TrimSpace(parts[3])
			durStr := strings.TrimSpace(parts[4])
			comment := strings.TrimSpace(parts[5])
			st, _ := time.Parse("15:04:05", startStr)
			et, _ := time.Parse("15:04:05", endStr)
			dur, _ := time.ParseDuration(durStr)
			entries = append(entries, entry{
				date:     m.selectedDate,
				start:    st,
				end:      et,
				duration: dur,
				comment:  comment,
				original: line,
			})
		}
		return entries
	}
}

func (m *model) createDayBackup() {
	exePath, _ := os.Executable()
	yearStr := m.selectedDate.Format("2006")
	monthStr := m.selectedDate.Format("01-January")
	logPath := filepath.Join(filepath.Dir(exePath), "logs", yearStr, monthStr+".md")
	backupPath := logPath + ".bak"
	input, err := os.ReadFile(logPath)
	if err == nil {
		os.WriteFile(backupPath, input, 0644)
	}
}

func (m *model) restoreDayBackup() {
	exePath, _ := os.Executable()
	yearStr := m.selectedDate.Format("2006")
	monthStr := m.selectedDate.Format("01-January")
	logPath := filepath.Join(filepath.Dir(exePath), "logs", yearStr, monthStr+".md")
	backupPath := logPath + ".bak"
	input, err := os.ReadFile(backupPath)
	if err == nil {
		os.WriteFile(logPath, input, 0644)
	}
}

func (m model) saveDayChanges() {
	exePath, _ := os.Executable()
	yearStr := m.selectedDate.Format("2006")
	monthStr := m.selectedDate.Format("01-January")
	logPath := filepath.Join(filepath.Dir(exePath), "logs", yearStr, monthStr+".md")
	dateStr := m.selectedDate.Format("2006-01-02")
	f, err := os.Open(logPath)
	if err != nil {
		return
	}
	var otherLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "| "+dateStr+" |") {
			otherLines = append(otherLines, line)
		}
	}
	f.Close()
	var newLines []string
	for _, e := range m.dayEntries {
		row := fmt.Sprintf("| %s | %s | %s | %s | %s |",
			dateStr,
			e.start.Format("15:04:05"),
			e.end.Format("15:04:05"),
			e.duration.Round(time.Second),
			e.comment,
		)
		newLines = append(newLines, row)
	}
	f, _ = os.OpenFile(logPath, os.O_TRUNC|os.O_WRONLY, 0644)
	defer f.Close()
	headerDone := false
	for _, l := range otherLines {
		if strings.HasPrefix(l, "| Date |") {
			headerDone = true
		}
		f.WriteString(l + "\n")
	}
	if !headerDone {
		f.WriteString("| Date | Start | End | Duration | Comment |\n")
		f.WriteString("| --- | --- | --- | --- | --- |\n")
	}
	for _, l := range newLines {
		f.WriteString(l + "\n")
	}
}

var digitsLarge = map[rune][]string{
	'0': {" █████ ", "█     █", "█     █", "█     █", "█     █", "█     █", " █████ "},
	'1': {"   █   ", "  ██   ", "   █   ", "   █   ", "   █   ", "   █   ", " █████ "},
	'2': {" █████ ", "█     █", "      █", "  ████ ", " █     ", "█      ", "███████"},
	'3': {" █████ ", "█     █", "      █", "  ████ ", "      █", "█     █", " █████ "},
	'4': {"█    █ ", "█    █ ", "█    █ ", "███████", "     █ ", "     █ ", "     █ "},
	'5': {"███████", "█      ", "█      ", "██████ ", "      █", "█     █", " █████ "},
	'6': {" █████ ", "█      ", "█      ", "██████ ", "█     █", "█     █", " █████ "},
	'7': {"███████", "      █", "     █ ", "    █  ", "   █   ", "  █    ", " █     "},
	'8': {" █████ ", "█     █", "█     █", " █████ ", "█     █", "█     █", " █████ "},
	'9': {" █████ ", "█     █", "█     █", " ██████", "      █", "     █ ", " ████  "},
	':': {"       ", "  ███  ", "  ███  ", "       ", "  ███  ", "  ███  ", "       "},
}

var digitsSmall = map[rune][]string{
	'0': {" ███ ", "█   █", "█   █", "█   █", " ███ "},
	'1': {"  █  ", " ██  ", "  █  ", "  █  ", " ███ "},
	'2': {" ███ ", "    █", " ███ ", "█    ", " ███ "},
	'3': {" ███ ", "    █", " ███ ", "    █", " ███ "},
	'4': {"█   █", "█   █", " ████", "    █", "    █"},
	'5': {"█████", "█    ", "████ ", "    █", "████ "},
	'6': {" ███ ", "█    ", "████ ", "█   █", " ███ "},
	'7': {"█████", "    █", "   █ ", "  █  ", "  █  "},
	'8': {" ███ ", "█   █", " ███ ", "█   █", " ███ "},
	'9': {" ███ ", "█   █", " ████", "    █", " ███ "},
	':': {"   ", " █ ", "   ", " █ ", "   "},
}

func renderClock(t string, mode ClockMode) string {
	if mode == ClockPlain {
		return t
	}
	var dMap map[rune][]string
	var rows int
	if mode == ClockLarge {
		dMap = digitsLarge
		rows = 7
	} else {
		dMap = digitsSmall
		rows = 5
	}
	res := make([]string, rows)
	for _, r := range t {
		d, ok := dMap[r]
		if !ok {
			continue
		}
		for i := 0; i < rows; i++ {
			res[i] += d[i] + "  "
		}
	}
	return strings.Join(res, "\n")
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
