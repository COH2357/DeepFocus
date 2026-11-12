//go:build windows

package gui

// The WASAPI peak meter lives in package system and is Windows-only.
const mediaDetectionAvailable = true
