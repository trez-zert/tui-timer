package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ClockMode int

const (
	ClockPlain ClockMode = iota
	ClockSmall
	ClockLarge
)

type Config struct {
	WeeklyTarget float64   `json:"weekly_target"`
	ClockColor   string    `json:"clock_color"` // Lipgloss color string (e.g. "6")
	ClockMode    ClockMode `json:"clock_mode"`
}

type sessionState int

const (
	stateSetup sessionState = iota
	stateTracking
	stateSettings
)

type model struct {
	state      sessionState
	focusIndex int // 0: Start, 1: End, 2: Comment

	startTimeInput textinput.Model
	endTimeInput   textinput.Model
	commentInput   textinput.Model

	startTime      time.Time
	endTime        time.Time
	pausedDuration time.Duration
	pauseStart     time.Time
	isPaused       bool

	suggestions     []string
	filtered        []string
	suggestionIndex int

	config Config

	err error
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func getRecentComments() []string {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	cachePath := filepath.Join(filepath.Dir(exePath), "recent_comments.json")

	f, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
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

func loadConfig() Config {
	exePath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")
	f, err := os.ReadFile(configPath)
	if err != nil {
		return Config{WeeklyTarget: 42, ClockColor: "6", ClockMode: ClockLarge}
	}
	var cfg Config
	json.Unmarshal(f, &cfg)
	if cfg.ClockColor == "" {
		cfg.ClockColor = "6"
	}
	return cfg
}

func saveConfig(cfg Config) {
	exePath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func initialModel() model {
	cfg := loadConfig()

	s := textinput.New()
	s.Placeholder = "Start Time (HH:MM or blank for now)"
	s.Focus()

	e := textinput.New()
	e.Placeholder = "End Time (HH:MM or leave blank for live timer)"

	c := textinput.New()
	c.Placeholder = "Comment (optional, defaults to 'work')"

	suggestions := getRecentComments()
	var filtered []string
	if len(suggestions) > 0 {
		limit := 10
		if len(suggestions) < limit {
			limit = len(suggestions)
		}
		filtered = suggestions[:limit]
	}

	return model{
		state:           stateSetup,
		focusIndex:      0,
		startTimeInput:  s,
		endTimeInput:    e,
		commentInput:    c,
		suggestions:     suggestions,
		filtered:        filtered,
		suggestionIndex: -1,
		config:          cfg,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

var colors = []string{"1", "2", "3", "4", "5", "6", "7"} // Red, Green, Yellow, Blue, Magenta, Cyan, White

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.state == stateSettings {
				m.state = stateTracking
				saveConfig(m.config)
				return m, tick()
			}
		case ",": // Settings
			if m.state == stateTracking {
				m.state = stateSettings
				return m, tick()
			}
		case "c": // Cycle colors
			if m.state == stateSettings {
				for i, c := range colors {
					if c == m.config.ClockColor {
						m.config.ClockColor = colors[(i+1)%len(colors)]
						break
					}
				}
				return m, nil
			}
		case "m": // Cycle modes
			if m.state == stateSettings {
				m.config.ClockMode = (m.config.ClockMode + 1) % 3
				return m, nil
			}
		case "enter":
			if m.state == stateSetup {
				if m.focusIndex < 2 {
					m.focusIndex++
					return m.updateFocus(), nil
				}
				valStart := m.startTimeInput.Value()
				if valStart == "" {
					m.startTime = time.Now()
				} else {
					t, err := parseTime(valStart)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.startTime = t
				}

				valEnd := m.endTimeInput.Value()
				if valEnd != "" {
					t, err := parseTime(valEnd)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.endTime = t
					m.logToFile()
					return m, tea.Quit
				}

				m.state = stateTracking
				return m, tick()
			}
		case "tab", "down":
			if m.state == stateSetup {
				if m.focusIndex == 2 && len(m.filtered) > 0 {
					if msg.String() == "tab" && m.suggestionIndex == -1 {
						m.commentInput.SetValue(m.filtered[0])
						m.suggestionIndex = -1
						// Refresh filtered to top 10 after selection
						m.updateFiltered("")
						return m, nil
					}
					m.suggestionIndex = (m.suggestionIndex + 1) % len(m.filtered)
					m.commentInput.SetValue(m.filtered[m.suggestionIndex])
					return m, nil
				}
				m.focusIndex = (m.focusIndex + 1) % 3
				return m.updateFocus(), nil
			}
		case "up":
			if m.state == stateSetup {
				if m.focusIndex == 2 && len(m.filtered) > 0 {
					m.suggestionIndex--
					if m.suggestionIndex < 0 {
						m.suggestionIndex = len(m.filtered) - 1
					}
					m.commentInput.SetValue(m.filtered[m.suggestionIndex])
					return m, nil
				}
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = 2
				}
				return m.updateFocus(), nil
			}
		case "p":
			if m.state == stateTracking {
				if !m.isPaused {
					m.isPaused = true
					m.pauseStart = time.Now()
				} else {
					m.isPaused = false
					m.pausedDuration += time.Since(m.pauseStart)
					return m, tick()
				}
			}
		case "s":
			if m.state == stateTracking {
				m.endTime = time.Now()
				if m.isPaused {
					m.pausedDuration += time.Since(m.pauseStart)
				}
				m.logToFile()
				return m, tea.Quit
			}
		}

	case tickMsg:
		if (m.state == stateTracking || m.state == stateSettings) && !m.isPaused {
			return m, tick()
		}

	case error:
		m.err = msg
		return m, nil
	}

	if m.state == stateSetup {
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
				m.updateFiltered(newVal)
			}
		}
	}

	return m, cmd
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

func (m model) updateFocus() model {
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
	}
	return m
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
	var digits map[rune][]string
	var rows int
	if mode == ClockLarge {
		digits = digitsLarge
		rows = 7
	} else {
		digits = digitsSmall
		rows = 5
	}

	res := make([]string, rows)
	for _, r := range t {
		d, ok := digits[r]
		if !ok {
			continue
		}
		for i := 0; i < rows; i++ {
			res[i] += d[i] + "  "
		}
	}
	return strings.Join(res, "\n")
}

func (m model) View() string {
	var s string

	switch m.state {
	case stateSetup:
		modeText := "LIVE TIMER"
		if m.endTimeInput.Value() != "" {
			modeText = "MANUAL LOG"
		}
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Render("Session Setup ["+modeText+"]") + "\n\n"
		s += fmt.Sprintf("Start Time: %s\n", m.startTimeInput.View())
		s += fmt.Sprintf("End Time:   %s\n", m.endTimeInput.View())
		s += fmt.Sprintf("Comment:    %s\n", m.commentInput.View())
		if m.focusIndex == 2 && len(m.filtered) > 0 {
			s += "\nSuggestions:\n"
			for i, f := range m.filtered {
				style := lipgloss.NewStyle()
				if i == m.suggestionIndex {
					style = style.Foreground(lipgloss.Color("205")).Bold(true)
				}
				s += style.Render(fmt.Sprintf("  • %s", f)) + "\n"
			}
		}
		s += "\n(tab/arrows to navigate, enter to submit, q to quit)"
		if m.err != nil {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("\n\nError: %v", m.err))
		}

	case stateSettings:
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Render("Settings") + "\n\n"
		modeName := "Large ASCII"
		if m.config.ClockMode == ClockPlain {
			modeName = "Plain Text"
		} else if m.config.ClockMode == ClockSmall {
			modeName = "Small ASCII"
		}
		s += fmt.Sprintf("(m) Clock Mode:  %s\n", modeName)
		s += fmt.Sprintf("(c) Clock Color: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color(m.config.ClockColor)).Render("██████"))
		s += "\n(esc) back to timer"

	case stateTracking:
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

		s = fmt.Sprintf(
			"Activity: %s\nStatus: %s\n\n%s\n\n(p)ause, (s)top, (,) settings",
			displayComment,
			status,
			clock,
		)
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(s)
}

func parseTime(val string) (time.Time, error) {
	now := time.Now()
	t, err := time.Parse("15:04", val)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid format, use HH:MM")
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), nil
}

func (m model) logToFile() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exeDir := filepath.Dir(exePath)

	// Determine log path: logs/YYYY/MM-MonthName.md
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

	// Update recent_comments.json
	updateRecentComments(comment)

	dateStr := m.startTime.Format("2006-01-02")
	startStr := m.startTime.Format("15:04:05")
	endStr := m.endTime.Format("15:04:05")
	row := fmt.Sprintf("| %s | %s | %s | %s | %s |\n", dateStr, startStr, endStr, duration.Round(time.Second), comment)
	f.WriteString(row)
}

func updateRecentComments(newComment string) {
	comments := getRecentComments()
	
	// Remove if already exists to move to top
	var updated []string
	updated = append(updated, newComment)
	for _, c := range comments {
		if c != newComment {
			updated = append(updated, c)
		}
	}
	
	// Limit to 50
	if len(updated) > 50 {
		updated = updated[:50]
	}
	
	saveRecentComments(updated)
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
