# Third-party notices

DeepFocus is a fork of [Focusd](https://github.com/0xarchit/Focusd), created by
**0xarchit** (<https://github.com/0xarchit>). The original engine, data model and
interface concept are his work. The original project is MIT licensed; its
copyright notice is preserved in `LICENSE`.

DeepFocus adds a native desktop interface, Chinese/English localisation,
media-aware idle handling, weekly archives and other changes. It is not
endorsed by or affiliated with the original author.

DeepFocus does not bundle an auto-updater. The app only opens the project's
release page in your browser. Downloading and installing a new build is always
a manual action you take yourself.

## Go dependencies

| Project | License | Used for |
| --- | --- | --- |
| [Go](https://go.dev/) | BSD-3-Clause | language runtime and standard library |
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | MIT | terminal UI runtime |
| [Lip Gloss](https://github.com/charmbracelet/lipgloss) | MIT | terminal styling |
| [go-runewidth](https://github.com/mattn/go-runewidth) | MIT | terminal column widths |
| [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) | BSD-3-Clause | Windows API bindings |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | BSD-3-Clause | pure-Go SQLite driver |
| [go-webview2](https://github.com/jchv/go-webview2) | MIT | Windows WebView2 host |
| [go-winloader](https://github.com/jchv/go-winloader) | MIT | embedded WebView2Loader |

Transitive dependencies include
[go-osc52](https://github.com/aymanbagabas/go-osc52),
[colorprofile](https://github.com/charmbracelet/colorprofile),
[charmbracelet/x](https://github.com/charmbracelet/x),
[go-humanize](https://github.com/dustin/go-humanize),
[go-colorful](https://github.com/lucasb-eyer/go-colorful),
[go-isatty](https://github.com/mattn/go-isatty),
[termenv](https://github.com/muesli/termenv),
[uniseg](https://github.com/rivo/uniseg),
[libc](https://gitlab.com/cznic/libc),
[mathutil](https://gitlab.com/cznic/mathutil),
[memory](https://gitlab.com/cznic/memory), and
[bigfft](https://github.com/remyoudompheng/bigfft). Their licenses are MIT or
BSD-3-Clause; the full texts are available from each project and in the Go
module cache used to build the binaries.

## Fonts

**Bricolage Grotesque** by Mathieu Triay is licensed under the SIL Open Font
License 1.1 and is embedded as a subset at
`gui/web/assets/fonts/bricolage-grotesque.woff2`. The license text is available
at <https://openfontlicense.org/>.

## Runtime components

DeepFocus uses the **Microsoft Edge WebView2 Runtime** that ships with Windows.
WebView2 is a Microsoft component and is not distributed as part of this
repository. If the runtime is missing, DeepFocus falls back to your default
browser.

## Trademarks

Windows, Microsoft Edge, and WebView2 are trademarks of Microsoft Corporation.
All other project names are used only for identification and belong to their
respective owners.
