//go:build !linux

package main

import rl "github.com/gen2brain/raylib-go/raylib"

// Non-Linux platforms keep raylib's own fullscreen path: the guess-based
// transit does not harm them (the Wayland slot leak does not exist there).
func requestFullscreenOnMonitor(index int) {
	rl.ToggleFullscreen()
	rl.SetWindowMonitor(index)
}

func releaseFullscreen() {
	if rl.IsWindowFullscreen() {
		rl.ToggleFullscreen()
	}
}
