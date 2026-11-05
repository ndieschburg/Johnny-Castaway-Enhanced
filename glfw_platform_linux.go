//go:build linux && cgo && !sdl && !sdl3 && !rgfw && !drm

package main

/*
 // glfwGetPlatform is provided by the GLFW 3.4 bundled into raylib-go.
 // Query it only after raylib has initialized the window/GLFW.
 extern int glfwGetPlatform(void);
 // Wayland-specific window state, read back after the compositor's first
 // configure: logical window size (wl.width/wl.height) and the fractional
 // content scale (wp_fractional_scale_v1 preferred_scale).
 extern void* glfwGetCurrentContext(void);
 extern void glfwGetWindowSize(void* window, int* width, int* height);
 extern void glfwGetWindowContentScale(void* window, float* xscale, float* yscale);
*/
import "C"

const glfwPlatformWayland = 0x00060003

func glfwPlatform() int {
	return int(C.glfwGetPlatform())
}

// glfwWindowSize returns the window size in logical (screen) coordinates as
// last set by the compositor's configure - on Wayland this is NOT the
// physical video mode when the output scale is fractional.
func glfwWindowSize() (int, int) {
	var w, h C.int
	C.glfwGetWindowSize(C.glfwGetCurrentContext(), &w, &h)
	return int(w), int(h)
}

// glfwWindowContentScale returns the window's effective content scale,
// including wp_fractional_scale_v1 (e.g. 1.5) when wl_output.scale is 1.
func glfwWindowContentScale() (float32, float32) {
	var x, y C.float
	C.glfwGetWindowContentScale(C.glfwGetCurrentContext(), &x, &y)
	return float32(x), float32(y)
}


