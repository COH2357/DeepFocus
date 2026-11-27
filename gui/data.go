package gui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type appEntry struct {
	Name         string `json:"name"`
	Exe          string `json:"exe"`
	AppID        string `json:"appId"`
	ExePath      string `json:"exePath,omitempty"`
	Seconds      int    `json:"seconds"`
	Opens        int    `json:"opens"`
	LimitMinutes int    `json:"limitMinutes,omitempty"`
	LimitKey     string `json:"limitKey,omitempty"`
}

type siteEntry struct {
	Name    string `json:"name"`
	Seconds int    `json:"seconds"`
}

type dayEntry struct {
	Date    string `json:"date"`
	Seconds int    `json:"seconds"`
	IsToday bool   `json:"isToday"`
	Weekday int    `json:"weekday"`
	Weekend bool   `json:"weekend"`
	Future  bool   `json:"future,omitempty"`
}

type sessionEntry struct {
	App     string `json:"app"`
	Exe     string `json:"exe"`
	Title   string `json:"title"`
	Seconds int    `json:"seconds"`
	Start   int64  `json:"start"`
	End     int64  `json:"end"`
	Date    string `json:"date"`
	Active  bool   `json:"active"`
}

type overviewData struct {
	Date      string         `json:"date"`
	Total     int            `json:"total"`
	AppCount  int            `json:"appCount"`
	Launches  int            `json:"launches"`
	Apps      []appEntry     `json:"apps"`
	Sites     []siteEntry    `json:"sites"`
	Days      []dayEntry     `json:"days"`
	Hourly    []int          `json:"hourly"`
	Yesterday int            `json:"yesterday"`
	WeekTotal int            `json:"weekTotal"`
	DailyAvg  int            `json:"dailyAvg"`
	PeakDay   *dayEntry      `json:"peakDay"`
	Active    *sessionEntry  `json:"active"`
	Recent    []sessionEntry `json:"recent"`
	LimitsHit int            `json:"limitsHit"`
}

type statsData struct {
	From      string         `json:"from"`
	To        string         `json:"to"`
	Days      int            `json:"days"`
	Total     int            `json:"total"`
	PrevTotal int            `json:"prevTotal"`
	AvgPerDay int            `json:"avgPerDay"`
	Apps      []appEntry     `json:"apps"`
	Sites     []siteEntry    `json:"sites"`
	Series    []dayEntry     `json:"series"`
	Hourly    []int          `json:"hourly"`
	Sessions  []sessionEntry `json:"sessions"`
	PeakDay   *dayEntry      `json:"peakDay"`
}

type limitEntry struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Exe            string `json:"exe"`
	AppID          string `json:"appId"`
	Minutes        int    `json:"minutes"`
	UsedSeconds    int    `json:"usedSeconds"`
	Percent        int    `json:"percent"`
	Status         string `json:"status"`
	Mode           string `json:"mode,omitempty"`
	MaxOpens       int    `json:"maxOpens,omitempty"`
	MinutesPerOpen int    `json:"minutesPerOpen,omitempty"`
	OpensUsed      int    `json:"opensUsed,omitempty"`
}

type siteLimitEntry struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Minutes     int    `json:"minutes"`
	UsedSeconds int    `json:"usedSeconds"`
	Percent     int    `json:"percent"`
	Status      string `json:"status"`
}

type reminderSettings struct {
	Enabled bool `json:"enabled"`
	Minutes int  `json:"minutes"`
}

type idleSettings struct {
	ThresholdMinutes int  `json:"thresholdMinutes"`
	TrackWhileIdle   bool `json:"trackWhileIdle"`
	TrackWhilePlay   bool `json:"trackWhilePlaying"`
}

type settingsData struct {
	Version           string           `json:"version"`
	DBPath            string           `json:"dbPath"`
	InstallDir        string           `json:"installDir"`
	DaemonRunning     bool             `json:"daemonRunning"`
	Autostart         bool             `json:"autostart"`
	Paused            bool             `json:"paused"`
	RetentionDays     int              `json:"retentionDays"`
	TrackingInterval  int              `json:"trackingIntervalSeconds"`
	Whitelist         []string         `json:"whitelist"`
	CustomBrowsers    []string         `json:"customBrowsers"`
	Limits            []limitEntry     `json:"limits"`
	BreakReminder     reminderSettings `json:"breakReminder"`
	PomodoroMinutes   int              `json:"pomodoroMinutes"`
	SnoozeMinutes     int              `json:"snoozeMinutes"`
	Idle              idleSettings     `json:"idle"`
	RedactTitles      bool             `json:"redactTitles"`
	MinRetentionDays  int              `json:"minRetentionDays"`
	MaxRetentionDays  int              `json:"maxRetentionDays"`
	MaxIdleMinutes    int              `json:"maxIdleMinutes"`
	MediaDetectionOK  bool             `json:"mediaDetection"`
	DomainGrouping    bool             `json:"domainGrouping"`
	CloseToTray       bool             `json:"closeToTray"`
	KeepHistory       bool             `json:"keepHistory"`
	ArchiveDir        string           `json:"archiveDir"`
	ArchiveCount      int              `json:"archiveCount"`
	ArchiveLatest     string           `json:"archiveLatest"`
	QuoteCount        int              `json:"quoteCount"`
	Language          string           `json:"language"`
	StrongWebMode     bool             `json:"strongWebMode"`
	WebAllowlist      []string         `json:"webAllowlist"`
	ExtensionActive   bool             `json:"extensionActive"`
	ExtensionBrowsers []string         `json:"extensionBrowsers"`
}

func extensionBrowsers() []string {
	out := []string{}
	for _, browser := range []string{"edge", "chrome", "firefox", "brave"} {
		if storage.ExtensionSeen(browser, 45*time.Second) {
			out = append(out, browser)
		}
	}
	return out
}

func (a *app) settingsSnapshot() settingsData {
	dbPath, _ := storage.GetDBPath()
	autostart, _, _ := system.GetAutoStartEnabled()
	daemonRunning := system.CountInstancesInDir(system.DaemonProcessName, system.GetInstallDir()) >= 1
	limits, _ := a.buildLimits()
	archiveDir, _ := storage.GetArchiveDir()
	archiveCount, archiveLatest := storage.ArchiveSummary()
	quotes, _ := storage.ListBlockQuotes()
	return settingsData{
		Version:          a.version,
		DBPath:           dbPath,
		InstallDir:       system.GetInstallDir(),
		DaemonRunning:    daemonRunning,
		Autostart:        autostart,
		Paused:           storage.IsPaused(),
		RetentionDays:    storage.GetRetentionDays(),
		TrackingInterval: storage.GetTrackingIntervalSeconds(),
		Whitelist:        system.GetWhitelistApps(),
		CustomBrowsers:   system.GetCustomBrowsersList(),
		Limits:           limits,
		BreakReminder: reminderSettings{
			Enabled: system.GetBreakReminderEnabled(),
			Minutes: system.GetBreakReminderMinutes(),
		},
		PomodoroMinutes: system.GetPomodoroMinutes(),
		SnoozeMinutes:   system.GetSnoozeDurationMinutes(),
		Idle: idleSettings{
			ThresholdMinutes: system.GetIdleThresholdMinutes(),
			TrackWhileIdle:   system.GetTrackWhileIdle(),
			TrackWhilePlay:   system.GetTrackWhilePlaying(),
		},
		RedactTitles:     system.GetRedactTitles(),
		MinRetentionDays: storage.MinRetentionDays,
		MaxRetentionDays: storage.MaxRetentionDays,
		MaxIdleMinutes:   system.MaxIdleThresholdMinutes,
		// The audio sampler is Windows-only. On a platform without it the
		// toggle is hidden rather than silently ineffective.
		MediaDetectionOK:  mediaDetectionAvailable,
		DomainGrouping:    storage.GetDomainGrouping(),
		CloseToTray:       system.GetCloseToTray(),
		KeepHistory:       storage.GetKeepHistory(),
		ArchiveDir:        archiveDir,
		ArchiveCount:      archiveCount,
		ArchiveLatest:     archiveLatest,
		QuoteCount:        len(quotes),
		Language:          system.GetLanguage(),
		StrongWebMode:     system.GetStrongWebMode(),
		WebAllowlist:      system.GetWebAllowlist(),
		ExtensionBrowsers: extensionBrowsers(),
		ExtensionActive:   len(extensionBrowsers()) > 0,
	}
}

func (a *app) buildOverview(date string) (*overviewData, error) {
	start, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid date")
	}

	stats, err := storage.GetAppStatsInRange(date, date)
	if err != nil {
		return nil, err
	}
	stats = filterAppStats(stats)
	apps := aggregateApps(stats, system.GetAppTimeLimits())
	total := 0
	launches := 0
	for _, s := range stats {
		total += s.TotalDurationSecs
		launches += s.OpenCount
	}

	sites := []siteEntry{}
	if browserStats, err := storage.GetBrowserStatsInRange(date, date); err == nil {
		sites = aggregateSites(browserStats)
	}

	weekStart := start.AddDate(0, 0, -6)
	weekStats, err := storage.GetAppStatsInRange(weekStart.Format("2006-01-02"), date)
	if err != nil {
		return nil, err
	}
	weekStats = filterAppStats(weekStats)
	days := buildDaySeries(weekStart, start, weekStats)
	weekTotal := 0
	for _, d := range days {
		weekTotal += d.Seconds
	}
	var peak *dayEntry
	for i := range days {
		if peak == nil || days[i].Seconds > peak.Seconds {
			peak = &days[i]
		}
	}
	yesterday := 0
	if len(days) >= 2 {
		yesterday = days[len(days)-2].Seconds
	}

	var hourly []int
	if buckets, err := storage.GetHourlyUsageToday(date); err == nil {
		hourly = buckets[:]
	} else {
		hourly = make([]int, 24)
	}

	recent := recentSessions(date, 6)
	active := activeSession()

	limitsHit := 0
	for _, lim := range buildLimitEntries(system.GetAppTimeLimits(), apps) {
		if lim.Status != "ok" {
			limitsHit++
		}
	}

	return &overviewData{
		Date:      date,
		Total:     total,
		AppCount:  len(apps),
		Launches:  launches,
		Apps:      apps,
		Sites:     sites,
		Days:      days,
		Hourly:    hourly,
		Yesterday: yesterday,
		WeekTotal: weekTotal,
		DailyAvg:  weekTotal / maxInt(1, len(days)),
		PeakDay:   peak,
		Active:    active,
		Recent:    recent,
		LimitsHit: limitsHit,
	}, nil
}

func (a *app) buildStats(from, to string) (*statsData, error) {
	fromTime, err := time.ParseInLocation("2006-01-02", from, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid start date")
	}
	toTime, err := time.ParseInLocation("2006-01-02", to, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid end date")
	}
	if toTime.Before(fromTime) {
		return nil, fmt.Errorf("start date is after end date")
	}
	maxDays := storage.MaxRetentionDays
	if span := int(toTime.Sub(fromTime).Hours()/24) + 1; span > maxDays {
		return nil, fmt.Errorf("range too large")
	}

	stats, err := storage.GetAppStatsInRange(from, to)
	if err != nil {
		return nil, err
	}
	stats = filterAppStats(stats)
	limits := system.GetAppTimeLimits()
	apps := aggregateApps(stats, limits)
	total := 0
	for _, s := range stats {
		total += s.TotalDurationSecs
	}

	sites := []siteEntry{}
	if browserStats, err := storage.GetBrowserStatsInRange(from, to); err == nil {
		sites = aggregateSites(browserStats)
	}

	series := buildDaySeries(fromTime, toTime, stats)
	peak := (*dayEntry)(nil)
	for i := range series {
		if peak == nil || series[i].Seconds > peak.Seconds {
			peak = &series[i]
		}
	}

	span := int(toTime.Sub(fromTime).Hours()/24) + 1
	prevTo := fromTime.AddDate(0, 0, -1)
	prevFrom := prevTo.AddDate(0, 0, -(span - 1))
	prevTotal := 0
	if prevStats, err := storage.GetAppStatsInRange(prevFrom.Format("2006-01-02"), prevTo.Format("2006-01-02")); err == nil {
		for _, s := range filterAppStats(prevStats) {
			prevTotal += s.TotalDurationSecs
		}
	}

	var hourly []int
	if from == to {
		if buckets, err := storage.GetHourlyUsageToday(from); err == nil {
			hourly = buckets[:]
		}
	}

	sessions := []sessionEntry{}
	if rows, err := storage.GetSessionsPaginated(60, 0, from, to); err == nil {
		sessions = toSessionEntries(rows)
	}

	return &statsData{
		From:      from,
		To:        to,
		Days:      span,
		Total:     total,
		PrevTotal: prevTotal,
		AvgPerDay: total / maxInt(1, span),
		Apps:      apps,
		Sites:     sites,
		Series:    series,
		Hourly:    hourly,
		Sessions:  sessions,
		PeakDay:   peak,
	}, nil
}

func aggregateApps(stats []storage.AppDailyStat, limits map[string]int) []appEntry {
	byKey := make(map[string]*appEntry)
	var order []string
	for _, s := range stats {
		key := s.AppID
		if key == "" {
			key = s.ExeName + "\x00" + s.AppName
		}
		e, ok := byKey[key]
		if !ok {
			name := s.AppName
			if name == "" {
				name = system.ResolveDisplayName(s.AppID, s.ExeName, "")
			}
			e = &appEntry{Name: name, Exe: s.ExeName, AppID: s.AppID}
			byKey[key] = e
			order = append(order, key)
		}
		e.Seconds += s.TotalDurationSecs
		e.Opens += s.OpenCount
		if e.Name == "" || e.Name == system.TitleCaseExe(e.Exe) {
			if s.AppName != "" {
				e.Name = s.AppName
			}
		}
		if limitKey, ok := system.MatchLimitKey(limits, s.AppID, s.ExeName); ok {
			e.LimitMinutes = limits[limitKey]
			e.LimitKey = limitKey
		}
	}

	out := make([]appEntry, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds > out[j].Seconds
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].AppID < out[j].AppID
	})
	return out
}

// filterAppStats removes applications that DeepFocus never counts: its own
// binaries, Windows shell/logon surfaces, and anything the user excluded. The
// tracker already refuses to start sessions for these; filtering the read path
// also hides rows left by older versions and makes the change visible at once.
func filterAppStats(stats []storage.AppDailyStat) []storage.AppDailyStat {
	if len(stats) == 0 {
		return stats
	}
	out := make([]storage.AppDailyStat, 0, len(stats))
	for _, s := range stats {
		if system.IsWhitelisted(s.ExeName) || system.IsWhitelisted(s.AppName) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func aggregateSites(stats []storage.AppDailyStat) []siteEntry {
	byName := make(map[string]int)
	var order []string
	groupByDomain := storage.GetDomainGrouping()
	for _, s := range stats {
		if s.AppName == "" {
			continue
		}
		name := coreapi.NormalizeSiteName(s.AppName, groupByDomain)
		if name == "" {
			continue
		}
		if _, ok := byName[name]; !ok {
			order = append(order, name)
		}
		byName[name] += s.TotalDurationSecs
	}
	out := make([]siteEntry, 0, len(order))
	for _, name := range order {
		out = append(out, siteEntry{Name: name, Seconds: byName[name]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds > out[j].Seconds
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func buildDaySeries(from, to time.Time, stats []storage.AppDailyStat) []dayEntry {
	totals := make(map[string]int)
	for _, s := range stats {
		totals[s.Date] += s.TotalDurationSecs
	}
	today := storage.Today()
	out := []dayEntry{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		out = append(out, dayEntry{
			Date:    key,
			Seconds: totals[key],
			IsToday: key == today,
			Weekday: int(d.Weekday()),
			Weekend: d.Weekday() == time.Saturday || d.Weekday() == time.Sunday,
			Future:  key > today,
		})
	}
	return out
}

func recentSessions(date string, limit int) []sessionEntry {
	rows, err := storage.GetSessionsPaginated(limit, 0, date, date)
	if err != nil {
		return []sessionEntry{}
	}
	return toSessionEntries(rows)
}

func toSessionEntries(rows []storage.Session) []sessionEntry {
	out := make([]sessionEntry, 0, len(rows))
	activeStart := int64(0)
	if active, err := storage.GetActiveSession(); err == nil && active != nil {
		activeStart = active.StartTime.Unix()
	}
	for _, s := range rows {
		if system.IsWhitelisted(s.ExeName) || system.IsWhitelisted(s.AppName) {
			continue
		}
		end := s.EndTime.Unix()
		isActive := s.EndTime.IsZero()
		if isActive {
			end = time.Now().Unix()
		}
		out = append(out, sessionEntry{
			App:     s.AppName,
			Exe:     s.ExeName,
			Title:   s.WindowTitle,
			Seconds: s.DurationSecs,
			Start:   s.StartTime.Unix(),
			End:     end,
			Date:    s.Date,
			Active:  isActive || s.StartTime.Unix() == activeStart,
		})
	}
	return out
}

func activeSession() *sessionEntry {
	active, err := storage.GetActiveSession()
	if err != nil || active == nil {
		return nil
	}
	if system.IsWhitelisted(active.ExeName) || system.IsWhitelisted(active.AppName) {
		return nil
	}
	now := time.Now()
	// A daemon crash leaves the row behind. Treat it as stale rather than
	// showing a session that keeps growing while nothing is being tracked.
	if active.LastSeen.IsZero() || now.Sub(active.LastSeen) > 2*time.Minute {
		return nil
	}
	end := now
	if active.LastSeen.Before(end) {
		end = active.LastSeen
	}
	elapsed := int(end.Sub(active.StartTime).Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	return &sessionEntry{
		App:     active.AppName,
		Exe:     active.ExeName,
		Title:   active.WindowTitle,
		Seconds: elapsed,
		Start:   active.StartTime.Unix(),
		End:     end.Unix(),
		Date:    active.Date,
		Active:  true,
	}
}

func buildSiteLimitEntries() []siteLimitEntry {
	limits := system.GetSiteTimeLimits()
	out := make([]siteLimitEntry, 0, len(limits))
	for key, minutes := range limits {
		used := coreapi.SiteUsageTodaySeconds(key)
		percent := 0
		if minutes > 0 {
			percent = used * 100 / (minutes * 60)
		}
		status := "ok"
		if percent >= 95 {
			status = "over"
		} else if percent >= 80 {
			status = "warn"
		}
		out = append(out, siteLimitEntry{
			Key:         key,
			Label:       coreapi.SiteClassDisplay(key),
			Minutes:     minutes,
			UsedSeconds: used,
			Percent:     percent,
			Status:      status,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func (a *app) buildLimits() ([]limitEntry, error) {
	limits := system.GetAppTimeLimits()
	openLimits := system.GetAppOpenLimits()
	apps := []appEntry{}
	if stats, err := storage.GetAppStatsInRange(storage.Today(), storage.Today()); err == nil {
		apps = aggregateApps(stats, limits)
	}
	out := buildLimitEntries(limits, apps)
	for i := range out {
		out[i].Mode = "time"
	}
	names := make(map[string]string)
	for _, app := range apps {
		if app.AppID != "" {
			names[app.AppID] = app.Name
		}
		names[strings.ToLower(app.Exe)] = app.Name
	}
	for key, limit := range openLimits {
		opens := storage.CountAppOpens(storage.Today(), key)
		percent := 0
		if limit.MaxOpens > 0 {
			percent = opens * 100 / limit.MaxOpens
		}
		status := "ok"
		if opens >= limit.MaxOpens {
			status = "over"
		} else if percent >= 80 {
			status = "warn"
		}
		name := names[key]
		if name == "" {
			name = system.ResolveDisplayName(key, system.ExeNameForLimitKey(key), "")
		}
		out = append(out, limitEntry{
			Key: key, Name: name, Exe: system.ExeNameForLimitKey(key), AppID: key,
			Minutes: limit.MinutesPerOpen, Mode: "opens", MaxOpens: limit.MaxOpens,
			MinutesPerOpen: limit.MinutesPerOpen, OpensUsed: opens,
			Percent: percent, Status: status,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status == "over" || (out[i].Status == "warn" && out[j].Status == "ok")
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func buildLimitEntries(limits map[string]int, apps []appEntry) []limitEntry {
	names := make(map[string]string)
	for _, app := range apps {
		if app.AppID != "" {
			names[app.AppID] = app.Name
		}
		names[strings.ToLower(app.Exe)] = app.Name
	}
	out := make([]limitEntry, 0, len(limits))
	for key, minutes := range limits {
		used := storage.GetAppUsageTodaySecondsForConfig(key)
		percent := 0
		if minutes > 0 {
			percent = used * 100 / (minutes * 60)
		}
		status := "ok"
		if percent >= 95 {
			status = "over"
		} else if percent >= 80 {
			status = "warn"
		}
		name := names[key]
		if name == "" {
			name = system.ResolveDisplayName(key, key, "")
		}
		out = append(out, limitEntry{
			Key:         key,
			Name:        name,
			Exe:         key,
			AppID:       key,
			Minutes:     minutes,
			UsedSeconds: used,
			Percent:     percent,
			Status:      status,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status == "over" || (out[i].Status == "warn" && out[j].Status == "ok")
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// limitSuggestions returns today's apps with the key a limit should be stored
// under. A specific install id scopes the limit to that install; the bare exe
// name is offered as a wildcard when the install id is unavailable.
func limitSuggestions() []map[string]interface{} {
	stats, err := storage.GetAppStatsInRange(storage.Today(), storage.Today())
	if err != nil {
		return []map[string]interface{}{}
	}
	limits := system.GetAppTimeLimits()
	apps := aggregateApps(stats, limits)
	out := make([]map[string]interface{}, 0, len(apps))
	for _, app := range apps {
		key := app.AppID
		if key == "" {
			key = strings.ToLower(app.Exe)
		}
		out = append(out, map[string]interface{}{
			"name":     app.Name,
			"key":      key,
			"exe":      app.Exe,
			"appId":    app.AppID,
			"seconds":  app.Seconds,
			"limited":  app.LimitKey != "",
			"limitKey": app.LimitKey,
		})
	}
	return out
}

func exportData(jsonOut bool) (string, error) {
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return "", fmt.Errorf("USERPROFILE not set")
	}
	exportDir := filepath.Join(userProfile, "Downloads")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		exportDir = userProfile
	}
	apps, err := storage.GetAppStatsInRange("", "")
	if err != nil {
		return "", err
	}
	if jsonOut {
		path := filepath.Join(exportDir, "deepfocus_export.json")
		b, err := json.MarshalIndent(apps, "", "  ")
		if err != nil {
			return "", err
		}
		return path, os.WriteFile(path, b, 0644)
	}
	path := filepath.Join(exportDir, "deepfocus_export.csv")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"Date", "App Name", "Executable", "Duration (seconds)", "Open Count"}); err != nil {
		return "", err
	}
	for _, app := range apps {
		if err := w.Write([]string{app.Date, app.AppName, app.ExeName, strconv.Itoa(app.TotalDurationSecs), strconv.Itoa(app.OpenCount)}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return path, nil
}

func openDataFolder() error {
	dbPath, err := storage.GetDBPath()
	if err != nil {
		return err
	}
	return exec.Command("explorer.exe", filepath.Dir(dbPath)).Start()
}

func openArchiveFolder() error {
	dir, err := storage.GetArchiveDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return exec.Command("explorer.exe", dir).Start()
}

// openReleasesPage shows the project releases in the user's browser. It never
// downloads or runs an installer: updates are manual by design.
func openReleasesPage() error {
	url := "https://github.com/" + system.RepoOwner + "/" + system.RepoName + "/releases"
	return openExternalURL(url)
}

func openExternalURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
