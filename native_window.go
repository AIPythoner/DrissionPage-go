package drissionpage

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"net"
	"net/url"
)

var ErrUnsupportedPlatform = errors.New("operation is not supported on this platform")

type NativeWindow struct {
	Handle  uintptr
	Visible bool
	Title   string
}

func (b *Chromium) ProcessID(ctx context.Context) (int, error) {
	result, err := (proto.SystemInfoGetProcessInfo{}).Call(b.browser.Context(ctx))
	if err != nil {
		return 0, err
	}
	for _, process := range result.ProcessInfo {
		if process.Type == "browser" {
			return process.ID, nil
		}
	}
	return 0, fmt.Errorf("browser process not reported")
}
func (t *ChromiumTab) NativeWindows(ctx context.Context) ([]NativeWindow, error) {
	if t.browser.launcher == nil {
		u, err := url.Parse(t.browser.endpoint)
		if err != nil {
			return nil, err
		}
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("native window operations require a local browser")
		}
	}
	pid, err := t.browser.ProcessID(ctx)
	if err != nil {
		return nil, err
	}
	title, err := t.Title(ctx)
	if err != nil {
		return nil, err
	}
	return nativeWindows(pid, title)
}
func (t *ChromiumTab) HideWindow(ctx context.Context) error { return t.showNativeWindow(ctx, false) }
func (t *ChromiumTab) ShowWindow(ctx context.Context) error { return t.showNativeWindow(ctx, true) }
func (t *ChromiumTab) showNativeWindow(ctx context.Context, show bool) error {
	windows, err := t.NativeWindows(ctx)
	if err != nil {
		return err
	}
	if len(windows) == 0 {
		return fmt.Errorf("no native browser window matched the tab title")
	}
	for _, window := range windows {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err = nativeShowWindow(window.Handle, show); err != nil {
			return err
		}
	}
	return nil
}
