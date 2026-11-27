package system

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type AppOpenLimit struct {
	MaxOpens       int `json:"max_opens"`
	MinutesPerOpen int `json:"minutes_per_open"`
}

type UserConfig struct {
	WhitelistApps         []string                `json:"whitelist_apps"`
	BreakReminderEnabled  bool                    `json:"break_reminder_enabled"`
	BreakReminderMinutes  int                     `json:"break_reminder_minutes"`
	AppTimeLimits         map[string]int          `json:"app_time_limits"`
	AppOpenLimits         map[string]AppOpenLimit `json:"app_open_limits"`
	SiteTimeLimits        map[string]int          `json:"site_time_limits"`
	Language              string                  `json:"language"`
	StrongWebMode         bool                    `json:"strong_web_mode"`
	WebAllowlist          []string                `json:"web_allowlist"`
	PomodoroMinutes       int                     `json:"pomodoro_minutes"`
	SnoozeDurationMinutes int                     `json:"snooze_duration_minutes"`
	CustomBrowsers        []string                `json:"custom_browsers"`
	IdleThresholdMinutes  int                     `json:"idle_threshold_minutes"`
	TrackWhileIdle        bool                    `json:"track_while_idle"`
	// TrackWhilePlaying keeps the clock running while media is audible, so
	// watching a video without touching the keyboard or mouse is still counted.
	TrackWhilePlaying bool `json:"track_while_playing"`
	// CloseToTray hides the window instead of quitting when the user closes
	// it, leaving the desktop app available from the notification area.
	CloseToTray bool `json:"close_to_tray"`
	// RedactTitles stores a placeholder instead of window title text.
	RedactTitles bool `json:"redact_titles"`
	// DisplayOverrides renames an app in the UI without touching history,
	// keyed by app id.
	DisplayOverrides map[string]string `json:"display_overrides,omitempty"`
	// SiteOverrides renames a site in the UI, keyed by the cleaned site name.
	SiteOverrides map[string]string `json:"site_overrides,omitempty"`
}

// userConfigFile mirrors UserConfig for decoding. IdleThresholdMinutes is a
// pointer so an absent key is distinguishable from an explicit zero, which
// means "never treat me as idle".
type userConfigFile struct {
	WhitelistApps         []string                `json:"whitelist_apps"`
	BreakReminderEnabled  bool                    `json:"break_reminder_enabled"`
	BreakReminderMinutes  int                     `json:"break_reminder_minutes"`
	AppTimeLimits         map[string]int          `json:"app_time_limits"`
	AppOpenLimits         map[string]AppOpenLimit `json:"app_open_limits"`
	SiteTimeLimits        map[string]int          `json:"site_time_limits"`
	Language              string                  `json:"language"`
	StrongWebMode         bool                    `json:"strong_web_mode"`
	WebAllowlist          []string                `json:"web_allowlist"`
	PomodoroMinutes       int                     `json:"pomodoro_minutes"`
	SnoozeDurationMinutes int                     `json:"snooze_duration_minutes"`
	CustomBrowsers        []string                `json:"custom_browsers"`
	IdleThresholdMinutes  *int                    `json:"idle_threshold_minutes"`
	TrackWhileIdle        *bool                   `json:"track_while_idle"`
	TrackWhilePlaying     *bool                   `json:"track_while_playing"`
	CloseToTray           *bool                   `json:"close_to_tray"`
	RedactTitles          bool                    `json:"redact_titles"`
	DisplayOverrides      map[string]string       `json:"display_overrides"`
	SiteOverrides         map[string]string       `json:"site_overrides"`
}

const (
	// DefaultIdleThresholdMinutes is how long the keyboard and mouse may be
	// idle before time stops accruing.
	DefaultIdleThresholdMinutes = 5
	MinIdleThresholdMinutes     = 0
	MaxIdleThresholdMinutes     = 240
)

var (
	userConfig  *UserConfig
	configMu    sync.RWMutex
	lastModTime time.Time
)

func getUserConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable is empty")
	}
	return filepath.Join(appData, "focusd", "config.json"), nil
}

func defaultUserConfig() *UserConfig {
	return &UserConfig{
		WhitelistApps:         []string{},
		BreakReminderEnabled:  false,
		BreakReminderMinutes:  60,
		AppTimeLimits:         make(map[string]int),
		AppOpenLimits:         make(map[string]AppOpenLimit),
		SiteTimeLimits:        make(map[string]int),
		Language:              "",
		StrongWebMode:         false,
		WebAllowlist:          defaultWebAllowlist(),
		PomodoroMinutes:       25,
		SnoozeDurationMinutes: 60,
		CustomBrowsers:        []string{},
		IdleThresholdMinutes:  DefaultIdleThresholdMinutes,
		TrackWhileIdle:        false,
		TrackWhilePlaying:     true,
		CloseToTray:           true,
		RedactTitles:          false,
		DisplayOverrides:      make(map[string]string),
		SiteOverrides:         make(map[string]string),
	}
}

func defaultWebAllowlist() []string {
	return []string{
		"google.com", "bing.com", "baidu.com", "duckduckgo.com",
		"search.brave.com", "yandex.com", "ecosia.org", "startpage.com",
		"so.com", "sogou.com",
	}
}

func loadFromDisk() {
	configMu.Lock()
	defer configMu.Unlock()
	reloadFromDiskLocked(false)
}

// reloadFromDiskLocked refreshes the in-memory config from disk. Caller holds
// configMu. With force, the mtime fast path is skipped so a read-modify-write
// always starts from the freshest file.
func reloadFromDiskLocked(force bool) {
	configPath, err := getUserConfigPath()
	if err != nil || configPath == "" {
		if userConfig == nil {
			userConfig = defaultUserConfig()
		}
		return
	}

	fi, err := os.Stat(configPath)
	if err != nil {
		if userConfig == nil {
			userConfig = defaultUserConfig()
		}
		return
	}

	if !force && userConfig != nil && !fi.ModTime().After(lastModTime) {
		return
	}

	lastModTime = fi.ModTime()
	if userConfig == nil {
		userConfig = defaultUserConfig()
	}

	if data, err := os.ReadFile(configPath); err == nil {
		var temp userConfigFile
		if err := json.Unmarshal(data, &temp); err == nil {
			userConfig.WhitelistApps = temp.WhitelistApps
			userConfig.BreakReminderEnabled = temp.BreakReminderEnabled
			userConfig.BreakReminderMinutes = temp.BreakReminderMinutes
			userConfig.AppTimeLimits = temp.AppTimeLimits
			userConfig.AppOpenLimits = temp.AppOpenLimits
			userConfig.SiteTimeLimits = temp.SiteTimeLimits
			userConfig.Language = temp.Language
			userConfig.StrongWebMode = temp.StrongWebMode
			userConfig.WebAllowlist = temp.WebAllowlist
			userConfig.PomodoroMinutes = temp.PomodoroMinutes
			userConfig.SnoozeDurationMinutes = temp.SnoozeDurationMinutes
			userConfig.CustomBrowsers = temp.CustomBrowsers

			// An absent key keeps the current value rather than decoding as
			// "disabled".
			if temp.IdleThresholdMinutes != nil {
				userConfig.IdleThresholdMinutes = *temp.IdleThresholdMinutes
			}
			if temp.TrackWhileIdle != nil {
				userConfig.TrackWhileIdle = *temp.TrackWhileIdle
			}
			if temp.TrackWhilePlaying != nil {
				userConfig.TrackWhilePlaying = *temp.TrackWhilePlaying
			}
			if temp.CloseToTray != nil {
				userConfig.CloseToTray = *temp.CloseToTray
			}
			userConfig.RedactTitles = temp.RedactTitles
			// Overrides are replaced wholesale, never merged. The file omits
			// them when they are empty, and a nil-check merge resurrected
			// entries the file no longer had, so deleting an override in
			// another process never took effect here.
			userConfig.DisplayOverrides = temp.DisplayOverrides
			userConfig.SiteOverrides = temp.SiteOverrides
		} else {
			log.Printf("WARN: failed to parse user config %s: %v", configPath, err)
		}
	}

	if userConfig.AppTimeLimits == nil {
		userConfig.AppTimeLimits = make(map[string]int)
	}
	if userConfig.AppOpenLimits == nil {
		userConfig.AppOpenLimits = make(map[string]AppOpenLimit)
	}
	if len(userConfig.WebAllowlist) == 0 {
		userConfig.WebAllowlist = defaultWebAllowlist()
	}
	if userConfig.SiteTimeLimits == nil {
		userConfig.SiteTimeLimits = make(map[string]int)
	}
	if userConfig.CustomBrowsers == nil {
		userConfig.CustomBrowsers = []string{}
	}
	if userConfig.DisplayOverrides == nil {
		userConfig.DisplayOverrides = make(map[string]string)
	}
	if userConfig.SiteOverrides == nil {
		userConfig.SiteOverrides = make(map[string]string)
	}
	if userConfig.BreakReminderMinutes < 1 {
		userConfig.BreakReminderMinutes = 60
	}
	if userConfig.PomodoroMinutes < 1 {
		userConfig.PomodoroMinutes = 25
	}
	if userConfig.SnoozeDurationMinutes < 1 {
		userConfig.SnoozeDurationMinutes = 60
	}
	if userConfig.IdleThresholdMinutes < MinIdleThresholdMinutes ||
		userConfig.IdleThresholdMinutes > MaxIdleThresholdMinutes {
		userConfig.IdleThresholdMinutes = DefaultIdleThresholdMinutes
	}
}

func saveUserConfigLocked() error {
	if userConfig == nil {
		userConfig = defaultUserConfig()
	}

	configPath, err := getUserConfigPath()
	if err != nil || configPath == "" {
		return err
	}

	dataDir := filepath.Dir(configPath)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(userConfig, "", "  ")
	if err != nil {
		return err
	}

	tempPath := configPath + ".tmp"
	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}
	// Flush to disk before the rename: otherwise a crash can leave the rename
	// pointing at a temp file whose contents never made it out of the cache.
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := os.Rename(tempPath, configPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	if fi, err := os.Stat(configPath); err == nil {
		lastModTime = fi.ModTime()
	}
	return nil
}

// lockUserConfig takes an exclusive lock on a sidecar file next to the config,
// serializing read-modify-write updates across processes (TUI, CLI, daemon).
// Windows releases the lock when the handle closes or the process dies, so a
// crash cannot leave the config wedged.
func lockUserConfig(configPath string) (func(), error) {
	lockPath := configPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	overlapped := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to lock the user config: %w", err)
	}
	return func() {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
		f.Close()
	}, nil
}

// beginConfigUpdate starts a cross-process-safe read-modify-write of the user
// config: it takes the in-process mutex plus the inter-process file lock and
// reloads the on-disk state, so a change another process made while we were
// idle is not silently overwritten. The caller must defer the returned
// release function.
func beginConfigUpdate() (func(), error) {
	configMu.Lock()

	configPath, err := getUserConfigPath()
	if err != nil {
		configMu.Unlock()
		return nil, err
	}
	release, err := lockUserConfig(configPath)
	if err != nil {
		configMu.Unlock()
		return nil, err
	}
	reloadFromDiskLocked(true)
	return func() {
		release()
		configMu.Unlock()
	}, nil
}

// IsWhitelisted reports whether an app should be excluded from tracking. The
// exe name is matched against the normalized config entries so both "foo" and
// "foo.exe" address the same app.
// builtinExcludedApps are never counted. They are either DeepFocus's own
// binaries (a GUI window is not "screen time") or Windows shell/logon surfaces
// that appear in the foreground while the user is not actually working in them.
// The map is keyed by normalizeConfigName, so no .exe suffix.
var builtinExcludedApps = map[string]bool{
	"deepfocus":               true,
	"deepfocus_gui":           true,
	"deepfocus_daemon":        true,
	"focusd":                  true,
	"focusd_gui":              true,
	"focusd_daemon":           true,
	"lockapp":                 true,
	"logonui":                 true,
	"shellexperiencehost":     true,
	"startmenuexperiencehost": true,
	"searchhost":              true,
	"searchapp":               true,
	"textinputhost":           true,
	"applicationframehost":    true,
	"dwm":                     true,
}

// IsBuiltinExcluded reports whether exeName is one of the processes DeepFocus
// always ignores, regardless of the user's exclusion list.
func IsBuiltinExcluded(exeName string) bool {
	return builtinExcludedApps[normalizeConfigName(exeName)]
}

func IsWhitelisted(exeName string) bool {
	if IsBuiltinExcluded(exeName) {
		return true
	}
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()

	target := normalizeConfigName(exeName)
	for _, a := range userConfig.WhitelistApps {
		if normalizeConfigName(a) == target {
			return true
		}
	}
	return false
}

func GetWhitelistApps() []string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := make([]string, len(userConfig.WhitelistApps))
	copy(out, userConfig.WhitelistApps)
	return out
}

func AddWhitelistApp(exeName string) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	exeName = normalizeConfigName(exeName)
	if exeName == "unknown" || exeName == "" {
		return fmt.Errorf("invalid app name")
	}
	for _, a := range userConfig.WhitelistApps {
		if normalizeConfigName(a) == exeName {
			return nil
		}
	}
	userConfig.WhitelistApps = append(userConfig.WhitelistApps, exeName)
	return saveUserConfigLocked()
}

func RemoveWhitelistApp(exeName string) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

	// Normalize exactly like insertion, so "foo" removes the stored "foo.exe".
	target := normalizeConfigName(exeName)

	matched := false
	filtered := userConfig.WhitelistApps[:0]
	for _, a := range userConfig.WhitelistApps {
		if normalizeConfigName(a) == target {
			matched = true
			continue
		}
		filtered = append(filtered, a)
	}
	if !matched {
		return nil
	}
	userConfig.WhitelistApps = filtered
	return saveUserConfigLocked()
}

func GetBreakReminderEnabled() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.BreakReminderEnabled
}

func GetBreakReminderMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.BreakReminderMinutes
}

func SetBreakReminder(enabled bool, minutes int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.BreakReminderEnabled = enabled
	if minutes > 0 {
		userConfig.BreakReminderMinutes = minutes
	}
	return saveUserConfigLocked()
}

func GetAppTimeLimits() map[string]int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	limits := make(map[string]int, len(userConfig.AppTimeLimits))
	for k, v := range userConfig.AppTimeLimits {
		limits[k] = v
	}
	return limits
}

func SetAppTimeLimit(exeName string, minutes int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	exeName = normalizeConfigName(exeName)
	if minutes <= 0 {
		delete(userConfig.AppTimeLimits, exeName)
	} else {
		userConfig.AppTimeLimits[exeName] = minutes
	}
	return saveUserConfigLocked()
}

func GetAppOpenLimits() map[string]AppOpenLimit {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := make(map[string]AppOpenLimit, len(userConfig.AppOpenLimits))
	for k, v := range userConfig.AppOpenLimits {
		out[k] = v
	}
	return out
}

func SetAppOpenLimit(appKey string, maxOpens, minutesPerOpen int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	appKey = normalizeConfigName(appKey)
	if appKey == "" {
		return fmt.Errorf("invalid app key")
	}
	if maxOpens <= 0 || minutesPerOpen <= 0 {
		delete(userConfig.AppOpenLimits, appKey)
	} else {
		userConfig.AppOpenLimits[appKey] = AppOpenLimit{
			MaxOpens:       maxOpens,
			MinutesPerOpen: minutesPerOpen,
		}
	}
	return saveUserConfigLocked()
}

func GetStrongWebMode() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.StrongWebMode
}

func SetStrongWebMode(enabled bool) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.StrongWebMode = enabled
	return saveUserConfigLocked()
}

func GetWebAllowlist() []string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := make([]string, len(userConfig.WebAllowlist))
	copy(out, userConfig.WebAllowlist)
	return out
}

func SetWebAllowlist(entries []string) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	seen := make(map[string]bool)
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.ToLower(strings.TrimSpace(entry))
		entry = strings.TrimPrefix(strings.TrimPrefix(entry, "https://"), "http://")
		entry = strings.Trim(entry, "/ ")
		if entry == "" || seen[entry] {
			continue
		}
		seen[entry] = true
		out = append(out, entry)
	}
	userConfig.WebAllowlist = out
	return saveUserConfigLocked()
}

// ExeNameForLimitKey turns a configured app limit key into the process image
// name used by the process snapshot. App ids look like "msedge|<hash>".
func ExeNameForLimitKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	if i := strings.Index(key, "|"); i > 0 {
		key = key[:i]
	}
	if key == "" {
		return ""
	}
	if !strings.HasSuffix(key, ".exe") {
		key += ".exe"
	}
	return key
}

func GetSiteTimeLimits() map[string]int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	limits := make(map[string]int, len(userConfig.SiteTimeLimits))
	for k, v := range userConfig.SiteTimeLimits {
		limits[k] = v
	}
	return limits
}

// SetSiteTimeLimit stores a browser-site limit for a normalized site class
// (for example "bilibili"). Zero removes it.
func SetSiteTimeLimit(siteClass string, minutes int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	siteClass = strings.ToLower(strings.TrimSpace(siteClass))
	if siteClass == "" {
		return fmt.Errorf("invalid site class")
	}
	if minutes <= 0 {
		delete(userConfig.SiteTimeLimits, siteClass)
	} else {
		userConfig.SiteTimeLimits[siteClass] = minutes
	}
	return saveUserConfigLocked()
}

// GetLanguage returns the explicit UI language, or the Windows UI language
// when the setting is "auto". Notifications, the tray menu and the CLI use the
// same value as the desktop app.
func GetLanguage() string {
	loadFromDisk()
	configMu.RLock()
	lang := strings.ToLower(strings.TrimSpace(userConfig.Language))
	configMu.RUnlock()
	if lang == "zh" || lang == "en" {
		return lang
	}
	return systemLanguage()
}

func SetLanguage(lang string) error {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang != "zh" && lang != "en" && lang != "auto" && lang != "" {
		return fmt.Errorf("invalid language")
	}
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.Language = lang
	return saveUserConfigLocked()
}

// UseChinese reports whether user-facing runtime strings should be Chinese.
func UseChinese() bool {
	return GetLanguage() == "zh"
}

func GetPomodoroMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.PomodoroMinutes
}

func SetPomodoroMinutes(minutes int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.PomodoroMinutes = minutes
	return saveUserConfigLocked()
}

// MatchLimitKey finds the config entry that governs an app. A bare exe name
// acts as a wildcard across every install; a specific app key wins.
func MatchLimitKey(limits map[string]int, appID, exeName string) (string, bool) {
	if appID != "" {
		if _, ok := limits[appID]; ok {
			return appID, true
		}
	}

	bare := normalizeExe(exeName)
	if bare == "" || bare == "unknown" {
		return "", false
	}
	if _, ok := limits[bare]; ok {
		return bare, true
	}

	// Accept the historical ".exe" spelling too.
	withExt := bare + ".exe"
	if _, ok := limits[withExt]; ok {
		return withExt, true
	}

	return "", false
}

// ResolveDisplayName returns the human label for an app, preferring a user
// override, then version info, then the built-in map, then the exe name.
func ResolveDisplayName(appID, exeName, versionLabel string) string {
	if override := GetDisplayOverride(appID); override != "" {
		return override
	}
	if versionLabel != "" {
		return versionLabel
	}
	if mapped, ok := lookupNameMap(exeName); ok {
		return mapped
	}
	return TitleCaseExe(exeName)
}

// GetDisplayOverride returns a user-supplied label for an app, if any.
func GetDisplayOverride(appID string) string {
	if appID == "" {
		return ""
	}
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.DisplayOverrides[appID]
}

// SetDisplayOverride renames an app in the UI. An empty name removes the
// override.
func SetDisplayOverride(appID, name string) error {
	if appID == "" {
		return fmt.Errorf("app id is required")
	}
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

	name = strings.TrimSpace(name)
	if name == "" {
		delete(userConfig.DisplayOverrides, appID)
	} else {
		userConfig.DisplayOverrides[appID] = name
	}
	return saveUserConfigLocked()
}

// GetDisplayOverrides returns a copy of every override, safe to iterate.
func GetDisplayOverrides() map[string]string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := make(map[string]string, len(userConfig.DisplayOverrides))
	for k, v := range userConfig.DisplayOverrides {
		out[k] = v
	}
	return out
}

// lookupNameMap resolves the built-in friendly names.
func lookupNameMap(exeName string) (string, bool) {
	name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(exeName)), ".exe")
	mapped, ok := displayNameMap[name]
	return mapped, ok
}

func GetSnoozeDurationMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.SnoozeDurationMinutes
}

// GetIdleThresholdMinutes returns how many minutes of keyboard and mouse
// inactivity stop time from accruing. Zero means idle time is still tracked.
func GetIdleThresholdMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.IdleThresholdMinutes
}

func GetTrackWhileIdle() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.TrackWhileIdle
}

// GetTrackWhilePlaying reports whether audible media playback keeps the clock
// running when no keyboard or mouse input has been seen for a while.
func GetTrackWhilePlaying() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.TrackWhilePlaying
}

// GetRedactTitles reports whether window titles are stored as a placeholder
// instead of their text.
func GetRedactTitles() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.RedactTitles
}

// SetRedactTitles turns window-title redaction on or off.
func SetRedactTitles(enabled bool) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.RedactTitles = enabled
	return saveUserConfigLocked()
}

// SetIdleThreshold sets the idle cutoff. A negative value disables it, which is
// how a user opts back into counting unattended time.
func SetIdleThreshold(minutes int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	if minutes < MinIdleThresholdMinutes {
		minutes = MinIdleThresholdMinutes
	}
	if minutes > MaxIdleThresholdMinutes {
		minutes = MaxIdleThresholdMinutes
	}
	userConfig.IdleThresholdMinutes = minutes
	return saveUserConfigLocked()
}

// SetTrackWhileIdle keeps recording activity even when the user is away.
func SetTrackWhileIdle(enabled bool) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.TrackWhileIdle = enabled
	return saveUserConfigLocked()
}

// SetTrackWhilePlaying keeps the clock running while media is audible.
func SetTrackWhilePlaying(enabled bool) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.TrackWhilePlaying = enabled
	return saveUserConfigLocked()
}

// GetCloseToTray reports whether closing the desktop window keeps DeepFocus
// running in the notification area.
func GetCloseToTray() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.CloseToTray
}

// SetCloseToTray chooses between hiding the window and quitting on close.
func SetCloseToTray(enabled bool) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.CloseToTray = enabled
	return saveUserConfigLocked()
}

var defaultBrowsers = map[string]bool{
	"chrome.exe":     true,
	"firefox.exe":    true,
	"msedge.exe":     true,
	"edge.exe":       true,
	"brave.exe":      true,
	"opera.exe":      true,
	"operagx.exe":    true,
	"sidekick.exe":   true,
	"orion.exe":      true,
	"tor.exe":        true,
	"torbrowser.exe": true,
	"coccoc.exe":     true,
	"vivaldi.exe":    true,
	"waterfox.exe":   true,
	"arc.exe":        true,
	"iexplore.exe":   true,
	"safari.exe":     true,
	"whale.exe":      true,
	"yandex.exe":     true,
	"thorium.exe":    true,
	"librewolf.exe":  true,
	"chromium.exe":   true,
	"floorp.exe":     true,
	"zen.exe":        true,
}

func IsBrowser(exeName string) bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()

	exeLower := strings.ToLower(exeName)
	if defaultBrowsers[exeLower] {
		return true
	}
	for _, cb := range userConfig.CustomBrowsers {
		if strings.ToLower(cb) == exeLower {
			return true
		}
	}
	return false
}

func GetCustomBrowsersList() []string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()

	res := make([]string, len(userConfig.CustomBrowsers))
	copy(res, userConfig.CustomBrowsers)
	return res
}

func AddCustomBrowser(exeName string) error {
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	if exeName == "" {
		return fmt.Errorf("invalid browser name")
	}
	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}

	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

	for _, b := range userConfig.CustomBrowsers {
		if b == exeName {
			return fmt.Errorf("%s is already in custom list", exeName)
		}
	}

	if defaultBrowsers[exeName] {
		return fmt.Errorf("%s is already a default browser", exeName)
	}

	userConfig.CustomBrowsers = append(userConfig.CustomBrowsers, exeName)
	return saveUserConfigLocked()
}

func RemoveCustomBrowser(exeName string) error {
	exeName = strings.ToLower(strings.TrimSpace(exeName))

	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}
	found := false
	var newList []string
	for _, b := range userConfig.CustomBrowsers {
		if b == exeName {
			found = true
			continue
		}
		newList = append(newList, b)
	}

	if !found {
		return fmt.Errorf("%s not found in custom list (cannot remove default browsers)", exeName)
	}

	userConfig.CustomBrowsers = newList
	return saveUserConfigLocked()
}
