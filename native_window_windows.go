package drissionpage

import (
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var enumWindows = user32.NewProc("EnumWindows")
var windowPID = user32.NewProc("GetWindowThreadProcessId")
var windowText = user32.NewProc("GetWindowTextW")
var isWindowVisible = user32.NewProc("IsWindowVisible")
var showWindow = user32.NewProc("ShowWindow")

var nativeEnumeration struct {
	sync.Mutex
	pid     int
	title   string
	windows []NativeWindow
}
var enumerateCallback = syscall.NewCallback(func(handle, unused uintptr) uintptr {
	var process uint32
	windowPID.Call(handle, uintptr(unsafe.Pointer(&process)))
	if int(process) != nativeEnumeration.pid {
		return 1
	}
	buffer := make([]uint16, 32768)
	n, _, _ := windowText.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	name := syscall.UTF16ToString(buffer[:n])
	if name == "" || !strings.Contains(name, nativeEnumeration.title) {
		return 1
	}
	visible, _, _ := isWindowVisible.Call(handle)
	nativeEnumeration.windows = append(nativeEnumeration.windows, NativeWindow{handle, visible != 0, name})
	return 1
})

func nativeWindows(pid int, title string) ([]NativeWindow, error) {
	nativeEnumeration.Lock()
	defer nativeEnumeration.Unlock()
	nativeEnumeration.pid = pid
	nativeEnumeration.title = title
	nativeEnumeration.windows = nil
	result, _, err := enumWindows.Call(enumerateCallback, 0)
	windows := nativeEnumeration.windows
	nativeEnumeration.windows = nil
	if result == 0 {
		return nil, err
	}
	return windows, nil
}

func nativeShowWindow(handle uintptr, show bool) error {
	mode := uintptr(0)
	if show {
		mode = 5
	}
	showWindow.Call(handle, mode)
	return nil
}

// Chrome's content viewport occupies the lower client area. Coordinates from
// ClientToScreen are physical pixels for this process's DPI-awareness context.
func nativeContentOrigin(handle uintptr, width, height, dpr float64) (Point, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	awareness := user32.NewProc("SetThreadDpiAwarenessContext")
	if awareness.Find() == nil {
		previous, _, _ := awareness.Call(^uintptr(3))
		if previous != 0 {
			defer awareness.Call(previous)
		}
	}

	var rect struct{ Left, Top, Right, Bottom int32 }
	var point struct{ X, Y int32 }
	getClientRect := user32.NewProc("GetClientRect")
	clientToScreen := user32.NewProc("ClientToScreen")
	if ok, _, err := getClientRect.Call(handle, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return Point{}, err
	}
	if ok, _, err := clientToScreen.Call(handle, uintptr(unsafe.Pointer(&point))); ok == 0 {
		return Point{}, err
	}
	return Point{float64(point.X) + (float64(rect.Right-rect.Left)-width*dpr)/2, float64(point.Y+rect.Bottom-rect.Top) - height*dpr}, nil
}
