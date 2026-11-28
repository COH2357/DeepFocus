//go:build windows

package system

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// UpdateInfo describes the latest release found on GitHub. The setup and
// checksum URLs stay private to the updater; the GUI only needs these fields.
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	SetupName string `json:"setupName,omitempty"`
	Notes     string `json:"notes,omitempty"`

	downloadURL string
	checksumURL string
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

var versionDigits = regexp.MustCompile(`\d+(?:\.\d+)+`)

func compareVersions(a, b string) int {
	am := versionDigits.FindString(a)
	bm := versionDigits.FindString(b)
	if am == "" || bm == "" {
		return 0
	}
	ap := strings.Split(am, ".")
	bp := strings.Split(bm, ".")
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bv, _ = strconv.Atoi(bp[i])
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func updateClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// CheckLatestRelease asks GitHub for the latest published release. It is
// intentionally best-effort: no update is reported when the network, release,
// or version metadata cannot be parsed.
func CheckLatestRelease(current string) (UpdateInfo, error) {
	info := UpdateInfo{Current: current}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+RepoOwner+"/"+RepoName+"/releases/latest", nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DeepFocus-Updater")
	resp, err := updateClient().Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&rel); err != nil {
		return info, err
	}
	info.Latest = versionDigits.FindString(rel.TagName + " " + rel.Name)
	if info.Latest == "" {
		info.Latest = strings.TrimPrefix(rel.TagName, "v")
	}
	for _, asset := range rel.Assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, "-setup.exe") || strings.HasSuffix(name, "_setup.exe") {
			info.SetupName = asset.Name
			info.downloadURL = asset.URL
		}
		if strings.HasPrefix(name, "sha256sums") && strings.HasSuffix(name, ".txt") {
			info.checksumURL = asset.URL
		}
	}
	if info.Latest == "" || info.downloadURL == "" {
		return info, fmt.Errorf("latest release has no installer asset")
	}
	info.Available = compareVersions(info.Latest, current) > 0
	info.Notes = strings.TrimSpace(rel.Body)
	return info, nil
}

func updatesDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA is empty")
	}
	dir := filepath.Join(appData, "focusd", "updates")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func downloadTo(url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "DeepFocus-Updater")
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(tmp)
		}
	}()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 200<<20)); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	ok = true
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func checksumFor(text, filename string) string {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if strings.EqualFold(filepath.Base(name), filepath.Base(filename)) {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

// DownloadRelease downloads the installer and verifies its SHA256 checksum
// before returning a path that can be executed.
func DownloadRelease(info UpdateInfo) (string, error) {
	if info.downloadURL == "" || info.SetupName == "" {
		return "", fmt.Errorf("missing installer metadata")
	}
	if info.checksumURL == "" {
		return "", fmt.Errorf("latest release has no SHA256SUMS asset")
	}
	dir, err := updatesDir()
	if err != nil {
		return "", err
	}
	setupPath := filepath.Join(dir, filepath.Base(info.SetupName))
	checksumPath := filepath.Join(dir, "SHA256SUMS-"+info.Latest+".txt")
	if err := downloadTo(info.checksumURL, checksumPath); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", err
	}
	want := checksumFor(string(raw), info.SetupName)
	if want == "" {
		return "", fmt.Errorf("checksum entry for %s is missing", info.SetupName)
	}
	if got, err := fileSHA256(setupPath); err == nil && got == want {
		return setupPath, nil
	}
	if err := downloadTo(info.downloadURL, setupPath); err != nil {
		return "", err
	}
	got, err := fileSHA256(setupPath)
	if err != nil {
		return "", err
	}
	if got != want {
		os.Remove(setupPath)
		return "", fmt.Errorf("installer checksum mismatch")
	}
	return setupPath, nil
}

// LaunchInstaller starts a detached helper. The helper waits for the current
// GUI to exit, runs the NSIS installer silently, then reopens DeepFocus. The
// helper lives in %TEMP% because the installer stops every process under the
// install directory.
func LaunchInstaller(setupPath string) error {
	gui := filepath.Join(os.Getenv("APPDATA"), "focusd", "deepfocus_gui.exe")
	if _, err := os.Stat(gui); err != nil {
		gui = filepath.Join(GetInstallDir(), "deepfocus_gui.exe")
	}
	if _, err := os.Stat(setupPath); err != nil {
		return err
	}
	script := filepath.Join(os.TempDir(), fmt.Sprintf("deepfocus-update-%d.cmd", time.Now().UnixNano()))
	content := "@echo off\r\n" +
		"ping 127.0.0.1 -n 3 >nul\r\n" +
		"\"" + setupPath + "\" /S\r\n" +
		"start \"\" \"" + gui + "\"\r\n" +
		"del \"%~f0\" >nul 2>&1\r\n"
	if err := os.WriteFile(script, []byte(content), 0o600); err != nil {
		return err
	}
	cmd := exec.Command("cmd.exe", "/c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		os.Remove(script)
		return err
	}
	return nil
}
