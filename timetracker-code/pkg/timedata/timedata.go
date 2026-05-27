package timedata

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	ClockColor   string    `json:"clock_color"`
	ClockMode    ClockMode `json:"clock_mode"`

	ShowDaily    bool    `json:"show_daily"`
	ShowWeekly   bool    `json:"show_weekly"`
	ShowMonthly  bool    `json:"show_monthly"`
	ShowYearly   bool    `json:"show_yearly"`
	VacationDays int     `json:"vacation_days"`
	PreferYearly bool    `json:"prefer_yearly"`
	YearlyTarget float64 `json:"yearly_target"`
}

type TimeEntry struct {
	Date     time.Time     `json:"date"`
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
	Comment  string        `json:"comment"`
	Original string        `json:"-"`
}

type ReportData struct {
	Daily      map[string]map[string]time.Duration `json:"daily"`
	Weekly     map[string]map[string]time.Duration `json:"weekly"`
	Monthly    map[string]map[string]time.Duration `json:"monthly"`
	Yearly     map[string]map[string]time.Duration `json:"yearly"`

	DailyKeys   []string `json:"daily_keys"`
	WeeklyKeys  []string `json:"weekly_keys"`
	MonthlyKeys []string `json:"monthly_keys"`
	YearlyKeys  []string `json:"yearly_keys"`
}

func GetExeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

func LoadConfig() (Config, bool) {
	configPath := filepath.Join(GetExeDir(), "config.json")
	f, err := os.ReadFile(configPath)
	if err != nil {
		return Config{
			WeeklyTarget: 40,
			ClockColor:   "6",
			ClockMode:    ClockLarge,
			ShowDaily:    true,
			ShowWeekly:   true,
			ShowMonthly:  true,
			ShowYearly:   true,
		}, true
	}
	var cfg Config
	json.Unmarshal(f, &cfg)
	if cfg.ClockColor == "" {
		cfg.ClockColor = "6"
	}
	if !cfg.ShowDaily && !cfg.ShowWeekly && !cfg.ShowMonthly && !cfg.ShowYearly && !cfg.NoGoal {
		cfg.ShowDaily = true
		cfg.ShowWeekly = true
		cfg.ShowMonthly = true
		cfg.ShowYearly = true
	}
	needsOnboarding := !cfg.NoGoal && cfg.WeeklyTarget == 0
	return cfg, needsOnboarding
}

func SaveConfig(cfg Config) {
	configPath := filepath.Join(GetExeDir(), "config.json")
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func GetRecentComments() []string {
	cachePath := filepath.Join(GetExeDir(), "recent_comments.json")
	f, err := os.ReadFile(cachePath)
	if err != nil {
		defaults := []string{"Coding", "Meeting", "Research", "Documentation", "Testing", "Debugging", "Planning", "Support", "Review", "Design"}
		SaveRecentComments(defaults)
		return defaults
	}
	var comments []string
	if err := json.Unmarshal(f, &comments); err != nil {
		return nil
	}
	return comments
}

func SaveRecentComments(comments []string) {
	cachePath := filepath.Join(GetExeDir(), "recent_comments.json")
	if comments == nil {
		comments = []string{}
	}
	data, _ := json.MarshalIndent(comments, "", "  ")
	os.WriteFile(cachePath, data, 0644)
}

func UpdateRecentComments(newComment string, existing []string) []string {
	var updated []string
	updated = append(updated, newComment)
	for _, c := range existing {
		if c != newComment {
			updated = append(updated, c)
		}
	}
	if len(updated) > 50 {
		updated = updated[:50]
	}
	SaveRecentComments(updated)
	return updated
}

func ParseTime(val string) (time.Time, error) {
	now := time.Now()
	t, err := time.Parse("15:04", val)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid format, use HH:MM")
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), nil
}

func ValidateTimeFormat(val string) bool {
	if val == "" {
		return true
	}
	_, err := time.Parse("15:04", val)
	return err == nil
}

func ParseFile(path string, data *ReportData) {
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
		if data.Daily[dateStr] == nil {
			data.Daily[dateStr] = make(map[string]time.Duration)
		}
		data.Daily[dateStr][comment] += duration
		year, week := date.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", year, week)
		if data.Weekly[weekKey] == nil {
			data.Weekly[weekKey] = make(map[string]time.Duration)
		}
		data.Weekly[weekKey][comment] += duration
		monthKey := date.Format("2006-01")
		if data.Monthly[monthKey] == nil {
			data.Monthly[monthKey] = make(map[string]time.Duration)
		}
		data.Monthly[monthKey][comment] += duration
		yearKey := date.Format("2006")
		if data.Yearly[yearKey] == nil {
			data.Yearly[yearKey] = make(map[string]time.Duration)
		}
		data.Yearly[yearKey][comment] += duration
	}
}

func ParseLog() (ReportData, error) {
	logsDir := filepath.Join(GetExeDir(), "logs")
	data := ReportData{
		Daily:   make(map[string]map[string]time.Duration),
		Weekly:  make(map[string]map[string]time.Duration),
		Monthly: make(map[string]map[string]time.Duration),
		Yearly:  make(map[string]map[string]time.Duration),
	}
	err := filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			ParseFile(path, &data)
		}
		return nil
	})
	if err != nil {
		return data, err
	}
	for k := range data.Daily {
		data.DailyKeys = append(data.DailyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.DailyKeys)))
	for k := range data.Weekly {
		data.WeeklyKeys = append(data.WeeklyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.WeeklyKeys)))
	for k := range data.Monthly {
		data.MonthlyKeys = append(data.MonthlyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.MonthlyKeys)))
	for k := range data.Yearly {
		data.YearlyKeys = append(data.YearlyKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(data.YearlyKeys)))
	return data, nil
}

func LogTimeEntry(startTime, endTime time.Time, pausedDuration time.Duration, comment string) {
	exeDir := GetExeDir()
	yearStr := startTime.Format("2006")
	monthStr := startTime.Format("01-January")
	logDir := filepath.Join(exeDir, "logs", yearStr)
	os.MkdirAll(logDir, 0755)
	filename := filepath.Join(logDir, monthStr+".md")

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil && info.Size() == 0 {
		f.WriteString("| Date | Start | End | Duration | Comment |\n")
		f.WriteString("| --- | --- | --- | --- | --- |\n")
	}

	duration := endTime.Sub(startTime) - pausedDuration
	if comment == "" {
		comment = "work"
	}

	dateStr := startTime.Format("2006-01-02")
	startStr := startTime.Format("15:04:05")
	endStr := endTime.Format("15:04:05")
	row := fmt.Sprintf("| %s | %s | %s | %s | %s |\n", dateStr, startStr, endStr, duration.Round(time.Second), comment)
	f.WriteString(row)
}

func LoadDayEntries(date time.Time) ([]TimeEntry, error) {
	dateStr := date.Format("2006-01-02")
	exeDir := GetExeDir()
	yearStr := date.Format("2006")
	monthStr := date.Format("01-January")
	logPath := filepath.Join(exeDir, "logs", yearStr, monthStr+".md")
	f, err := os.Open(logPath)
	if err != nil {
		return []TimeEntry{}, nil
	}
	defer f.Close()
	var entries []TimeEntry
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
		entries = append(entries, TimeEntry{
			Date:     date,
			Start:    st,
			End:      et,
			Duration: dur,
			Comment:  comment,
			Original: line,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Start.Before(entries[j].Start)
	})

	return entries, nil
}

func CreateDayBackup(date time.Time) {
	exeDir := GetExeDir()
	yearStr := date.Format("2006")
	monthStr := date.Format("01-January")
	logPath := filepath.Join(exeDir, "logs", yearStr, monthStr+".md")
	backupPath := logPath + ".bak"
	input, err := os.ReadFile(logPath)
	if err == nil {
		os.WriteFile(backupPath, input, 0644)
	}
}

func RestoreDayBackup(date time.Time) {
	exeDir := GetExeDir()
	yearStr := date.Format("2006")
	monthStr := date.Format("01-January")
	logPath := filepath.Join(exeDir, "logs", yearStr, monthStr+".md")
	backupPath := logPath + ".bak"
	input, err := os.ReadFile(backupPath)
	if err == nil {
		os.WriteFile(logPath, input, 0644)
	}
}

func SaveDayChanges(date time.Time, entries []TimeEntry) {
	exeDir := GetExeDir()
	yearStr := date.Format("2006")
	monthStr := date.Format("01-January")
	logDir := filepath.Join(exeDir, "logs", yearStr)
	os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, monthStr+".md")
	dateStr := date.Format("2006-01-02")

	var otherLines []string
	f, err := os.Open(logPath)
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(line, "| "+dateStr+" |") {
				otherLines = append(otherLines, line)
			}
		}
		f.Close()
	}

	var newLines []string
	for _, e := range entries {
		row := fmt.Sprintf("| %s | %s | %s | %s | %s |",
			dateStr,
			e.Start.Format("15:04:05"),
			e.End.Format("15:04:05"),
			e.Duration.Round(time.Second),
			e.Comment,
		)
		newLines = append(newLines, row)
	}

	f, err = os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
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
