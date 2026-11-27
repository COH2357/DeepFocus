package nativehost

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
)

const extensionID = "jkepdhaoafljfalimepgfohpgfmfmmbk"

type request struct {
	Type    string `json:"type"`
	Browser string `json:"browser"`
	URL     string `json:"url"`
	Site    string `json:"site"`
	Minutes int    `json:"minutes"`
	Reason  string `json:"reason"`
	Version string `json:"version"`
	TabID   int    `json:"tabId"`
}

type response struct {
	OK          bool            `json:"ok"`
	Error       string          `json:"error,omitempty"`
	Action      string          `json:"action,omitempty"`
	Site        string          `json:"site,omitempty"`
	Label       string          `json:"label,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	UsedSeconds int             `json:"usedSeconds,omitempty"`
	LimitMinute int             `json:"limitMinutes,omitempty"`
	Until       int64           `json:"until,omitempty"`
	Config      *configResponse `json:"config,omitempty"`
}

type configResponse struct {
	StrongMode bool           `json:"strongMode"`
	Allowlist  []string       `json:"allowlist"`
	SiteLimits map[string]int `json:"siteLimits"`
	Version    string         `json:"version"`
	Extension  string         `json:"extensionId"`
}

func Run() error {
	if err := storage.Init(); err != nil {
		return err
	}
	defer storage.Close()

	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for {
		raw, err := readMessage(reader)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var req request
		if err := json.Unmarshal(raw, &req); err != nil {
			writeMessage(writer, response{OK: false, Error: "invalid message"})
			continue
		}
		resp := handle(req)
		if err := writeMessage(writer, resp); err != nil {
			return err
		}
	}
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return nil, err
	}
	if size == 0 || size > 1024*1024 {
		return nil, fmt.Errorf("invalid native message size")
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeMessage(w *bufio.Writer, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.Flush()
}

func handle(req request) response {
	if req.Browser == "" {
		req.Browser = "chrome"
	}
	if req.Type != "navigation" && req.Type != "heartbeat" && req.Type != "hello" && req.Type != "request_access" && req.Type != "status" {
		return response{OK: false, Error: "unknown message type"}
	}
	if req.Type == "request_access" {
		return grant(req)
	}
	_ = storage.SetExtensionSeen(strings.ToLower(req.Browser), req.Version, time.Now())
	if req.Type == "hello" || req.Type == "heartbeat" || req.Type == "status" {
		return response{OK: true, Config: currentConfig(req.Browser)}
	}
	return decide(req)
}

func currentConfig(browser string) *configResponse {
	return &configResponse{
		StrongMode: system.GetStrongWebMode(),
		Allowlist:  system.GetWebAllowlist(),
		SiteLimits: system.GetSiteTimeLimits(),
		Version:    "1",
		Extension:  extensionID,
	}
}

func grant(req request) response {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return response{OK: false, Error: "a reason is required"}
	}
	if req.Minutes < 1 || req.Minutes > 1440 {
		return response{OK: false, Error: "allow duration must be between 1 and 1440 minutes"}
	}
	site := strings.ToLower(strings.TrimSpace(req.Site))
	if site == "" {
		site = siteFromURL(req.URL)
	}
	if site == "" {
		return response{OK: false, Error: "a site is required"}
	}
	now := time.Now()
	until := now.Add(time.Duration(req.Minutes) * time.Minute)
	if err := storage.SetSiteAllowance(site, until, reason); err != nil {
		return response{OK: false, Error: err.Error()}
	}
	limit := system.GetSiteTimeLimits()[site]
	used := coreapi.SiteUsageTodaySeconds(site)
	_ = storage.RecordBlockEvent("site", coreapi.SiteClassDisplay(site), "bypassed", limit, used, reason)
	_ = storage.SetExtensionSeen(strings.ToLower(req.Browser), req.Version, now)
	return response{OK: true, Site: site, Until: until.Unix()}
}

func decide(req request) response {
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil {
		return response{OK: true, Action: "allow"}
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return response{OK: true, Action: "allow"}
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return response{OK: true, Action: "allow"}
	}
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return response{OK: true, Action: "allow"}
	}
	site := siteFromURL(req.URL)
	if site == "" {
		return response{OK: true, Action: "allow"}
	}
	if isAllowlisted(host, site) {
		return response{OK: true, Action: "allow"}
	}
	now := time.Now()
	if _, ok := storage.SiteAllowanceUntil(site, now); ok {
		return response{OK: true, Action: "allow"}
	}
	limits := system.GetSiteTimeLimits()
	limit, limited := limits[site]
	if !limited || limit <= 0 {
		// Strong mode only governs sites the user explicitly added to the
		// website-limit list. Everything else is ordinary browsing.
		return response{OK: true, Action: "allow"}
	}
	used := coreapi.SiteUsageTodaySeconds(site)
	if used < limit*60 {
		// Never interrupt a site before its daily allowance is actually used up.
		return response{OK: true, Action: "allow", Site: site, Label: coreapi.SiteClassDisplay(site), UsedSeconds: used, LimitMinute: limit}
	}
	reason := "limit"
	if system.GetStrongWebMode() {
		reason = "strong"
	}
	return response{OK: true, Action: "block", Site: site, Label: coreapi.SiteClassDisplay(site), Reason: reason, UsedSeconds: used, LimitMinute: limit}
}

func isAllowlisted(host, site string) bool {
	for _, entry := range system.GetWebAllowlist() {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
		if site == entry {
			return true
		}
	}
	return false
}

func siteFromURL(raw string) string {
	site, err := coreapi.SiteClassFromInput(raw)
	if err != nil {
		return ""
	}
	return site
}

// Install registers the native messaging manifests for Chrome and Edge.
// It is called by the installer or `deepfocus_native.exe --install`.
func Install() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(os.Getenv("APPDATA"), "focusd", "native")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	manifest := map[string]interface{}{
		"name":            "com.deepfocus.host",
		"description":     "DeepFocus website supervision host",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": []string{"chrome-extension://" + extensionID + "/"},
	}
	path := filepath.Join(dataDir, "com.deepfocus.host.json")
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	if err := system.SetNativeMessagingRegistration("com.deepfocus.host", path); err != nil {
		return err
	}
	return nil
}
