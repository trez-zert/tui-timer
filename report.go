package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type entry struct {
	date     time.Time
	duration time.Duration
	comment  string
}

type reportData struct {
	daily   map[string]map[string]time.Duration
	weekly  map[string]map[string]time.Duration
	monthly map[string]map[string]time.Duration
	
	dailyKeys   []string
	weeklyKeys  []string
	monthlyKeys []string
}

type model struct {
	data   reportData
	cursor int
	err    error
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
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

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render("Time Tracking Report") + "\n\n")

	s.WriteString(lipgloss.NewStyle().Underline(true).Render("DAILY TOTALS") + "\n")
	for _, k := range m.data.dailyKeys {
		total := time.Duration(0)
		for _, d := range m.data.daily[k] {
			total += d
		}
		s.WriteString(fmt.Sprintf("%s: %s\n", k, total))
		for comment, dur := range m.data.daily[k] {
			s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur))
		}
	}

	s.WriteString("\n" + lipgloss.NewStyle().Underline(true).Render("WEEKLY TOTALS") + "\n")
	for _, k := range m.data.weeklyKeys {
		total := time.Duration(0)
		for _, d := range m.data.weekly[k] {
			total += d
		}
		s.WriteString(fmt.Sprintf("%s: %s\n", k, total))
		for comment, dur := range m.data.weekly[k] {
			s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur))
		}
	}

	s.WriteString("\n" + lipgloss.NewStyle().Underline(true).Render("MONTHLY TOTALS") + "\n")
	for _, k := range m.data.monthlyKeys {
		total := time.Duration(0)
		for _, d := range m.data.monthly[k] {
			total += d
		}
		s.WriteString(fmt.Sprintf("%s: %s\n", k, total))
		for comment, dur := range m.data.monthly[k] {
			s.WriteString(fmt.Sprintf("  - %s: %s\n", comment, dur))
		}
	}

	s.WriteString("\n(q to quit)")
	
	// Apply cursor offset if we wanted scrolling, for now just basic output
	return lipgloss.NewStyle().Padding(1, 2).Render(s.String())
}

func parseLog() (reportData, error) {
	f, err := os.Open("timelog.md")
	if err != nil {
		return reportData{}, err
	}
	defer f.Close()

	data := reportData{
		daily:   make(map[string]map[string]time.Duration),
		weekly:  make(map[string]map[string]time.Duration),
		monthly: make(map[string]map[string]time.Duration),
	}

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

		// Daily
		if data.daily[dateStr] == nil {
			data.daily[dateStr] = make(map[string]time.Duration)
		}
		data.daily[dateStr][comment] += duration

		// Weekly
		year, week := date.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", year, week)
		if data.weekly[weekKey] == nil {
			data.weekly[weekKey] = make(map[string]time.Duration)
		}
		data.weekly[weekKey][comment] += duration

		// Monthly
		monthKey := date.Format("2006-01")
		if data.monthly[monthKey] == nil {
			data.monthly[monthKey] = make(map[string]time.Duration)
		}
		data.monthly[monthKey][comment] += duration
	}

	for k := range data.daily {
		data.dailyKeys = append(data.dailyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.dailyKeys)))

	for k := range data.weekly {
		data.weeklyKeys = append(data.weeklyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.weeklyKeys)))

	for k := range data.monthly {
		data.monthlyKeys = append(data.monthlyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.monthlyKeys)))

	return data, nil
}

func main() {
	data, err := parseLog()
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	p := tea.NewProgram(model{data: data, err: err})
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
