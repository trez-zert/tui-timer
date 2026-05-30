package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"time-tracker/pkg/timedata"
)

//go:embed static/*
var staticFiles embed.FS

type SharedTimer struct {
	mu             sync.Mutex
	running        bool
	startTime      time.Time
	comment        string
	paused         bool
	pausedDuration time.Duration
	pauseStart     time.Time
}

var sharedTimer SharedTimer

func (st *SharedTimer) IsRunning() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.running
}

func (st *SharedTimer) GetStatus() (running, paused bool, elapsed time.Duration, comment string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.running {
		return false, false, 0, ""
	}
	if st.paused {
		elapsed = st.pauseStart.Sub(st.startTime) - st.pausedDuration
	} else {
		elapsed = time.Since(st.startTime) - st.pausedDuration
	}
	return true, st.paused, elapsed, st.comment
}

func (st *SharedTimer) Start(comment string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.running = true
	st.startTime = time.Now()
	st.comment = comment
	st.paused = false
	st.pausedDuration = 0
	st.pauseStart = time.Time{}
}

func (st *SharedTimer) TogglePause() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.running {
		return
	}
	if !st.paused {
		st.paused = true
		st.pauseStart = time.Now()
	} else {
		st.paused = false
		st.pausedDuration += time.Since(st.pauseStart)
	}
}

func (st *SharedTimer) StopAndLog() (startTime, endTime time.Time, pausedDuration time.Duration, comment string, ok bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.running {
		return time.Time{}, time.Time{}, 0, "", false
	}
	endTime = time.Now()
	if st.paused {
		st.pausedDuration += time.Since(st.pauseStart)
	}
	startTime = st.startTime
	pausedDuration = st.pausedDuration
	comment = st.comment
	st.running = false
	st.paused = false
	st.pausedDuration = 0
	return startTime, endTime, pausedDuration, comment, true
}

func (st *SharedTimer) SwitchTask(newComment string) (startTime, endTime time.Time, oldComment string, ok bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.running {
		st.running = true
		st.startTime = time.Now()
		st.comment = newComment
		st.paused = false
		st.pausedDuration = 0
		st.pauseStart = time.Time{}
		return time.Time{}, time.Time{}, "", false
	}
	endTime = time.Now()
	if st.paused {
		st.pausedDuration += time.Since(st.pauseStart)
	}
	startTime = st.startTime
	oldComment = st.comment
	st.startTime = endTime
	st.comment = newComment
	st.paused = false
	st.pausedDuration = 0
	st.pauseStart = time.Time{}
	return startTime, endTime, oldComment, true
}

type WebServerInfo struct {
	Running     bool
	Port        int
	LocalURL    string
	LanIPs      []string
	TailscaleIPs []string
}

func StartWebServer(port int) (WebServerInfo, func(), error) {
	info := WebServerInfo{Port: port}

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
		return info, nil, fmt.Errorf("embedding static files: %v", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/", fileServer)

	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		addr = fmt.Sprintf("localhost:%d", port)
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			return info, nil, fmt.Errorf("cannot bind port %d: %v", port, err)
		}
	}

	info.Running = true
	info.LocalURL = fmt.Sprintf("http://localhost:%d", port)

	addrs := localIPs()
	for _, ip := range addrs.lan {
		info.LanIPs = append(info.LanIPs, fmt.Sprintf("http://%s:%d", ip, port))
	}
	for _, ip := range addrs.tailscale {
		info.TailscaleIPs = append(info.TailscaleIPs, fmt.Sprintf("http://%s:%d", ip, port))
	}

	shutdown := func() { listener.Close() }

	go func() {
		http.Serve(listener, mux)
	}()

	return info, shutdown, nil
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

	sharedTimer.Start(body.Comment)

	jsonResponse(w, map[string]interface{}{
		"status":  "started",
		"comment": body.Comment,
	})
}

func handleTimerStatus(w http.ResponseWriter, r *http.Request) {
	running, paused, elapsed, comment := sharedTimer.GetStatus()
	if !running {
		jsonResponse(w, map[string]interface{}{
			"running": false,
		})
		return
	}

	jsonResponse(w, map[string]interface{}{
		"running":    true,
		"paused":     paused,
		"elapsed":    elapsed.String(),
		"elapsed_ns": elapsed.Nanoseconds(),
		"comment":    comment,
	})
}

func handleTimerPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	sharedTimer.TogglePause()
	_, paused, _, _ := sharedTimer.GetStatus()
	jsonResponse(w, map[string]interface{}{
		"paused": paused,
	})
}

func handleTimerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	startTime, endTime, pausedDuration, comment, ok := sharedTimer.StopAndLog()
	if !ok {
		jsonError(w, "no timer running", 400)
		return
	}
	timedata.LogTimeEntry(startTime, endTime, pausedDuration, comment)
	existing := timedata.GetRecentComments()
	timedata.UpdateRecentComments(comment, existing)

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

	startTime, endTime, oldComment, wasRunning := sharedTimer.SwitchTask(body.Comment)
	if wasRunning {
		timedata.LogTimeEntry(startTime, endTime, 0, oldComment)
		existing := timedata.GetRecentComments()
		timedata.UpdateRecentComments(oldComment, existing)
	}
	if body.Comment != "" {
		existing := timedata.GetRecentComments()
		timedata.UpdateRecentComments(body.Comment, existing)
	}

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
	layouts := []string{"15:04:05", "15:04", "1504"}
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
		Key       string            `json:"key"`
		Total     string            `json:"total"`
		TotalNs   int64             `json:"total_ns"`
		Comments  map[string]string `json:"comments"`
		WeekRange string            `json:"week_range,omitempty"`
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

	for i := range weeklyItems {
		weeklyItems[i].WeekRange = timedata.WeekKeyToRange(weeklyItems[i].Key)
	}

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
