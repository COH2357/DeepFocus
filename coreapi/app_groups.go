package coreapi

import (
	"focusd/storage"
	"regexp"
	"sort"
	"strings"
)

type groupedBrowserStat struct {
	Category   string
	TotalSecs  int
	SubEntries []subEntry
}

type subEntry struct {
	Title    string
	Duration int
}

var defaultBrowserTitles = map[string]bool{
	"new tab":         true,
	"new page":        true,
	"start page":      true,
	"home":            true,
	"blank page":      true,
	"speed dial":      true,
	"google chrome":   true,
	"mozilla firefox": true,
	"microsoft edge":  true,
	"zen browser":     true,
	"brave":           true,
	"opera":           true,
	"vivaldi":         true,
	"chrome":          true,
	"firefox":         true,
	"edge":            true,
}

// groupBrowserStats rolls site entries up into categories. An entry that is
// already a category name becomes its own group rather than a sub-entry.
func groupBrowserStats(stats []storage.AppDailyStat, groupByDomain bool) []groupedBrowserStat {
	groups := make(map[string]*groupedBrowserStat)
	var order []string

	for _, stat := range stats {
		title := CollapseURLTitle(stat.AppName)
		duration := stat.TotalDurationSecs

		category := NormalizeSiteName(title, groupByDomain)

		g, ok := groups[category]
		if !ok {
			g = &groupedBrowserStat{Category: category, TotalSecs: 0}
			groups[category] = g
			order = append(order, category)
		}
		g.TotalSecs += duration

		if category != title {
			g.SubEntries = append(g.SubEntries, subEntry{
				Title:    cleanTitleForDisplay(title),
				Duration: duration,
			})
		}
	}

	result := make([]groupedBrowserStat, 0, len(order))
	for _, cat := range order {
		result = append(result, *groups[cat])
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalSecs != result[j].TotalSecs {
			return result[i].TotalSecs > result[j].TotalSecs
		}
		return result[i].Category < result[j].Category
	})

	return result
}

var separatorRegex = regexp.MustCompile(`\s*[-–—|·]\s*`)

func cleanTitleForDisplay(title string) string {
	parts := separatorRegex.Split(title, -1)
	if len(parts) <= 1 {
		return title
	}

	cleaned := strings.TrimSpace(parts[0])
	if cleaned == "" && len(parts) > 1 {
		cleaned = strings.TrimSpace(parts[1])
	}

	if cleaned == "" {
		return title
	}

	return cleaned
}

// NormalizeSiteName maps one stored browser title to the label shown in stats.
//
// The catalogue always wins, so a Bilibili page title becomes "Bilibili" even
// when the experimental setting is off. With groupByDomain enabled, unmatched
// titles are also rolled up by a domain embedded in the title; the tracker only
// sees window titles, not URLs, so this is a best-effort heuristic.
func NormalizeSiteName(raw string, groupByDomain bool) string {
	title := CollapseURLTitle(strings.TrimSpace(raw))
	if title == "" {
		return ""
	}
	if name := SiteNameFor(title); name != "" {
		return name
	}
	if groupByDomain {
		if domain := domainLabelFor(title); domain != "" {
			return domain
		}
	}
	return title
}

var domainTokenRegex = regexp.MustCompile(`(?i)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,24}`)

// fileLikeExtensions are common suffixes that look like TLDs in page titles
// ("index.html", "report.pdf"). Treating them as domains would create noisy
// one-off groups, so they are skipped.
var fileLikeExtensions = map[string]bool{
	"html": true, "htm": true, "php": true, "asp": true, "aspx": true,
	"jsp": true, "pdf": true, "png": true, "jpg": true, "jpeg": true,
	"gif": true, "webp": true, "svg": true, "js": true, "css": true,
	"json": true, "txt": true, "mp4": true, "mp3": true, "zip": true,
	"md": true, "doc": true, "docx": true, "csv": true, "xml": true,
	"exe": true, "dll": true, "log": true, "yaml": true, "yml": true,
}

func domainLabelFor(text string) string {
	matches := domainTokenRegex.FindAllString(text, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		host := strings.ToLower(strings.Trim(matches[i], ".,;:!?()[]{}<>\"'`"))
		if !isDomainShaped(host) {
			continue
		}
		labels := strings.Split(host, ".")
		tld := labels[len(labels)-1]
		if fileLikeExtensions[tld] {
			continue
		}
		return strings.Split(registrableDomain(host), ".")[0]
	}
	return ""
}

// registrableDomain keeps the last two labels, plus one common second-level
// label, so "www.bilibili.com" and "m.bilibili.com" both become "bilibili.com".
func registrableDomain(host string) string {
	labels := strings.Split(host, ".")
	if len(labels) <= 2 {
		return host
	}
	second := labels[len(labels)-2]
	switch second {
	case "co", "com", "net", "org", "gov", "edu", "ac":
		if len(labels) >= 3 {
			return strings.Join(labels[len(labels)-3:], ".")
		}
	}
	return strings.Join(labels[len(labels)-2:], ".")
}
