package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	stateStartTime sessionState = iota
	stateComment
	stateTracking
)

type model struct {
	state          sessionState
	startTimeInput textinput.Model
	commentInput   textinput.Model

	startTime      time.Time
	endTime        time.Time
	pausedDuration time.Duration
	pauseStart     time.Time
	isPaused       bool

	suggestions     []string
	filtered        []string
	suggestionIndex int

	err error
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func getUniqueComments() []string {
	f, err := os.Open("timelog.md")
	if err != nil {
		return nil
	}
	defer f.Close()

	comments := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") || strings.Contains(line, "| Date |") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 6 {
			comment := strings.TrimSpace(parts[5])
			if comment != "" && comment != "work" {
				comments[comment] = true
			}
		}
	}

	var res []string
	for c := range comments {
		res = append(res, c)
	}
	sort.Strings(res)
	return res
}

func initialModel() model {
	s := textinput.New()
	s.Placeholder = "Start Time (HH:MM or leave blank for now)"
	s.Focus()

	c := textinput.New()
	c.Placeholder = "Comment (optional, defaults to 'work')"

	suggestions := getUniqueComments()

	return model{
		state:           stateStartTime,
		startTimeInput:  s,
		commentInput:    c,
		suggestions:     suggestions,
		suggestionIndex: -1,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			switch m.state {
			case stateStartTime:
				val := m.startTimeInput.Value()
				if val == "" {
					m.startTime = time.Now()
				} else {
					t, err := parseTime(val)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.startTime = t
				}
				m.state = stateComment
				m.commentInput.Focus()
			case stateComment:
				m.state = stateTracking
				return m, tick()
			}
		case "down", "tab":
			if m.state == stateComment && len(m.filtered) > 0 {
				if msg.String() == "tab" && m.suggestionIndex == -1 {
					// Auto-complete with the first match if nothing is selected
					m.commentInput.SetValue(m.filtered[0])
					m.suggestionIndex = -1
					m.filtered = nil
				} else {
					m.suggestionIndex = (m.suggestionIndex + 1) % len(m.filtered)
					m.commentInput.SetValue(m.filtered[m.suggestionIndex])
				}
				return m, nil
			}
		case "up":
			if m.state == stateComment && len(m.filtered) > 0 {
				m.suggestionIndex--
				if m.suggestionIndex < 0 {
					m.suggestionIndex = len(m.filtered) - 1
				}
				m.commentInput.SetValue(m.filtered[m.suggestionIndex])
				return m, nil
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
		if m.state == stateTracking && !m.isPaused {
			return m, tick()
		}

	case error:
		m.err = msg
		return m, nil
	}

	switch m.state {
	case stateStartTime:
		m.startTimeInput, cmd = m.startTimeInput.Update(msg)
	case stateComment:
		prevVal := m.commentInput.Value()
		m.commentInput, cmd = m.commentInput.Update(msg)
		newVal := m.commentInput.Value()

		if newVal != prevVal {
			m.suggestionIndex = -1
			if newVal == "" {
				m.filtered = nil
			} else {
				m.filtered = nil
				for _, s := range m.suggestions {
					if strings.HasPrefix(strings.ToLower(s), strings.ToLower(newVal)) {
						m.filtered = append(m.filtered, s)
					}
				}
			}
		}
	}

	return m, cmd
}

func (m model) View() string {
	var s string

	switch m.state {
	case stateStartTime:
		s = fmt.Sprintf("Enter Start Time (HH:MM or blank for now):\n\n%s\n\n(ctrl+c to quit)", m.startTimeInput.View())
		if m.err != nil {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("\n\nError: %v", m.err))
		}
	case stateComment:
		s = fmt.Sprintf("Enter Comment (optional):\n\n%s", m.commentInput.View())
		if len(m.filtered) > 0 {
			s += "\n\nSuggestions (tab/arrows to select):\n"
			for i, f := range m.filtered {
				style := lipgloss.NewStyle()
				if i == m.suggestionIndex {
					style = style.Foreground(lipgloss.Color("205")).Bold(true)
				}
				s += style.Render(fmt.Sprintf("  • %s", f)) + "\n"
			}
		}
		s += "\n(ctrl+c to quit)"
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

		s = fmt.Sprintf(
			"Activity: %s\nStatus: %s\nElapsed: %s\n\n(p)ause/resume, (s)top",
			displayComment,
			status,
			elapsed.Round(time.Second),
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
	filename := "timelog.md"
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

	dateStr := m.startTime.Format("2006-01-02")
	startStr := m.startTime.Format("15:04:05")
	endStr := m.endTime.Format("15:04:05")
	
	row := fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
		dateStr,
		startStr,
		endStr,
		duration.Round(time.Second),
		comment,
	)
	f.WriteString(row)
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
