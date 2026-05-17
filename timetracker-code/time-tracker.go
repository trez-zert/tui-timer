package main

import (
	"fmt"
	"log"
	"os"
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
	
	err            error
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func initialModel() model {
	s := textinput.New()
	s.Placeholder = "Start Time (HH:MM or leave blank for now)"
	s.Focus()

	c := textinput.New()
	c.Placeholder = "Comment (optional, defaults to 'work')"

	return model{
		state:          stateStartTime,
		startTimeInput: s,
		commentInput:   c,
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
		m.commentInput, cmd = m.commentInput.Update(msg)
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
		s = fmt.Sprintf("Enter Comment (optional):\n\n%s\n\n(ctrl+c to quit)", m.commentInput.View())
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
