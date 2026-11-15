package gui

import (
	"errors"
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// clientError marks an input problem so handleAPI can answer 400 instead of
// 500; the message is shown directly in the UI.
type clientError struct{ msg string }

func (e clientError) Error() string { return e.msg }

func badRequest(format string, args ...interface{}) error {
	return clientError{msg: fmt.Sprintf(format, args...)}
}

type pomodoroData struct {
	Active           bool `json:"active"`
	RemainingSeconds int  `json:"remainingSeconds"`
	TotalMinutes     int  `json:"totalMinutes"`
	Configured       int  `json:"configuredMinutes"`
}

type bootstrapData struct {
	Version     string                   `json:"version"`
	Settings    settingsData             `json:"settings"`
	Overview    *overviewData            `json:"overview"`
	Pomodoro    pomodoroData             `json:"pomodoro"`
	Limits      []limitEntry             `json:"limits"`
	Suggestions []map[string]interface{} `json:"suggestions"`
	Platform    platformInfo             `json:"platform"`
}

type platformInfo struct {
	OS             string `json:"os"`
	MediaDetection bool   `json:"mediaDetection"`
	CanExport      bool   `json:"canExport"`
	CanOpenData    bool   `json:"canOpenData"`
	CanUpdate      bool   `json:"canUpdate"`
	SupportsDaemon bool   `json:"supportsDaemon"`
}

type actionRequest struct {
	Action            string `json:"action"`
	Value             string `json:"value"`
	Key               string `json:"key"`
	Format            string `json:"format"`
	Confirm           string `json:"confirm"`
	Minutes           int    `json:"minutes"`
	Text              string `json:"text"`
	ID                int64  `json:"id"`
	Language          string `json:"language"`
	Days              int    `json:"days"`
	Enabled           *bool  `json:"enabled"`
	ThresholdMinutes  *int   `json:"thresholdMinutes"`
	TrackWhileIdle    *bool  `json:"trackWhileIdle"`
	TrackWhilePlaying *bool  `json:"trackWhilePlaying"`
	RedactTitles      *bool  `json:"redactTitles"`
	Dark              *bool  `json:"dark"`
}

type actionResponse struct {
	OK           bool   `json:"ok"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
	Path         string `json:"path,omitempty"`
	Running      bool   `json:"running,omitempty"`
	ArchiveCount int    `json:"archiveCount,omitempty"`
}

func (a *app) bootstrap(w http.ResponseWriter, r *http.Request) error {
	overview, err := a.buildOverview(storage.Today())
	if err != nil {
		return err
	}
	limits, err := a.buildLimits()
	if err != nil {
		return err
	}
	return writeJSON(w, bootstrapData{
		Version:     a.version,
		Settings:    a.settingsSnapshot(),
		Overview:    overview,
		Pomodoro:    readPomodoro(),
		Limits:      limits,
		Suggestions: limitSuggestions(),
		Platform: platformInfo{
			OS:             "windows",
			MediaDetection: mediaDetectionAvailable,
			CanExport:      true,
			CanOpenData:    true,
			CanUpdate:      true,
			SupportsDaemon: true,
		},
	})
}

func (a *app) overview(w http.ResponseWriter, r *http.Request) error {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = storage.Today()
	}
	data, err := a.buildOverview(date)
	if err != nil {
		return badRequest("%s", err.Error())
	}
	return writeJSON(w, data)
}

func (a *app) stats(w http.ResponseWriter, r *http.Request) error {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		to = storage.Today()
		from = time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	}
	data, err := a.buildStats(from, to)
	if err != nil {
		return badRequest("%s", err.Error())
	}
	return writeJSON(w, data)
}

func (a *app) sessions(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}
	from := q.Get("from")
	to := q.Get("to")
	if from == "" {
		from = storage.Today()
	}
	if to == "" {
		to = from
	}
	rows, err := storage.GetSessionsPaginated(limit, offset, from, to)
	if err != nil {
		return err
	}
	return writeJSON(w, map[string]interface{}{
		"sessions": toSessionEntries(rows),
		"limit":    limit,
		"offset":   offset,
	})
}

func (a *app) limits(w http.ResponseWriter, r *http.Request) error {
	entries, err := a.buildLimits()
	if err != nil {
		return err
	}
	events, _ := storage.ListBlockEvents(30)
	blocked, bypassed := storage.BlockStatsToday()
	return writeJSON(w, map[string]interface{}{
		"limits":        entries,
		"suggestions":   limitSuggestions(),
		"sites":         buildSiteLimitEntries(),
		"events":        events,
		"blockedToday":  blocked,
		"bypassedToday": bypassed,
	})
}

func (a *app) settings(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, a.settingsSnapshot())
}

func (a *app) quotes(w http.ResponseWriter, r *http.Request) error {
	quotes, err := storage.ListBlockQuotes()
	if err != nil {
		return err
	}
	return writeJSON(w, map[string]interface{}{"quotes": quotes})
}

func (a *app) pomodoro(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, readPomodoro())
}

func readPomodoro() pomodoroData {
	active, remaining, total := coreapi.GetPomodoroStatus()
	seconds := int(remaining / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	return pomodoroData{
		Active:           active,
		RemainingSeconds: seconds,
		TotalMinutes:     total,
		Configured:       system.GetPomodoroMinutes(),
	}
}

func (a *app) action(w http.ResponseWriter, r *http.Request) error {
	var req actionRequest
	if err := decodeJSON(r, &req); err != nil {
		return badRequest("invalid request body")
	}
	resp, err := a.runAction(req)
	if err != nil {
		return err
	}
	return writeJSON(w, resp)
}

func (a *app) runAction(req actionRequest) (actionResponse, error) {
	ok := actionResponse{OK: true, Code: req.Action}
	switch req.Action {
	case "pomodoro-start":
		minutes := req.Minutes
		if minutes <= 0 {
			minutes = system.GetPomodoroMinutes()
		}
		if minutes < 1 || minutes > 1440 {
			return ok, badRequest("minutes must be between 1 and 1440")
		}
		if err := coreapi.StartPomodoro(minutes); err != nil {
			return ok, badRequest("%s", err.Error())
		}
		return ok, nil

	case "pomodoro-stop":
		if err := coreapi.StopPomodoro(); err != nil {
			return ok, badRequest("%s", err.Error())
		}
		return ok, nil

	case "pause":
		return ok, storage.SetPaused(true)

	case "resume":
		return ok, storage.SetPaused(false)

	case "daemon-start":
		if system.CountInstancesInDir(system.DaemonProcessName, system.GetInstallDir()) > 0 {
			return ok, badRequest("tracker is already running")
		}
		pid, err := system.StartDaemon()
		if err != nil {
			return ok, badRequest("cannot start tracker: %v", err)
		}
		ok.Running = true
		ok.Message = fmt.Sprintf("pid %d", pid)
		return ok, nil

	case "daemon-stop":
		if system.CountInstancesInDir(system.DaemonProcessName, system.GetInstallDir()) == 0 {
			return ok, nil
		}
		coreapi.SendIPCCmd("stop")
		for i := 0; i < 60; i++ {
			if system.CountInstancesInDir(system.DaemonProcessName, system.GetInstallDir()) == 0 {
				return ok, nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return ok, badRequest("tracker did not stop in time")

	case "autostart-enable":
		return ok, system.EnableAutoStart()

	case "autostart-disable":
		return ok, system.DisableAutoStart()

	case "retention":
		if req.Days < storage.MinRetentionDays || req.Days > storage.MaxRetentionDays {
			return ok, badRequest("retention must be between %d and %d days",
				storage.MinRetentionDays, storage.MaxRetentionDays)
		}
		if err := storage.SetRetentionDays(req.Days); err != nil {
			return ok, err
		}
		storage.MaintainHistory()
		return ok, nil

	case "whitelist-add":
		value := strings.TrimSpace(req.Value)
		if value == "" {
			return ok, badRequest("executable name is required")
		}
		if !strings.HasSuffix(strings.ToLower(value), ".exe") {
			value += ".exe"
		}
		return ok, system.AddWhitelistApp(value)

	case "whitelist-remove":
		return ok, system.RemoveWhitelistApp(strings.TrimSpace(req.Value))

	case "browser-add":
		value := strings.TrimSpace(req.Value)
		if value == "" {
			return ok, badRequest("browser name is required")
		}
		if !strings.HasSuffix(strings.ToLower(value), ".exe") {
			value += ".exe"
		}
		return ok, system.AddCustomBrowser(value)

	case "browser-remove":
		return ok, system.RemoveCustomBrowser(strings.TrimSpace(req.Value))

	case "limit-set":
		key := strings.TrimSpace(req.Key)
		if key == "" {
			return ok, badRequest("an app key is required")
		}
		if req.Minutes < 1 || req.Minutes > 1440 {
			return ok, badRequest("limit must be between 1 and 1440 minutes")
		}
		return ok, system.SetAppTimeLimit(key, req.Minutes)

	case "limit-delete":
		return ok, system.SetAppTimeLimit(strings.TrimSpace(req.Key), 0)

	case "language":
		return ok, system.SetLanguage(req.Language)

	case "site-limit-set":
		class, err := coreapi.SiteClassFromInput(req.Key)
		if err != nil {
			return ok, badRequest("%s", err.Error())
		}
		if req.Minutes < 1 || req.Minutes > 1440 {
			return ok, badRequest("limit must be between 1 and 1440 minutes")
		}
		if err := system.SetSiteTimeLimit(class, req.Minutes); err != nil {
			return ok, err
		}
		ok.Code = class
		return ok, nil

	case "site-limit-delete":
		class, err := coreapi.SiteClassFromInput(req.Key)
		if err != nil {
			return ok, badRequest("%s", err.Error())
		}
		return ok, system.SetSiteTimeLimit(class, 0)

	case "quote-add":
		if _, err := storage.AddBlockQuote(req.Text); err != nil {
			return ok, badRequest("%s", err.Error())
		}
		return ok, nil

	case "quote-update":
		if err := storage.UpdateBlockQuote(req.ID, req.Text); err != nil {
			return ok, badRequest("%s", err.Error())
		}
		return ok, nil

	case "quote-delete":
		if err := storage.DeleteBlockQuote(req.ID); err != nil {
			if errors.Is(err, storage.ErrLastQuote) {
				return ok, badRequest("at least one quote is required")
			}
			return ok, err
		}
		return ok, nil

	case "break-reminder":
		if req.Enabled == nil {
			return ok, badRequest("enabled is required")
		}
		minutes := req.Minutes
		if minutes < 1 || minutes > 600 {
			return ok, badRequest("break interval must be between 1 and 600 minutes")
		}
		return ok, system.SetBreakReminder(*req.Enabled, minutes)

	case "pomodoro-minutes":
		if req.Minutes < 1 || req.Minutes > 1440 {
			return ok, badRequest("duration must be between 1 and 1440 minutes")
		}
		return ok, system.SetPomodoroMinutes(req.Minutes)

	case "idle":
		if req.ThresholdMinutes == nil || req.TrackWhileIdle == nil || req.TrackWhilePlaying == nil {
			return ok, badRequest("idle settings are incomplete")
		}
		if *req.ThresholdMinutes < 0 || *req.ThresholdMinutes > system.MaxIdleThresholdMinutes {
			return ok, badRequest("idle threshold must be between 0 and %d minutes", system.MaxIdleThresholdMinutes)
		}
		if err := system.SetIdleThreshold(*req.ThresholdMinutes); err != nil {
			return ok, err
		}
		if err := system.SetTrackWhileIdle(*req.TrackWhileIdle); err != nil {
			return ok, err
		}
		return ok, system.SetTrackWhilePlaying(*req.TrackWhilePlaying)

	case "domain-grouping":
		if req.Enabled == nil {
			return ok, badRequest("enabled is required")
		}
		return ok, storage.SetDomainGrouping(*req.Enabled)

	case "keep-history":
		if req.Enabled == nil {
			return ok, badRequest("enabled is required")
		}
		if err := storage.SetKeepHistory(*req.Enabled); err != nil {
			return ok, err
		}
		if *req.Enabled {
			if _, err := storage.ArchiveCompletedWeeks(time.Now()); err != nil {
				// Don't leave the switch on when the archive failed: the user
				// would believe their history is safe while retention still runs.
				_ = storage.SetKeepHistory(false)
				return ok, err
			}
		}
		ok.ArchiveCount, _ = storage.ArchiveSummary()
		return ok, nil

	case "open-archive":
		return ok, openArchiveFolder()

	case "close-to-tray":
		if req.Enabled == nil {
			return ok, badRequest("enabled is required")
		}
		return ok, system.SetCloseToTray(*req.Enabled)

	case "redact-titles":
		if req.RedactTitles == nil {
			return ok, badRequest("value is required")
		}
		return ok, system.SetRedactTitles(*req.RedactTitles)

	case "export":
		jsonOut := strings.EqualFold(req.Format, "json")
		path, err := exportData(jsonOut)
		if err != nil {
			return ok, err
		}
		ok.Path = path
		if jsonOut {
			ok.Code = "export-json"
		} else {
			ok.Code = "export-csv"
		}
		return ok, nil

	case "open-data-folder":
		return ok, openDataFolder()

	case "update":
		return ok, openReleasesPage()

	case "theme":
		if req.Dark == nil {
			return ok, badRequest("value is required")
		}
		setTitleBarTheme(*req.Dark)
		return ok, nil

	case "wipe":
		if req.Confirm != "DELETE" {
			return ok, badRequest("type DELETE to confirm")
		}
		return ok, storage.ClearAllTrackingData()

	default:
		return ok, badRequest("unknown action %q", req.Action)
	}
}

func actionError(err error) (clientError, bool) {
	var ce clientError
	return ce, errors.As(err, &ce)
}
