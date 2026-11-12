# DeepFocus

[![Version](https://img.shields.io/github/v/release/COH2357/DeepFocus?style=for-the-badge&logo=github&logoColor=white&labelColor=000000&color=000000)](https://github.com/COH2357/DeepFocus/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/COH2357/DeepFocus/release.yml?style=for-the-badge&logo=githubactions&logoColor=white&labelColor=000000&color=000000)](https://github.com/COH2357/DeepFocus/actions)
[![Downloads](https://img.shields.io/github/downloads/COH2357/DeepFocus/total?style=for-the-badge&logo=rolldown&logoColor=white&labelColor=000000&color=000000)](https://github.com/COH2357/DeepFocus/releases)  
[![License](https://img.shields.io/badge/License-MIT-000000.svg?style=for-the-badge&logo=apache&logoColor=white&labelColor=000000&color=000000)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-000000.svg?style=for-the-badge&logo=go&logoColor=white&labelColor=000000&color=000000)](https://go.dev)
![Platform](https://img.shields.io/badge/Platform-Windows_x64-000000.svg?style=for-the-badge&logo=nsis&logoColor=white&labelColor=000000&color=000000)

<p align="center">
  <img src="website/assets/deepfocus-icon.svg" width="128" alt="DeepFocus icon">
</p>

<h3 align="center">Privacy-First Digital Wellbeing for Windows</h3>

<p align="center">
  <strong>Track your screen time. Own your data. No cloud required.</strong><br/>
  <a href="https://github.com/COH2357/DeepFocus">github.com/COH2357/DeepFocus</a>
</p>

---

## Overview

DeepFocus is a native Windows **screen time tracker** and **digital wellbeing** tool written in Go. A lightweight background daemon (`deepfocus_daemon.exe`) polls the foreground window via Win32 syscalls and records per-app and per-browser-tab usage into a local SQLite database. `deepfocus_gui.exe` is a native desktop app (WebView2, not Electron) with dashboards, focus timer, limits and settings; the same features remain available from the `deepfocus.exe` CLI/TUI, which talks to the daemon over a local TCP IPC channel.

It functions as an **app usage monitor**, **browser time tracker**, **Pomodoro focus timer**, and **productivity dashboard** — three small Go binaries and an NSIS installer, with no Electron or Node runtime. If you're looking for a **local, no-account alternative to RescueTime, StayFree, or Toggl Track**, DeepFocus is built for that.

Nothing leaves your machine. DeepFocus makes no network requests at runtime: updating is a manual visit to the releases page, never a background check.

| App | Distribution | Sync model | Account required |
|-----|-------------|-----------|-----------------|
| StayFree (Windows) | Microsoft Store (UWP) + Electron desktop | Cross-device sync | Yes |
| Toggl Track | Native C++/.NET desktop client ([source](https://github.com/toggl/toggldesktop)) | Cloud sync (Toggl account) | Yes |
| RescueTime | Native Windows client + web dashboard | Cloud sync (RescueTime account) | Yes |
| **DeepFocus** | **Native Windows app + CLI** | **None — local SQLite only** | **No** |

Competitor distribution details come from each vendor's own product pages and repos. RAM figures were dropped from this table because they cannot be reproducibly sourced — measure on your own machine if you need them.

---

## Why DeepFocus?

| Problem | How DeepFocus solves it |
|---------|---------------------|
| Screen time trackers that send your data to the cloud | All data stays in a local SQLite database — no accounts, no telemetry |
| Trackers that ship a bundled browser or heavy runtime | Two small native Go binaries (~12 MB CLI, ~8 MB daemon); ~9 MB installer |
| No visibility into browser tab usage | Tracks time per tab via window title, grouped under 74 hard-coded categories |
| No way to limit distracting apps | Set daily per-app time limits with notifications |
| Hard to stay focused | Built-in Pomodoro timer with break reminders |

---

## Quick Start

**PowerShell** (recommended):
```powershell
iwr "https://github.com/COH2357/DeepFocus/releases/latest/download/deepfocus_setup.exe" -OutFile deepfocus_setup.exe; ./deepfocus_setup.exe
```

**Command Prompt**:
```cmd
curl -L -o deepfocus_setup.exe "https://github.com/COH2357/DeepFocus/releases/latest/download/deepfocus_setup.exe" && deepfocus_setup.exe
```

The NSIS installer (`installer.nsi`) installs three binaries to `%APPDATA%\focusd` (kept from the original Focusd so existing history survives), adds the install directory to your user `PATH`, registers `deepfocus_daemon.exe` under the `HKCU\...\Run` autostart key, registers an entry in **Add/Remove Programs**, starts the tracker, and opens the desktop app. Desktop and Start Menu shortcuts are created for `deepfocus_gui.exe`.

Running setup over an existing install upgrades it in place: setup stops the running binaries, replaces only the program files, and keeps `focusd.db`, `config.json`, the archive folder and your settings. Silent installs (`/S`) do not open the GUI.

Open a new terminal and type `deepfocus` (or use the shortcut) to launch the desktop GUI; `deepfocus tui` opens the terminal interface.

---

## Features

### Desktop GUI
`deepfocus_gui.exe` is the primary interface: a native window with five sections — **Dashboard**, **Stats**, **Focus**, **Limits**, **Settings**. It is built with WebView2 and a hand-written HTML/CSS/JS bundle embedded in the binary, so there is no Electron runtime or Node dependency. The whole interface is fully bilingual: **简体中文** and English, switchable at any time from the sidebar (or with `L`); it follows the system language on first launch. A warm-paper light theme and a midnight dark theme are both included (`T` toggles), with a custom title bar that matches the theme.

Mouse interaction is first-class: hover states, click-through rows for app/site details, drag-free range pickers, toasts, and modal dialogs. Keyboard shortcuts: `Ctrl+1..5` sections, `R` refresh, `T` theme, `L` language, `?` shortcuts, `Esc` close dialog.

The GUI is a client of the same SQLite store and IPC channel as the CLI. It can start/stop the daemon, pause tracking, edit limits, export data, and wipe history. If WebView2 is unavailable, DeepFocus falls back to opening the same UI in your default browser and closes the local server shortly after the tab is closed.

The first window is sized against the monitor work area (not the full screen) and stays comfortably inside it, so the native minimize/maximize/close buttons are always reachable. With **Settings → Tracking → Close to tray** enabled, closing the window hides it and keeps the process running; click the notification-area icon to reopen the window or choose **Quit DeepFocus**. Launching the shortcut again reveals the existing window instead of starting a second copy.

### Interactive TUI
`deepfocus tui` opens a Bubble Tea / Lipgloss terminal UI with the same five tabs: **Dashboard**, **Stats**, **Focus**, **Limits**, **Settings**. Mouse and alt-screen are enabled; minimum terminal size is 80×24. Press `?` for the full key-binding overlay.

### Background daemon
`deepfocus_daemon.exe` is built with the `-H=windowsgui` linker flag so it runs with no console window. It polls the active window every **5 seconds** (configurable 1–60s), batches sessions to SQLite every **5 minutes**, persists the active session every 5 minutes (so a crash recovers cleanly), enforces retention every hour, and checks Pomodoro / break / app-limit conditions every 5 seconds.

### Per-app and per-browser tracking
The daemon reads the foreground window's title and process executable using `GetForegroundWindow`, `GetWindowTextW`, and `GetModuleBaseNameW`. App sessions are written to the `sessions` table; daily totals are upserted into `apps_daily`. When the foreground process is a recognised browser, the cleaned window **title** (not URL) is upserted into `browsing_daily`.

### Smart browser-tab grouping
Tab titles are matched against a built-in catalog in `coreapi/sitecatalog.go` and grouped under a parent (YouTube, GitHub, Bilibili, LeetCode, ChatGPT, Claude, Notion, Figma, AWS, Coursera, Discord, Gmail, Reddit, and so on).

The **Settings → Experimental → Group sites by domain** switch is a best-effort mode that merges everything from the same site into one entry, so all Bilibili videos count as “Bilibili”. It extracts a domain from page titles when the title contains one; pages whose titles do not expose a domain still fall back to their own title.

### Default browser detection
`chrome`, `firefox`, `msedge`, `brave`, `opera`, `vivaldi`, `waterfox`, `arc`, `iexplore`, `safari`, `whale`, `yandex`, `thorium`, `librewolf`, `chromium`, `floorp`, `zen`. Add others with `deepfocus browser add <name>.exe`.

### Pomodoro timer
File-backed timer at `%APPDATA%\focusd\pomodoro.json` so state survives across processes. Default 25 minutes; configurable. Completion is announced through a Windows Toast (`Windows.UI.Notifications.ToastNotification`, invoked from `core/notifications.go` via a short PowerShell command). A 10-second cooldown suppresses duplicate toasts.

### Break reminders
When enabled, the daemon notifies after a configurable number of minutes of continuous use (default 60). The reminder can be snoozed for the global snooze duration (default 60 min).

### Daily app time limits
Set per-executable limits (`deepfocus limit chrome.exe 60`). The daemon notifies once when the day's usage exceeds the limit; the snooze button suppresses repeats for the snooze window. Limits reset at local midnight.

### Whitelist and built-in exclusions
Whitelisted executables are skipped entirely by the tracker (never recorded). DeepFocus also excludes its own binaries (`deepfocus.exe`, `deepfocus_gui.exe`, `deepfocus_daemon.exe`, plus the legacy `focusd*.exe` names) and Windows shell/lock surfaces such as `LockApp.exe`, `LogonUI.exe`, `ShellExperienceHost.exe`, `StartMenuExperienceHost.exe`, `SearchHost.exe`, `TextInputHost.exe` and `ApplicationFrameHost.exe`. These rows are filtered out of the dashboard, stats and session lists as well, so they disappear immediately rather than lingering until retention.

### Long-term history
Retention is on by default: rows older than the configured window (7 days by default) are deleted hourly. Enable **Settings → Long-term history** to keep everything. When it is on, completed ISO weeks are copied into one SQLite file per week under `%APPDATA%\focusd\archive\` (`deepfocus-week-YYYY-Www.db`). The working database is left intact, so every screen keeps working; archives are idempotent (no duplicate weeks) and empty weeks are skipped. The Settings card shows the archive count, the latest week, and a button to open the folder.

### Data export
Export tracking data to CSV or JSON from Settings in the desktop app or the TUI. Files are saved to `%USERPROFILE%\Downloads`. You can also open the SQLite database directly at `%APPDATA%\focusd\focusd.db`.

### Updates are manual by design
DeepFocus never downloads or runs an installer on its own. `deepfocus update` (and the Settings button) simply opens the project's GitHub **releases page** in your browser. You choose whether to download a build, and the installer is per-user, so no elevation prompt appears.

> **Verification.** Each release publishes `checksums.txt`. Compare the SHA-256 of a downloaded file with the published value before running it. Releases are currently unsigned, so Windows SmartScreen may show an "unknown publisher" prompt.

### Open-source notices
DeepFocus is a fork of [Focusd](https://github.com/0xarchit/Focusd) by **0xarchit** and is built on open-source Go libraries plus an embedded OFL font. The full list and licenses are in [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md); the desktop app also shows them under **Settings → Open-source notices**. Future enforcement ideas that are deliberately not implemented yet are recorded in [`ROADMAP.md`](ROADMAP.md).

---

## Commands

```
deepfocus                       Open the desktop GUI (falls back to the TUI)
deepfocus gui                   Open the desktop GUI explicitly
deepfocus tui                   Open the terminal UI
deepfocus start                 Start the background daemon
deepfocus stop                  Stop the daemon (graceful via IPC, falls back to kill)
deepfocus status        (s)     Daemon + tracking status with today's top 5 apps
deepfocus stats         (st)    Full daily breakdown: top apps + grouped browsing
deepfocus pause         (p)     Pause tracking (daemon keeps running, drops samples)
deepfocus resume        (r)     Resume tracking
deepfocus focus [min]           Start a Pomodoro timer (default 25 min)
deepfocus stop-timer            Stop the running Pomodoro
deepfocus limit <app> <min>     Set a daily time limit (omit args to list)
deepfocus update                Open the GitHub releases page (no auto-install)
deepfocus help          (h)     Show built-in help
deepfocus version       (-v)    Print version
```

### Subcommands

```
deepfocus retention status      Show current retention window
deepfocus retention set <days>  Set retention (1–365, default 7)
deepfocus retention reset       Reset retention to default

deepfocus autostart status      Show autostart state
deepfocus autostart enable      Add daemon to HKCU\...\Run
deepfocus autostart disable     Remove daemon from HKCU\...\Run

deepfocus path status           Show whether install dir is on PATH
deepfocus path enable           Append install dir to user PATH
deepfocus path disable          Remove install dir from user PATH

deepfocus browser list          Show user-defined browsers
deepfocus browser add <exe>     Add a custom browser executable
deepfocus browser remove <exe>  Remove a custom browser
```

`deepfocus --daemon` is the entry the daemon binary uses to enter its tracker loop; you should not invoke it directly.

---

## Architecture

```
┌─────────────────────┐      TCP 127.0.0.1:48321      ┌──────────────────────────┐
│ deepfocus.exe       │ ─── "stop" / "flush" / "ok" ─► │ deepfocus_daemon.exe     │
│ CLI + TUI           │ ◄───────────────────────────── │ Windows GUI subsystem    │
├─────────────────────┤                                └────────────┬─────────────┘
│ deepfocus_gui.exe   │                                             │
│ desktop GUI         │                                             │
└──────────┬──────────┘                                             │
           │ reads / writes                                         │
           ▼                                                        ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ %APPDATA%\focusd\   (kept from Focusd for upgrade compatibility)             │
│ focusd.db (SQLite WAL) · config.json · browsers.json · pomodoro.json         │
└──────────────────────────────────────────────────────────────────────────────┘
```

| Package | Purpose |
|---------|---------|
| `cmd/deepfocus` | Entry point for the CLI/TUI binary. Calls `system.AttachParentConsole`, `system.CleanupOldBinary`, then `cli.Run`. |
| `cmd/deepfocus_gui` | Entry point for the desktop GUI (`-H=windowsgui`). Calls `gui.Run`. |
| `cmd/deepfocus_daemon` | Entry point for the silent daemon. Calls `cli.RunDaemon`. |
| `cmd/release_notes` | Build-time helper that generates `release_notes.md` from `git log` via the GitHub Models API. |
| `cli` | Command parsing, interactive menu, releases-page handoff, status / stats / export commands. |
| `core` | Tracker loop, IPC client helpers, app classification, title cleaner, notifications (Windows Toast). |
| `coreapi` | Shared cross-package data: IPC address/constants, browser-tab group definitions, Pomodoro state on disk, aggregation. |
| `storage` | SQLite schema and queries, retention enforcement, browser registry, durable config. |
| `system` | All Windows-specific code: Win32 syscalls (`user32`, `kernel32`, `psapi`), process enumeration, registry (autostart + PATH), user-config file, daemon process spawn. Stubs exist for non-Windows builds so `go build` still passes on other OSes (the daemon refuses to run). |
| `tui` | Bubble Tea / Lipgloss model, splash, tabs, dashboard, stats, focus, limits, settings panels. |
| `gui` | Native WebView2 host, localhost API server with a per-session token, and the embedded bilingual HTML/CSS/JS interface. |
| `ui` | ANSI colours, table renderer, duration formatter for the classic CLI commands. |

### IPC
The daemon listens on **`127.0.0.1:48321`** and accepts two text commands:

| Command | Effect |
|---------|--------|
| `stop` | Acknowledge with `ok`, flush state, exit. |
| `flush` | Close the current session, write batched sessions, persist active-session checkpoint, reply `ok`. |

The CLI calls `flush` before reading stats so reports never miss the in-flight session.

### Tickers inside the tracker loop
| Ticker | Interval | What it does |
|--------|----------|--------------|
| `pollTicker` | 5s (configurable) | Sample the foreground window |
| `batchTicker` | 5 min | Write batched sessions to SQLite |
| `persistTicker` | 5 min | Snapshot the active session for crash recovery |
| `retentionTicker` | 1 hour | Archive completed weeks when long-term history is on; otherwise delete rows older than retention |
| `focusTicker` | 5s | Pomodoro completion, break reminders, app-limit notifications |

---

## Data & Configuration

Everything lives under `%APPDATA%\focusd\`.

| File | Contents |
|------|----------|
| `focusd.db` | SQLite database (WAL mode, incremental autovacuum, NORMAL synchronous) |
| `config.json` | User config: whitelist, app limits, Pomodoro minutes, break reminder, snooze duration, custom browsers |
| `pomodoro.json` | Current Pomodoro state (active flag, start time, duration, notified flag) |

### Database schema

```sql
config           (key, value, updated_at)
sessions         (id, app_name, exe_name, window_title, start_time, end_time, duration_secs, date)
apps_daily       (id, date, app_name, exe_name, total_duration_secs, open_count)   -- UNIQUE(date, exe_name)
browsing_daily   (id, date, domain_or_title, total_duration_secs, open_count)      -- UNIQUE(date, domain_or_title)
active_session   (id=1, app_name, exe_name, window_title, start_time, last_seen, date)
```

Indexes: `idx_sessions_date`, `idx_sessions_start`, `idx_apps_daily_date`.

### Defaults & ranges (from `storage/config.go` and `system/userconfig.go`)

| Setting | Default | Range |
|---------|---------|-------|
| Retention (days) | 7 | 1 – 365 |
| Long-term history | off | on / off |
| Tracking poll interval (s) | 5 | 1 – 60 |
| Idle threshold (min) | 5 | 0 – 240 (0 = never idle) |
| Track while idle | off | on / off |
| Track while playing media | on | on / off |
| Close to tray | on | on / off |
| Pomodoro (min) | 25 | > 0 |
| Break reminder (min) | 60 | > 0 |
| Snooze (min) | 60 | > 0 |

### Registry keys written

- `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\DeepFocus` — autostart entry (full quoted path to `deepfocus_daemon.exe`); the old `focusd` value is removed on upgrade.
- `HKCU\Environment\Path` — append-only modification of user PATH.
- `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\DeepFocus` — written by the NSIS installer for **Add/Remove Programs**.

---

## Privacy

DeepFocus is built for privacy by design:

- **No telemetry, no update pings.** `deepfocus update` only asks Windows to open the releases page in your browser; the app itself never contacts GitHub. Grep for `http.` if you don't believe it.
- **Local-only storage.** All data lives in `%APPDATA%\focusd\focusd.db`. The SQLite file is standard; query it with any SQL tool.
- **Titles, not URLs.** Browser tracking records the window title only. There is no browser extension, no HTTP interception, no DNS sniffing.
- **Whitelist & pause** give you full control. Whitelisted executables are never recorded; DeepFocus's own binaries and Windows shell surfaces are excluded automatically.
- **No privilege escalation.** All three binaries and the installer run entirely under your user account; nothing in DeepFocus requests elevation. The install directory, the autostart entry, and the user PATH all live under `%APPDATA%` or HKCU.
- **Open source.** Full source code available. Audit it yourself.

---

## Building from Source

**Requirements:** Go **1.26+**, Windows (the daemon only runs on Windows). NSIS is required to build the installer.

```powershell
git clone https://github.com/COH2357/DeepFocus.git
cd DeepFocus

# CLI / TUI binary
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w -X 'focusd/system.Version=dev-build'" -trimpath -o deepfocus.exe ./cmd/deepfocus

# Desktop GUI (WebView2; embeds the web assets)
go build -ldflags="-s -w -H=windowsgui -X 'focusd/system.Version=dev-build'" -trimpath -o deepfocus_gui.exe ./cmd/deepfocus_gui

# Silent background daemon
go build -ldflags="-s -w -H=windowsgui -X 'focusd/system.Version=dev-build'" -trimpath -o deepfocus_daemon.exe ./cmd/deepfocus_daemon

# NSIS installer (optional)
makensis /DVERSION=dev-build installer.nsi
```

Replace `dev-build` with the release version. The `-X 'focusd/system.Version=...'` ldflag is read at runtime by `deepfocus version`.

### CI

| Workflow | Trigger | Output |
|----------|---------|--------|
| `.github/workflows/release.yml` | Push of a `vX.Y.Z` tag | Builds all three binaries, NSIS installer, SHA-256 `checksums.txt`, AI-generated release notes via GitHub Models (`gpt-4o-mini`), publishes a GitHub Release. |
| `.github/workflows/test_build.yml` | Push to `test` branch | Builds an unsigned dev installer, uploads as a 5-day artifact. |
| `.github/workflows/deploy-site.yml` | Push to `main`/`master` touching `website/**` | Deploys the static site under `website/` to GitHub Pages (`github.com/COH2357/DeepFocus`). |

---

## Uninstalling

Use **Settings → Apps → Installed apps → DeepFocus → Uninstall**, which invokes the NSIS uninstaller. It stops the daemon and GUI, removes the binaries and both shortcuts, deletes the autostart registry entry, removes the install directory from your user `PATH`, and preserves your database and config.

To delete your usage history as well, remove `%APPDATA%\focusd\` manually after uninstalling.

---

## Contributing

Contributions welcome. See [CONTRIBUTING.md](.github/CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](.github/CODE_OF_CONDUCT.md). Security issues: see [SECURITY.md](.github/SECURITY.md).

## License

MIT License — see [LICENSE](LICENSE).

---

<p align="center">
  <sub>DeepFocus is maintained by <a href="https://github.com/COH2357">@COH2357</a>;<br>originally built as Focusd by <a href="https://github.com/0xarchit">@0xarchit</a>.</sub>
</p>
