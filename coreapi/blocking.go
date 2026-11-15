package coreapi

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"focusd/storage"
	"focusd/system"
)

// SiteClassFromInput turns a URL, a domain, or a bare site name into the same
// stable class used by browser tracking. bilibili.com, www.bilibili.com and a
// deep /video link all become "bilibili"; b23.tv maps through the catalog too.
func SiteClassFromInput(input string) (string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", fmt.Errorf("site is required")
	}
	candidate := raw
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	u, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("invalid site")
	}
	host := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(u.Hostname(), "www."), "."))
	if host == "" {
		// Accept bare labels such as "bilibili" as a class.
		host = strings.ToLower(strings.Trim(raw, " /"))
		host = strings.SplitN(host, "/", 2)[0]
	}
	if host == "" || strings.ContainsAny(host, " \t") {
		return "", fmt.Errorf("invalid site")
	}
	if name := SiteNameFor(host); name != "" {
		return strings.ToLower(name), nil
	}
	if !strings.Contains(host, ".") {
		host = strings.Trim(host, ".")
		if host == "" {
			return "", fmt.Errorf("invalid site")
		}
		return host, nil
	}
	domain := registrableDomain(host)
	label := strings.Split(domain, ".")[0]
	if label == "" {
		return "", fmt.Errorf("invalid site")
	}
	return strings.ToLower(label), nil
}

// SiteClassForTitle returns the class for a browser window title. Catalog
// matches win, then the best-effort domain extraction, matching the dashboard's
// "group sites by domain" behaviour.
func SiteClassForTitle(title string) string {
	name := NormalizeSiteName(title, true)
	return strings.ToLower(strings.TrimSpace(name))
}

func SiteClassDisplay(class string) string {
	class = strings.TrimSpace(class)
	if class == "" {
		return ""
	}
	r := []rune(class)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

// SiteUsageTodaySeconds keeps the browser-only total for one site class. The
// unflushed current session is added when it is a browser on the same class.
func SiteUsageTodaySeconds(class string) int {
	class = strings.ToLower(strings.TrimSpace(class))
	if class == "" {
		return 0
	}
	today := storage.Today()
	total := 0
	if rows, err := storage.GetBrowserStatsInRange(today, today); err == nil {
		for _, row := range rows {
			if SiteClassForTitle(row.AppName) == class {
				total += row.TotalDurationSecs
			}
		}
	}
	if active, err := storage.GetActiveSession(); err == nil && active != nil && active.Date == today && system.IsBrowser(active.ExeName) {
		if SiteClassForTitle(active.WindowTitle) == class {
			elapsed := int(time.Since(active.StartTime).Seconds())
			if elapsed > 0 {
				total += elapsed
			}
		}
	}
	return total
}
