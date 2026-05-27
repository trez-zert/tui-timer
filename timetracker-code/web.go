package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"time-tracker/pkg/timedata"
)

//go:embed static/*
var staticFiles embed.FS

type webTimerState struct {
	mu             sync.Mutex
	running        bool
	startTime      time.Time
	comment        string
	paused         bool
	pausedDuration time.Duration
	pauseStart     time.Time
}

var webTimer webTimerState

func startWebServer() {
	port := 8080
	for i, arg := range os.Args[1:] {
		if arg == "--port" && i+2 < len(os.Args) {
			if p, err := strconv.Atoi(os.Args[i+2]); err == nil {
				port = p
			}
		}
		parts := strings.SplitN(arg, "=", 2)
		if parts[0] == "--port" && len(parts) == 2 {
			if p, err := strconv.Atoi(parts[1]); err == nil {
				port = p
			}
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/comments", handleComments)
	mux.HandleFunc("/api/timer/start", handleTimerStart)
	mux.HandleFunc("/api/timer/status", handleTimerStatus)
	mux.HandleFunc("/api/timer/pause", handleTimerPause)
	mux.HandleFunc("/api/timer/stop", handleTimerStop)
	mux.HandleFunc("/api/timer/switch", handleTimerSwitch)
	mux.HandleFunc("/api/logs", handleLogs)
	mux.HandleFunc("/api/reports", handleReports)

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error embedding static files: %v\n", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/", fileServer)

	addr := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		addr = fmt.Sprintf(":%d", port)
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("\n  tuitime web UI\n")
	fmt.Printf("  ─────────────────\n")
	fmt.Printf("  Local:   http://localhost:%d\n", port)

	addrs := localIPs()
	for _, ip := range addrs.lan {
		fmt.Printf("  LAN:     http://%s:%d\n", ip, port)
	}
	for _, ip := range addrs.tailscale {
		fmt.Printf("  Tailscale: http://%s:%d\n", ip, port)
	}
	fmt.Printf("\n  Open this URL on your phone or tablet.\n")
	fmt.Printf("  Press Ctrl+C to stop.\n\n")

	if err := http.Serve(listener, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

type ipList struct {
	lan       []string
	tailscale []string
}

func localIPs() ipList {
	var result ipList
	ifaces, err := net.Interfaces()
	if err != nil {
		return result
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			if ip == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
				continue
			}
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}
			if isTailscaleIP(ip4) {
				result.tailscale = append(result.tailscale, ip4.String())
			} else if isPrivateLAN(ip4) {
				result.lan = append(result.lan, ip4.String())
			}
		}
	}
	return result
}

func isTailscaleIP(ip net.IP) bool {
	if len(ip) < 4 {
		return false
	}
	return ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127
}

func isPrivateLAN(ip net.IP) bool {
	if len(ip) < 4 {
		return false
	}
	if ip[0] == 10 {
		return true
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	return false
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- Config ---

func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cfg, _ := timedata.LoadConfig()
		jsonResponse(w, cfg)
	case "POST":
		var cfg timedata.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonError(w, "invalid json", 400)
			return
		}
		timedata.SaveConfig(cfg)
		jsonResponse(w, map[string]string{"status": "ok"})
	default:
		jsonError(w, "method not allowed", 405)
	}
}

// --- Comments ---

func handleComments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		comments := timedata.GetRecentComments()
		jsonResponse(w, comments)
	case "POST":
		var body struct {
			Comment string `json:"comment"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid json", 400)
			return
		}
		existing := timedata.GetRecentComments()
		updated := timedata.UpdateRecentComments(body.Comment, existing)
		jsonResponse(w, updated)
	default:
		jsonError(w, "method not allowed", 405)
	}
}

// --- Timer ---

func handleTimerStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	var body struct {
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	webTimer.mu.Lock()
	defer webTimer.mu.Unlock()

	webTimer.running = true
	webTimer.startTime = time.Now()
	webTimer.comment = body.Comment
	webTimer.paused = false
	webTimer.pausedDuration = 0
	webTimer.pauseStart = time.Time{}

	jsonResponse(w, map[string]interface{}{
		"status":     "started",
		"start_time": webTimer.startTime,
		"comment":    webTimer.comment,
	})
}

func handleTimerStatus(w http.ResponseWriter, r *http.Request) {
	webTimer.mu.Lock()
	defer webTimer.mu.Unlock()

	if !webTimer.running {
		jsonResponse(w, map[string]interface{}{
			"running": false,
		})
		return
	}

	elapsed := time.Since(webTimer.startTime) - webTimer.pausedDuration
	if webTimer.paused {
		elapsed = webTimer.pauseStart.Sub(webTimer.startTime) - webTimer.pausedDuration
	}

	jsonResponse(w, map[string]interface{}{
		"running":  true,
		"paused":   webTimer.paused,
		"elapsed":  elapsed.String(),
		"elapsed_ns": elapsed.Nanoseconds(),
		"start_time": webTimer.startTime,
		"comment":    webTimer.comment,
	})
}

func handleTimerPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	webTimer.mu.Lock()
	defer webTimer.mu.Unlock()

	if !webTimer.paused {
		webTimer.paused = true
		webTimer.pauseStart = time.Now()
	} else {
		webTimer.paused = false
		webTimer.pausedDuration += time.Since(webTimer.pauseStart)
	}

	jsonResponse(w, map[string]interface{}{
		"paused": webTimer.paused,
	})
}

func handleTimerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	webTimer.mu.Lock()
	defer webTimer.mu.Unlock()

	if !webTimer.running {
		jsonError(w, "no timer running", 400)
		return
	}

	endTime := time.Now()
	if webTimer.paused {
		webTimer.pausedDuration += time.Since(webTimer.pauseStart)
	}
	comment := webTimer.comment
	timedata.LogTimeEntry(webTimer.startTime, endTime, webTimer.pausedDuration, comment)

	existing := timedata.GetRecentComments()
	timedata.UpdateRecentComments(comment, existing)

	webTimer.running = false
	webTimer.paused = false
	webTimer.pausedDuration = 0

	jsonResponse(w, map[string]string{"status": "logged"})
}

func handleTimerSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	var body struct {
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	webTimer.mu.Lock()
	defer webTimer.mu.Unlock()

	if webTimer.running {
		endTime := time.Now()
		if webTimer.paused {
			webTimer.pausedDuration += time.Since(webTimer.pauseStart)
		}
		timedata.LogTimeEntry(webTimer.startTime, endTime, webTimer.pausedDuration, webTimer.comment)
		existing := timedata.GetRecentComments()
		timedata.UpdateRecentComments(webTimer.comment, existing)
	}

	if body.Comment != "" {
		existing := timedata.GetRecentComments()
		timedata.UpdateRecentComments(body.Comment, existing)
	}

	webTimer.startTime = time.Now()
	webTimer.comment = body.Comment
	webTimer.paused = false
	webTimer.pausedDuration = 0
	webTimer.pauseStart = time.Time{}
	webTimer.running = true

	jsonResponse(w, map[string]string{"status": "switched"})
}

// --- Logs ---

type apiEntry struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration string `json:"duration"`
	Comment  string `json:"comment"`
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	var b strings.Builder
	if h > 0 { fmt.Fprintf(&b, "%dh", h) }
	if m > 0 { fmt.Fprintf(&b, "%dm", m) }
	if s > 0 || b.Len() == 0 { fmt.Fprintf(&b, "%ds", s) }
	return b.String()
}

func parseAPITime(date time.Time, timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	layouts := []string{"15:04:05", "15:04"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, timeStr)
		if err == nil {
			return time.Date(date.Year(), date.Month(), date.Day(),
				t.Hour(), t.Minute(), t.Second(), 0, date.Location())
		}
	}
	return time.Time{}
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		dateStr := r.URL.Query().Get("date")
		if dateStr == "" {
			dateStr = time.Now().Format("2006-01-02")
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			jsonError(w, "invalid date, use YYYY-MM-DD", 400)
			return
		}
		entries, err := timedata.LoadDayEntries(date)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		var api []apiEntry
		for _, e := range entries {
			api = append(api, apiEntry{
				Start:    e.Start.Format("15:04"),
				End:      e.End.Format("15:04"),
				Duration: formatDuration(e.Duration),
				Comment:  e.Comment,
			})
		}
		if api == nil {
			api = []apiEntry{}
		}
		jsonResponse(w, api)

	case "POST":
		var body struct {
			Date    string     `json:"date"`
			Entries []apiEntry `json:"entries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid json", 400)
			return
		}
		date, err := time.Parse("2006-01-02", body.Date)
		if err != nil {
			jsonError(w, "invalid date", 400)
			return
		}
		if body.Entries == nil {
			body.Entries = []apiEntry{}
		}
		var timedataEntries []timedata.TimeEntry
		for _, ae := range body.Entries {
			st := parseAPITime(date, ae.Start)
			et := parseAPITime(date, ae.End)
			if ae.Comment == "" {
				ae.Comment = "work"
			}
			timedataEntries = append(timedataEntries, timedata.TimeEntry{
				Date:     date,
				Start:    st,
				End:      et,
				Duration: et.Sub(st),
				Comment:  ae.Comment,
			})
		}
		timedata.CreateDayBackup(date)
		timedata.SaveDayChanges(date, timedataEntries)
		jsonResponse(w, map[string]string{"status": "saved"})

	default:
		jsonError(w, "method not allowed", 405)
	}
}

// --- Reports ---

func handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}
	data, err := timedata.ParseLog()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	cfg, _ := timedata.LoadConfig()
	weeklyTarget := time.Duration(cfg.WeeklyTarget * float64(time.Hour))
	dailyTarget := weeklyTarget / 5

	// Format durations for JSON
	type sectionItem struct {
		Key      string            `json:"key"`
		Total    string            `json:"total"`
		TotalNs  int64             `json:"total_ns"`
		Comments map[string]string `json:"comments"`
	}

	var dailyItems, weeklyItems, monthlyItems, yearlyItems []sectionItem

	formatSection := func(keys []string, dataMap map[string]map[string]time.Duration) []sectionItem {
		var items []sectionItem
		for _, k := range keys {
			total := time.Duration(0)
			comments := make(map[string]string)
			for c, d := range dataMap[k] {
				comments[c] = d.Round(time.Minute).String()
				total += d
			}
			items = append(items, sectionItem{
				Key:      k,
				Total:    total.Round(time.Minute).String(),
				TotalNs:  total.Nanoseconds(),
				Comments: comments,
			})
		}
		return items
	}

	dailyItems = formatSection(data.DailyKeys, data.Daily)
	weeklyItems = formatSection(data.WeeklyKeys, data.Weekly)
	monthlyItems = formatSection(data.MonthlyKeys, data.Monthly)
	yearlyItems = formatSection(data.YearlyKeys, data.Yearly)

	result := map[string]interface{}{
		"daily":        dailyItems,
		"weekly":       weeklyItems,
		"monthly":      monthlyItems,
		"yearly":       yearlyItems,
		"config":       cfg,
		"weekly_target_ns": weeklyTarget.Nanoseconds(),
		"daily_target_ns":  dailyTarget.Nanoseconds(),
	}

	jsonResponse(w, result)
}
