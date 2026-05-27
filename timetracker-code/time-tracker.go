package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"time-tracker/pkg/timedata"

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

type sessionState int

const (
	stateSetup sessionState = iota
	stateTracking
	stateSettings
	stateChangeTask
)

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
	originalComment  string

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
	dayEntries   []timedata.TimeEntry
	dayCursor    int
	colCursor    int // 0: Start, 1: End, 2: Comment
	isEditing    bool
	editInput    textinput.Model

	termWidth  int
	termHeight int

	confirmNegative bool
	config          timedata.Config
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

// --- Helpers ---

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
	comment := m.commentInput.Value()
	if comment == "" {
		comment = "work"
	}
	duration := m.endTime.Sub(m.startTime) - m.pausedDuration
	timedata.LogTimeEntry(m.startTime, m.endTime, m.pausedDuration, comment)
	m.updateRecentComments(comment)

	_ = duration
}

func (m *model) updateRecentComments(newComment string) {
	existing := timedata.GetRecentComments()
	updated := timedata.UpdateRecentComments(newComment, existing)
	m.suggestions = updated
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

// --- Initial Model ---

func initialModel() model {
	cfg, onboarding := timedata.LoadConfig()
	s := textinput.New()
	s.Placeholder = "Start Time (HH:MM or blank for now)"
	s.Focus()
	e := textinput.New()
	e.Placeholder = "End Time (HH:MM for manual log)"
	c := textinput.New()
	c.Placeholder = "Comment (defaults to 'work')"
	ct := textinput.New()
	ct.Placeholder = "New task description"
	suggestions := timedata.GetRecentComments()
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
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		return m, nil
	case reportData:
		m.reportData = msg
		m.repCursor = 0
		return m, nil
	case []timedata.TimeEntry:
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
			timedata.SaveConfig(m.config)
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
			timedata.SaveConfig(m.config)
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
					m.commentInput.SetValue(m.originalComment)
					m.suggestionIndex = -1
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
					if m.suggestionIndex >= 0 {
						m.commentInput.SetValue(m.filtered[m.suggestionIndex])
					} else {
						m.commentInput.SetValue(m.filtered[0])
					}
					m.updateFiltered("")
					return m, nil
				}
				m.suggestionIndex = (m.suggestionIndex + 1) % len(m.filtered)
				m.commentInput.SetValue(m.filtered[m.suggestionIndex])
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
				if !timedata.ValidateTimeFormat(val) {
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
			if !timedata.ValidateTimeFormat(valStart) {
				m.err = fmt.Errorf("invalid start time")
				m.focusIndex = 0
				m.updateFocus()
				return m, nil
			}
			if valStart == "" {
				m.startTime = time.Now()
			} else {
				m.startTime, _ = timedata.ParseTime(valStart)
			}
			valEnd := m.endTimeInput.Value()
			if valEnd != "" {
				if !timedata.ValidateTimeFormat(valEnd) {
					m.err = fmt.Errorf("invalid end time")
					m.focusIndex = 1
					m.updateFocus()
					return m, nil
				}
				m.endTime, _ = timedata.ParseTime(valEnd)
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
		newVal := m.commentInput.Value()
		if newVal != prevVal {
			if m.suggestionIndex == -1 {
				m.originalComment = newVal
			}
			m.updateFiltered(newVal)
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
			if m.state != stateTracking {
				break
			}
			m.state = stateChangeTask
			if m.isPaused {
				m.endTime = m.pauseStart
			} else {
				m.endTime = time.Now()
			}
			m.changeTaskInput.SetValue("")
			m.originalComment = ""
			m.changeTaskInput.Focus()
			m.updateFiltered("")
			return m, nil
		case ",":
			if m.state != stateTracking {
				break
			}
			m.state = stateSettings
			return m, tick()
		case "up":
			if m.state == stateChangeTask && len(m.filtered) > 0 {
				if m.suggestionIndex <= 0 {
					m.changeTaskInput.SetValue(m.originalComment)
					m.suggestionIndex = -1
					return m, nil
				}
				m.suggestionIndex--
				m.changeTaskInput.SetValue(m.filtered[m.suggestionIndex])
				return m, nil
			}
		case "down", "tab":
			if m.state == stateChangeTask && len(m.filtered) > 0 {
				if msg.String() == "tab" {
					if m.suggestionIndex >= 0 {
						m.changeTaskInput.SetValue(m.filtered[m.suggestionIndex])
					} else {
						m.changeTaskInput.SetValue(m.filtered[0])
					}
					m.updateFiltered("")
					return m, nil
				}
				m.suggestionIndex = (m.suggestionIndex + 1) % len(m.filtered)
				m.changeTaskInput.SetValue(m.filtered[m.suggestionIndex])
				return m, nil
			}
		case "p":
			if m.state != stateTracking {
				break
			}
			if !m.isPaused {
				m.isPaused = true
				m.pauseStart = time.Now()
			} else {
				m.isPaused = false
				m.pausedDuration += time.Since(m.pauseStart)
				return m, tick()
			}
		case "s":
			if m.state != stateTracking {
				break
			}
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

				m.updateRecentComments(newComment)

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
		newVal := m.changeTaskInput.Value()
		if newVal != prevVal {
			if m.suggestionIndex == -1 {
				m.originalComment = newVal
			}
			m.updateFiltered(newVal)
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
			m.startTimeInput.SetValue("")
			m.endTimeInput.SetValue("")
			m.commentInput.SetValue("")
			m.focusIndex = 0
			m.updateFocus()
			return m, nil
		case "up", "k":
			if m.repCursor > 0 {
				m.repCursor--
			}
		case "down", "j":
			m.repCursor++
		case "pgup", "pageup":
			m.repCursor -= 10
			if m.repCursor < 0 {
				m.repCursor = 0
			}
		case "pgdown", "pagedown":
			m.repCursor += 10
		case ",":
			m.view = viewSettings
			m.state = stateSettings
			m.toastMsg = "fromReports"
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
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
			timedata.SaveConfig(m.config)
			if m.view == viewTimer {
				m.state = stateTracking
				return m, tick()
			}
			if (m.view == viewSettings || m.view == viewReports) && m.toastMsg == "fromReports" {
				m.view = viewReports
				m.repCursor = 0
				m.toastMsg = ""
				return m, nil
			}
			m.view = viewSetup
			return m, nil

		case "c":
			if m.view != viewReports && m.toastMsg != "fromReports" {
				for i, c := range colors {
					if c == m.config.ClockColor {
						m.config.ClockColor = colors[(i+1)%len(colors)]
						break
					}
				}
			}
		case "m":
			if m.view != viewReports && m.toastMsg != "fromReports" {
				m.config.ClockMode = (m.config.ClockMode + 1) % 3
			}

		// Report specific toggles
		case "1":
			if !m.isEditing {
				m.config.ShowDaily = !m.config.ShowDaily
			}
		case "2":
			if !m.isEditing {
				m.config.ShowWeekly = !m.config.ShowWeekly
			}
		case "3":
			if !m.isEditing {
				m.config.ShowMonthly = !m.config.ShowMonthly
			}
		case "4":
			if !m.isEditing {
				m.config.ShowYearly = !m.config.ShowYearly
			}
		case "n":
			if !m.isEditing {
				m.config.NoGoal = !m.config.NoGoal
			}
		case "t":
			if !m.isEditing {
				m.isEditing = true
				m.editInput = textinput.New()
				m.editInput.Placeholder = "Weekly Target (e.g. 40)"
				m.editInput.SetValue(fmt.Sprintf("%.1f", m.config.WeeklyTarget))
				m.editInput.Focus()
				return m, nil
			}
		case "v":
			if !m.isEditing {
				m.isEditing = true
				m.editInput = textinput.New()
				m.editInput.Placeholder = "Vacation Days (e.g. 25)"
				m.editInput.SetValue(fmt.Sprintf("%d", m.config.VacationDays))
				m.editInput.Focus()
				return m, nil
			}
		case "y":
			if !m.isEditing {
				m.isEditing = true
				m.editInput = textinput.New()
				m.editInput.Placeholder = "Yearly Target (e.g. 1900)"
				workingWeeks := 52.0 - float64(m.config.VacationDays)/5.0
				yearlyVal := m.config.WeeklyTarget * workingWeeks
				m.editInput.SetValue(fmt.Sprintf("%.1f", yearlyVal))
				m.editInput.Focus()
				return m, nil
			}
		case "enter":
			if m.isEditing {
				valStr := m.editInput.Value()
				if strings.Contains(m.editInput.Placeholder, "Vacation") {
					val, err := strconv.Atoi(valStr)
					if err == nil && val >= 0 {
						m.config.VacationDays = val
						workingWeeks := 52.0 - float64(val)/5.0
						if m.config.PreferYearly {
							m.config.WeeklyTarget = m.config.YearlyTarget / workingWeeks
						} else {
							m.config.YearlyTarget = m.config.WeeklyTarget * workingWeeks
						}
						m.isEditing = false
						return m, nil
					}
				} else if strings.Contains(m.editInput.Placeholder, "Yearly") {
					val, err := strconv.ParseFloat(valStr, 64)
					if err == nil && val > 0 {
						m.config.YearlyTarget = val
						m.config.PreferYearly = true
						workingWeeks := 52.0 - float64(m.config.VacationDays)/5.0
						m.config.WeeklyTarget = val / workingWeeks
						m.isEditing = false
						return m, nil
					}
				} else {
					val, err := strconv.ParseFloat(valStr, 64)
					if err == nil && val > 0 {
						m.config.WeeklyTarget = val
						m.config.PreferYearly = false
						workingWeeks := 52.0 - float64(m.config.VacationDays)/5.0
						m.config.YearlyTarget = val * workingWeeks
						m.isEditing = false
						return m, nil
					}
				}
			}
		}
	}

	if m.isEditing {
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
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
				if m.colCursor < 2 && !timedata.ValidateTimeFormat(newVal) {
					m.err = fmt.Errorf("invalid format")
					return m, nil
				}
				e := m.dayEntries[m.dayCursor]
				if m.colCursor == 0 {
					e.Start, _ = time.Parse("15:04:05", newVal+":00")
				} else if m.colCursor == 1 {
					e.End, _ = time.Parse("15:04:05", newVal+":00")
				} else {
					e.Comment = newVal
				}
				if e.End.Before(e.Start) && !m.confirmNegative {
					m.confirmNegative = true
					return m, nil
				}
				e.Duration = e.End.Sub(e.Start)
				timedata.CreateDayBackup(m.selectedDate)
				if m.colCursor == 2 {
					m.updateRecentComments(e.Comment)
				}

				m.dayEntries[m.dayCursor] = e
				timedata.SaveDayChanges(m.selectedDate, m.dayEntries)
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
			m.startTimeInput.SetValue("")
			m.endTimeInput.SetValue("")
			m.commentInput.SetValue("")
			m.focusIndex = 0
			m.updateFocus()
			return m, nil
		case "u":
			timedata.RestoreDayBackup(m.selectedDate)
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
		case "pgup", "pageup":
			m.dayCursor -= 10
			if m.dayCursor < -1 {
				m.dayCursor = -1
			}
		case "pgdown", "pagedown":
			m.dayCursor += 10
			if m.dayCursor >= len(m.dayEntries) {
				m.dayCursor = len(m.dayEntries) - 1
			}
		case "enter":
			if m.dayCursor > -1 && len(m.dayEntries) > 0 {
				m.isEditing = true
				m.editInput = textinput.New()
				e := m.dayEntries[m.dayCursor]
				val := ""
				if m.colCursor == 0 {
					val = e.Start.Format("15:04")
				} else if m.colCursor == 1 {
					val = e.End.Format("15:04")
				} else {
					val = e.Comment
				}
				m.editInput.SetValue(val)
				m.editInput.Focus()
				return m, nil
			}
		case "delete", "backspace":
			if m.dayCursor > -1 && len(m.dayEntries) > 0 {
				timedata.CreateDayBackup(m.selectedDate)
				m.dayEntries = append(m.dayEntries[:m.dayCursor], m.dayEntries[m.dayCursor+1:]...)
				timedata.SaveDayChanges(m.selectedDate, m.dayEntries)
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
			if !timedata.ValidateTimeFormat(valStart) || !timedata.ValidateTimeFormat(valEnd) {
				m.err = fmt.Errorf("invalid time format")
				return m, nil
			}
			st, _ := timedata.ParseTime(valStart)
			et, _ := timedata.ParseTime(valEnd)
			st = time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), st.Hour(), st.Minute(), 0, 0, st.Location())
			et = time.Date(m.selectedDate.Year(), m.selectedDate.Month(), m.selectedDate.Day(), et.Hour(), et.Minute(), 0, 0, et.Location())
			if et.Before(st) && !m.confirmNegative {
				m.confirmNegative = true
				m.err = fmt.Errorf("End time before start time. Press Enter again to confirm negative, or 'c' to cross midnight.")
				return m, nil
			}

			if m.confirmNegative && msg.String() == "c" {
				todayStart := st
				todayEnd := time.Date(st.Year(), st.Month(), st.Day(), 23, 59, 59, 0, st.Location())

				tomorrow := m.selectedDate.AddDate(0, 0, 1)
				tomorrowStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, st.Location())
				tomorrowEnd := et.AddDate(0, 0, 1)

				timedata.CreateDayBackup(m.selectedDate)
				e1 := timedata.TimeEntry{Date: m.selectedDate, Start: todayStart, End: todayEnd, Duration: todayEnd.Sub(todayStart), Comment: m.commentInput.Value()}
				e2 := timedata.TimeEntry{Date: tomorrow, Start: tomorrowStart, End: tomorrowEnd, Duration: tomorrowEnd.Sub(tomorrowStart), Comment: m.commentInput.Value()}
				if e1.Comment == "" { e1.Comment = "work" }
				if e2.Comment == "" { e2.Comment = "work" }

				m.dayEntries = append(m.dayEntries, e1, e2)
				timedata.SaveDayChanges(m.selectedDate, m.dayEntries)

				m.confirmNegative = false
				m.err = nil
				m.view = viewDay
				return m, m.loadDayEntries()
			}

			timedata.CreateDayBackup(m.selectedDate)
			newEntry := timedata.TimeEntry{
				Date:     m.selectedDate,
				Start:    st,
				End:      et,
				Duration: et.Sub(st),
				Comment:  m.commentInput.Value(),
			}
			if newEntry.Comment == "" {
				newEntry.Comment = "work"
			}

			m.updateRecentComments(newEntry.Comment)

			m.dayEntries = append(m.dayEntries, newEntry)
			timedata.SaveDayChanges(m.selectedDate, m.dayEntries)
			m.view = viewDay
			m.err = nil
			m.confirmNegative = false
			m.startTimeInput.Placeholder = "Start Time (HH:MM or blank for now)"
			return m, m.loadDayEntries()
		case "tab", "down":
			if m.focusIndex == 2 && len(m.filtered) > 0 {
				if msg.String() == "tab" {
					if m.suggestionIndex >= 0 {
						m.commentInput.SetValue(m.filtered[m.suggestionIndex])
					} else {
						m.commentInput.SetValue(m.filtered[0])
					}
					m.updateFiltered("")
					return m, nil
				}
				m.suggestionIndex = (m.suggestionIndex + 1) % len(m.filtered)
				m.commentInput.SetValue(m.filtered[m.suggestionIndex])
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
					m.commentInput.SetValue(m.originalComment)
					m.suggestionIndex = -1
					m.focusIndex--
					m.updateFocus()
					return m, nil
				}
				m.suggestionIndex--
				m.commentInput.SetValue(m.filtered[m.suggestionIndex])
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
		newVal := m.commentInput.Value()
		if newVal != prevVal {
			if m.suggestionIndex == -1 {
				m.originalComment = newVal
			}
			m.updateFiltered(newVal)
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
	s.WriteString(titleStyle.Render("tuitime") + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("v1.1.0") + "\n\n")

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

	s.WriteString("\n(tab/arrows) navigate  (enter) select/start")
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
	yearlyTarget := (weeklyTarget * 52) - (time.Duration(m.config.VacationDays) * dailyTarget)

	renderSection := func(title string, target time.Duration, keys []string, data map[string]map[string]time.Duration) {
		s.WriteString(lipgloss.NewStyle().Underline(true).Render(title) + "\n")
		for _, k := range keys {
			total := time.Duration(0)

			var comments []string
			for c := range data[k] {
				comments = append(comments, c)
				total += data[k][c]
			}
			sort.Strings(comments)

			if !m.config.NoGoal {
				bar := renderProgressBar(total, target, 20)
				targetHours := float64(target) / float64(time.Hour)
				keyStyle := lipgloss.NewStyle().Width(14)
				timeStyle := lipgloss.NewStyle().Width(12).Align(lipgloss.Right)
				targetStyle := lipgloss.NewStyle().Width(8).Align(lipgloss.Right)

				line := fmt.Sprintf("%s: %s of %s %s",
					keyStyle.Render(k),
					timeStyle.Render(total.Round(time.Minute).String()),
					targetStyle.Render(fmt.Sprintf("%.1fh", targetHours)),
					bar)
				s.WriteString(line + "\n")
			} else {
				keyStyle := lipgloss.NewStyle().Width(14)
				s.WriteString(fmt.Sprintf("%s: %s\n", keyStyle.Render(k), total.Round(time.Minute)))
			}
			for _, comment := range comments {
				dur := data[k][comment]
				s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur.Round(time.Minute)))
			}
		}
		s.WriteString("\n")
	}

	if m.config.ShowDaily {
		renderSection("DAILY TOTALS", dailyTarget, m.reportData.dailyKeys, m.reportData.daily)
	}
	if m.config.ShowWeekly {
		renderSection("WEEKLY TOTALS", weeklyTarget, m.reportData.weeklyKeys, m.reportData.weekly)
	}
	if m.config.ShowMonthly {
		renderSection("MONTHLY TOTALS", monthlyTarget, m.reportData.monthlyKeys, m.reportData.monthly)
	}
	if m.config.ShowYearly {
		renderSection("YEARLY TOTALS", yearlyTarget, m.reportData.yearlyKeys, m.reportData.yearly)
	}

	s.WriteString("(esc) back  (,) settings")

	lines := strings.Split(s.String(), "\n")

	maxViewLines := m.termHeight - 4
	if maxViewLines < 10 {
		maxViewLines = 25
	}

	if m.repCursor > len(lines)-maxViewLines {
		m.repCursor = len(lines) - maxViewLines
	}
	if m.repCursor < 0 {
		m.repCursor = 0
	}

	end := m.repCursor + maxViewLines
	if end > len(lines) {
		end = len(lines)
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines[m.repCursor:end], "\n"))
}

func (m model) viewSettings() string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Render("Settings") + "\n\n"

	if m.view == viewReports || m.toastMsg == "fromReports" {
		s += "--- Report Content ---\n"
		check := func(b bool) string {
			if b {
				return "[X]"
			}
			return "[ ]"
		}
		s += fmt.Sprintf("(1) Daily Totals:   %s\n", check(m.config.ShowDaily))
		s += fmt.Sprintf("(2) Weekly Totals:  %s\n", check(m.config.ShowWeekly))
		s += fmt.Sprintf("(3) Monthly Totals: %s\n", check(m.config.ShowMonthly))
		s += fmt.Sprintf("(4) Yearly Totals:  %s\n", check(m.config.ShowYearly))
		s += "\n--- Goals ---\n"
		s += fmt.Sprintf("(n) Disable Goals: %s\n", check(m.config.NoGoal))
		targetStr := fmt.Sprintf("%.1f", m.config.WeeklyTarget)
		if m.isEditing && strings.Contains(m.editInput.Placeholder, "Weekly Target") {
			targetStr = m.editInput.View()
		}
		s += fmt.Sprintf("(t) Weekly Target: %s hours\n", targetStr)

		workingWeeks := 52.0 - float64(m.config.VacationDays)/5.0
		yearlyVal := m.config.YearlyTarget
		if !m.config.PreferYearly {
			yearlyVal = m.config.WeeklyTarget * workingWeeks
		}
		yearlyStr := fmt.Sprintf("%.1f", yearlyVal)
		if m.isEditing && strings.Contains(m.editInput.Placeholder, "Yearly") {
			yearlyStr = m.editInput.View()
		}
		s += fmt.Sprintf("(y) Yearly Target: %s hours\n", yearlyStr)

		vacationStr := fmt.Sprintf("%d", m.config.VacationDays)
		if m.isEditing && strings.Contains(m.editInput.Placeholder, "Vacation") {
			vacationStr = m.editInput.View()
		}
		s += fmt.Sprintf("(v) Vacation Days: %s days\n", vacationStr)
	} else {
		modeName := "Large ASCII"
		if m.config.ClockMode == timedata.ClockPlain {
			modeName = "Plain Text"
		} else if m.config.ClockMode == timedata.ClockSmall {
			modeName = "Small ASCII"
		}
		s += fmt.Sprintf("(m) Clock Mode:  %s\n", modeName)
		s += fmt.Sprintf("(c) Clock Color: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color(m.config.ClockColor)).Render("██████"))
	}

	s += "\n(esc) back"
	return lipgloss.NewStyle().Padding(1, 2).Render(s)
}

func (m model) viewDay() string {
	var s strings.Builder

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
			startStr := e.Start.Format("15:04")
			endStr := e.End.Format("15:04")
			commStr := e.Comment
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
			durStr := e.Duration.Round(time.Second).String()
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
		data, err := timedata.ParseLog()
		if err != nil {
			return err
		}
		rd := reportData{
			daily:      data.Daily,
			weekly:     data.Weekly,
			monthly:    data.Monthly,
			yearly:     data.Yearly,
			dailyKeys:  data.DailyKeys,
			weeklyKeys: data.WeeklyKeys,
			monthlyKeys: data.MonthlyKeys,
			yearlyKeys: data.YearlyKeys,
		}
		return rd
	}
}

func (m model) loadDayEntries() tea.Cmd {
	return func() tea.Msg {
		entries, err := timedata.LoadDayEntries(m.selectedDate)
		if err != nil {
			return err
		}
		return entries
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

func renderClock(t string, mode timedata.ClockMode) string {
	if mode == timedata.ClockPlain {
		return t
	}
	var dMap map[rune][]string
	var rows int
	if mode == timedata.ClockLarge {
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
	for _, arg := range os.Args[1:] {
		if arg == "--web" {
			startWebServer()
			return
		}
	}

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
