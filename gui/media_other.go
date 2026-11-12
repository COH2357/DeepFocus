//go:build !windows

package gui

// Audio playback detection is only wired up for Windows/WASAPI.
const mediaDetectionAvailable = false
