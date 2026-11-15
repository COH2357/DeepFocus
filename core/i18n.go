package core

import "focusd/system"

// tr returns the runtime string in the language selected in Settings. The
// desktop UI and the daemon share config.json, so notifications follow the
// in-app language instead of always being English.
func tr(en, zh string) string {
	if system.UseChinese() {
		return zh
	}
	return en
}
