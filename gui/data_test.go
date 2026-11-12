package gui

import (
	"focusd/storage"
	"testing"
)

func TestFilterAppStatsDropsSelfAndBuiltins(t *testing.T) {
	stats := []storage.AppDailyStat{
		{AppName: "Visual Studio Code", ExeName: "Code.exe", TotalDurationSecs: 120},
		{AppName: "DeepFocus", ExeName: "deepfocus_gui.exe", TotalDurationSecs: 300},
		{AppName: "LockApp", ExeName: "LockApp.exe", TotalDurationSecs: 30},
		{AppName: "", ExeName: "deepfocus_daemon.exe", TotalDurationSecs: 10},
		{AppName: "Focusd", ExeName: "", TotalDurationSecs: 5},
	}

	got := filterAppStats(stats)
	if len(got) != 1 {
		t.Fatalf("filterAppStats kept %d rows, want 1: %+v", len(got), got)
	}
	if got[0].ExeName != "Code.exe" {
		t.Fatalf("kept %q, want Code.exe", got[0].ExeName)
	}
}
