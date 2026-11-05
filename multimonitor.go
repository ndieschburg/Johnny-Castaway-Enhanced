package main

import (
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// getMonitorCount returns the number of connected monitors.
// On Windows it queries GetSystemMetrics(SM_CMONITORS) directly, avoiding
// needing to initialize and destroy a dummy Raylib window beforehand.
func getMonitorCount() int {
	if ret := getMonitorCountWin32(); ret > 0 {
		return ret
	}
	if ret := getMonitorCountNative(); ret > 0 {
		return ret
	}
	// Fallback for platforms/build backends without a native pre-window
	// enumeration path. If a Raylib window already exists, use it directly.
	// Otherwise retain the old throwaway-window behavior as a last resort.
	if !rl.IsWindowReady() {
		rl.InitWindow(64, 64, "")
		defer rl.CloseWindow()
	}
	return rl.GetMonitorCount()
}

// TMonitorRect describes one connected monitor's region within the single
// application window, in window-local coordinates (i.e. already offset so
// that (0,0) is the window's own top-left corner, not the OS desktop's).
type TMonitorRect struct {
	X, Y, W, H float32
}

// monitorRects holds one entry per connected monitor, computed once by
// setupMonitors(). The renderer draws a separate, correctly letterboxed
// copy of the scene into each entry's rectangle.
var monitorRects []TMonitorRect

// isWaylandSession checks the window system actually selected by GLFW after
// rl.InitWindow. Session environment variables are not reliable here: XWayland
// clients can inherit WAYLAND_DISPLAY and XDG_SESSION_TYPE=wayland.
func isWaylandSession() bool {
	return isWaylandPlatform(glfwPlatform())
}

func isWaylandPlatform(platform int) bool {
	return platform == glfwPlatformWayland
}

func validMonitorIndex(index, count int) bool {
	return index >= 0 && index < count
}

func selectedMonitorIndex(requested int, hasRequested bool, current int, count int) int {
	if count < 1 {
		return 0
	}
	if hasRequested && validMonitorIndex(requested, count) {
		return requested
	}
	if validMonitorIndex(current, count) {
		return current
	}
	return 0
}

// setupMonitors sizes and positions the application window to span every
// connected monitor (the combined virtual desktop bounding box, which
// correctly handles monitors of different sizes and irregular/offset
// arrangements, not just a simple side-by-side layout), and records each
// monitor's own rectangle in window-local coordinates.
//
// On a single-monitor system this reduces to exactly the previous
// behavior (one window sized to that one monitor, with a single
// full-window rectangle), so this is always safe to call instead of the
// old single-monitor sizing code, not just an opt-in extra mode.
func setupMonitors() {
	// Wayland compositors ignore programmatic window positioning entirely
	// and usually refuse the monitor-sized manual resize done below, which
	// leaves the screensaver stuck in its initial 640x480 window at a
	// compositor-chosen spot while the renderer letterboxes for the full
	// monitor size (showing up as a tiny zoomed scene in a screen corner).
	// A Wayland client can only reliably request fullscreen, so switch to a
	// fullscreen window on the target monitor and leave monitorRects empty:
	// the renderer then letterboxes against the actual window size on every
	// frame, which also absorbs the asynchronous compositor resize.
	if isWaylandSession() {
		monitorCount := rl.GetMonitorCount()
		monitor := selectedMonitorIndex(runOnMonitorIndex, hasMonitorIndex, rl.GetCurrentMonitor(), monitorCount)

		pumpFrame := func() {
			rl.BeginDrawing()
			rl.ClearBackground(rl.Black)
			rl.EndDrawing()
		}

		// waitCompositorSize pumps frames until the compositor has sized the
		// window to its final (full-output) size in LOGICAL pixels and the
		// fractional content scale (wp_fractional_scale preferred_scale,
		// which arrives asynchronously after the first configure) has settled.
		// Read both through GLFW: rl.GetMonitorWidth would report the
		// PHYSICAL video mode whenever the output scale is fractional
		// (wl_output.scale stays 1 there).
		waitCompositorSize := func(timeout time.Duration) (int, int, float32, float32) {
			deadline := time.Now().Add(timeout)
			lastW, lastH := -1, -1
			lastSx, lastSy := float32(-1), float32(-1)
			stableSince := time.Now()
			for time.Now().Before(deadline) {
				pumpFrame()
				w, h := glfwWindowSize()
				sx, sy := glfwWindowContentScale()
				if sx <= 0 {
					sx = 1
				}
				if sy <= 0 {
					sy = 1
				}
				if w == lastW && h == lastH && sx == lastSx && sy == lastSy {
					if w > 0 && h > 0 && (w != 640 || h != 480) && time.Since(stableSince) >= 300*time.Millisecond {
						return w, h, sx, sy
					}
				} else {
					lastW, lastH, lastSx, lastSy = w, h, sx, sy
					stableSince = time.Now()
				}
				time.Sleep(5 * time.Millisecond)
			}
			if lastW > 0 && lastH > 0 {
				return lastW, lastH, lastSx, lastSy
			}
			return 0, 0, 1, 1
		}

		pumpFrames := func(n int) {
			for i := 0; i < n; i++ {
				pumpFrame()
				time.Sleep(time.Millisecond)
			}
		}

		if !layerShellMode() {
			// xdg toplevel: the fullscreen request only works once the surface
			// is mapped, so pump a few frames first, then request fullscreen -
			// the only window placement a Wayland client can rely on. Select
			// the requested output while still windowed so the fullscreen
			// request lands on it.
			pumpFrames(100)
			if hasMonitorIndex && validMonitorIndex(runOnMonitorIndex, monitorCount) && rl.GetCurrentMonitor() != monitor {
				rl.SetWindowMonitor(monitor)
				pumpFrames(50)
			}
			rl.ToggleFullscreen()
		}
		// layer-shell: the first configure (full output, logical size) already
		// arrived during window creation. A compositor without
		// wlr-layer-shell support (e.g. GNOME/Mutter) makes the patched GLFW
		// fall back to a regular xdg toplevel, which then needs the fullscreen
		// request; detect that by the missing configure.

		targetW, targetH, sx, sy := waitCompositorSize(3 * time.Second)
		if layerShellMode() && (targetW <= 0 || (targetW == 640 && targetH == 480)) {
			pumpFrames(100)
			if hasMonitorIndex && validMonitorIndex(runOnMonitorIndex, monitorCount) && rl.GetCurrentMonitor() != monitor {
				rl.SetWindowMonitor(monitor)
			}
			rl.ToggleFullscreen()
			targetW, targetH, sx, sy = waitCompositorSize(5 * time.Second)
		}
		if targetW <= 0 || targetH <= 0 {
			// Last resort: derive the logical size from the monitor mode.
			targetW = int(float32(rl.GetMonitorWidth(monitor)) / sx)
			targetH = int(float32(rl.GetMonitorHeight(monitor)) / sy)
		}

		// SetWindowSize is echoed back by the patched GLFW, so raylib's window
		// (screen) size follows the logical size; the chained window-size
		// callback (installViewportScaleFix) then sets the GL viewport to the
		// framebuffer size (logical x content scale).
		rl.SetWindowSize(targetW, targetH)
		pumpFrames(30)
		monitorRects = nil
		return
	}
	if hasMonitorIndex {
		count := rl.GetMonitorCount()
		if !validMonitorIndex(runOnMonitorIndex, count) {
			runOnMonitorIndex = selectedMonitorIndex(runOnMonitorIndex, true, rl.GetCurrentMonitor(), count)
		}
		pos := rl.GetMonitorPosition(runOnMonitorIndex)
		w := float32(rl.GetMonitorWidth(runOnMonitorIndex))
		h := float32(rl.GetMonitorHeight(runOnMonitorIndex))
		if w <= 0 || h <= 0 {
			w, h = 1920, 1080
		}
		rl.SetWindowSize(int(w), int(h))
		rl.SetWindowPosition(int(pos.X), int(pos.Y))
		monitorRects = []TMonitorRect{{X: 0, Y: 0, W: w, H: h}}
		return
	}

	count := rl.GetMonitorCount()
	if count < 1 {
		count = 1
	}

	type rawMonitor struct {
		x, y, w, h float32
	}
	raw := make([]rawMonitor, 0, count)

	haveBounds := false
	var minX, minY, maxX, maxY float32

	for i := 0; i < count; i++ {
		pos := rl.GetMonitorPosition(i)
		w := float32(rl.GetMonitorWidth(i))
		h := float32(rl.GetMonitorHeight(i))
		if w <= 0 || h <= 0 {
			// Fallback for a monitor that fails to report geometry.
			w, h = 1920, 1080
		}

		raw = append(raw, rawMonitor{pos.X, pos.Y, w, h})

		left, top := pos.X, pos.Y
		right, bottom := pos.X+w, pos.Y+h
		if !haveBounds {
			minX, minY, maxX, maxY = left, top, right, bottom
			haveBounds = true
		} else {
			if left < minX {
				minX = left
			}
			if top < minY {
				minY = top
			}
			if right > maxX {
				maxX = right
			}
			if bottom > maxY {
				maxY = bottom
			}
		}
	}

	totalW := maxX - minX
	totalH := maxY - minY

	rl.SetWindowSize(int(totalW), int(totalH))
	rl.SetWindowPosition(int(minX), int(minY))

	monitorRects = monitorRects[:0]
	for _, m := range raw {
		monitorRects = append(monitorRects, TMonitorRect{
			X: m.x - minX,
			Y: m.y - minY,
			W: m.w,
			H: m.h,
		})
	}
}