package drissionpage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type Screencast struct {
	tab       *ChromiumTab
	cancel    context.CancelFunc
	done      chan struct{}
	mu        sync.Mutex
	err       error
	frames    int
	directory string
}

// StartScreencast records JPEG frames on Chrome's repaint events. The destination
// must be new so recordings never overwrite an earlier capture.
func (t *ChromiumTab) StartScreencast(ctx context.Context, directory string) (*Screencast, error) {
	dir, e := filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	if e = os.Mkdir(dir, 0700); e != nil {
		return nil, e
	}
	life, cancel := context.WithCancel(ctx)
	p := t.page.Context(life)
	s := &Screencast{tab: t, cancel: cancel, done: make(chan struct{}), directory: dir}
	wait := p.EachEvent(func(e *proto.PageScreencastFrame) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.err == nil {
			s.frames++
			path := filepath.Join(dir, fmt.Sprintf("frame-%06d.jpg", s.frames))
			s.err = os.WriteFile(path, e.Data, 0600)
		}
		if err := (proto.PageScreencastFrameAck{SessionID: e.SessionID}).Call(p); s.err == nil {
			s.err = err
		}
	})
	quality, every := 80, 1
	if e := (proto.PageStartScreencast{Format: proto.PageStartScreencastFormatJpeg, Quality: &quality, EveryNthFrame: &every}).Call(p); e != nil {
		cancel()
		return nil, e
	}
	go func() { defer close(s.done); wait() }()
	return s, nil
}
func (s *Screencast) Stop(ctx context.Context) error {
	e := (proto.PageStopScreencast{}).Call(s.tab.page.Context(ctx))
	s.cancel()
	select {
	case <-s.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e != nil {
		return e
	}
	return s.err
}
func (s *Screencast) Frames() int { s.mu.Lock(); defer s.mu.Unlock(); return s.frames }

// Video converts a stopped recording using an explicitly supplied ffmpeg binary.
// Repaint frames are played at fps; original wall-clock timing is not preserved.
func (s *Screencast) Video(ctx context.Context, ffmpeg, path string, fps int) error {
	select {
	case <-s.done:
	default:
		return fmt.Errorf("stop screencast before exporting video")
	}
	if fps <= 0 {
		return fmt.Errorf("fps must be positive")
	}
	if s.Frames() == 0 {
		return fmt.Errorf("recording has no frames")
	}
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-n", "-framerate", fmt.Sprint(fps), "-i", filepath.Join(s.directory, "frame-%06d.jpg"), "-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2", "-pix_fmt", "yuv420p", path)
	output, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("ffmpeg: %w: %s", e, output)
	}
	return nil
}

// AutoHandleAlerts must be installed before the action that opens a dialog.
func (t *ChromiumTab) AutoHandleAlerts(ctx context.Context, accept bool, prompt string) (func(), error) {
	life, cancel := context.WithCancel(ctx)
	p := t.page.Context(life)
	if e := (proto.PageEnable{}).Call(p); e != nil {
		cancel()
		return nil, e
	}
	wait := p.EachEvent(func(e *proto.PageJavascriptDialogOpening) {
		callCtx, c := context.WithTimeout(life, 5*time.Second)
		defer c()
		_ = (proto.PageHandleJavaScriptDialog{Accept: accept, PromptText: prompt}).Call(t.page.Context(callCtx))
	})
	go wait()
	return cancel, nil
}
