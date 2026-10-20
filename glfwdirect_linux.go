//go:build linux

package main

/*
#cgo LDFLAGS: -lm -ldl -lpthread
typedef struct GLFWwindow GLFWwindow;
typedef struct GLFWmonitor GLFWmonitor;
extern GLFWmonitor** glfwGetMonitors(int* count);
extern void glfwSetWindowMonitor(GLFWwindow* window, GLFWmonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);
*/
import "C"

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"unsafe"
)

// GLFW_DONT_CARE
const glfwDontCare = -1

// requestFullscreenOnMonitor issues a single xdg_toplevel
// set_fullscreen(output) request for the given monitor, from any window
// state (windowed or fullscreen elsewhere).
//
// raylib's ToggleFullscreen() + SetWindowMonitor() pair cannot do this:
// ToggleFullscreen first fullscreens on GetCurrentMonitor(), which on
// Wayland is a position-based guess (Wayland clients cannot read their
// window position), transiently occupying - and leaking - the fullscreen
// slot of a shared workspace, which makes the sibling instance's own
// fullscreen request get silently dropped.
func requestFullscreenOnMonitor(index int) {
	handle := rl.GetWindowHandle()
	if handle == nil {
		return
	}

	var count C.int
	monitors := C.glfwGetMonitors(&count)
	if monitors == nil || index < 0 || index >= int(count) {
		return
	}
	monitorList := (*[1 << 20]*C.GLFWmonitor)(unsafe.Pointer(monitors))

	w := rl.GetMonitorWidth(index)
	h := rl.GetMonitorHeight(index)
	if w <= 0 || h <= 0 {
		return
	}

	C.glfwSetWindowMonitor((*C.GLFWwindow)(unsafe.Pointer(handle)),
		monitorList[index], 0, 0, C.int(w), C.int(h), glfwDontCare)
}

// releaseFullscreen drops GLFW's fullscreen-monitor bookkeeping
// (xdg_toplevel unset_fullscreen when fullscreen). Needed before
// re-issuing requestFullscreenOnMonitor for the same index: GLFW's
// Wayland backend early-returns when window->monitor already equals the
// requested monitor, and would never send a fresh request.
func releaseFullscreen() {
	handle := rl.GetWindowHandle()
	if handle == nil {
		return
	}

	C.glfwSetWindowMonitor((*C.GLFWwindow)(unsafe.Pointer(handle)),
		nil, 0, 0, C.int(rl.GetScreenWidth()), C.int(rl.GetScreenHeight()), glfwDontCare)
}
