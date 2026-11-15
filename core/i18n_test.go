package core

import (
	"os"
	"path/filepath"
	"testing"

	"focusd/system"
)

func TestNotificationLanguageFollowsSettings(t *testing.T) {
	dir := t.TempDir()
	old, had := os.LookupEnv("APPDATA")
	os.Setenv("APPDATA", filepath.Join(dir, "appdata"))
	defer func() {
		if had {
			os.Setenv("APPDATA", old)
		} else {
			os.Unsetenv("APPDATA")
		}
	}()
	if err := os.MkdirAll(filepath.Join(os.Getenv("APPDATA"), "focusd"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := system.SetLanguage("zh"); err != nil {
		t.Fatal(err)
	}
	if got := tr("English", "中文"); got != "中文" {
		t.Fatalf("tr with zh = %q", got)
	}
	if err := system.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	if got := tr("English", "中文"); got != "English" {
		t.Fatalf("tr with en = %q", got)
	}
}
