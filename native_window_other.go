//go:build !windows

package drissionpage

func nativeWindows(pid int, title string) ([]NativeWindow, error) { return nil, ErrUnsupportedPlatform }
func nativeShowWindow(handle uintptr, show bool) error            { return ErrUnsupportedPlatform }

func nativeContentOrigin(handle uintptr, width, height, dpr float64) (Point, error) {
	return Point{}, ErrUnsupportedPlatform
}
