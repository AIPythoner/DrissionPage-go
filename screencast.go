package drissionpage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type Screencast struct {
	tab        *ChromiumTab
	cancel     context.CancelFunc
	done       chan struct{}
	mu         sync.Mutex
	err        error
	frames     int
	directory  string
	stopOnce   sync.Once
	stopErr    error
	periodic   bool
	timestamps []time.Time
	stopped    time.Time
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
			s.timestamps = append(s.timestamps, time.Now())
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
	s.stopOnce.Do(func() {
		if !s.periodic {
			s.stopErr = (proto.PageStopScreencast{}).Call(s.tab.page.Context(ctx))
		}
		s.cancel()
	})
	select {
	case <-s.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped.IsZero() {
		s.stopped = time.Now()
	}
	if s.stopErr != nil {
		return s.stopErr
	}
	return s.err
}

// StartPeriodicScreencast captures frames at a fixed interval, including pages
// that do not repaint. This implements the original imgs/video recording modes.
func (t *ChromiumTab) StartPeriodicScreencast(ctx context.Context, directory string, interval time.Duration) (*Screencast, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("capture interval must be positive")
	}
	dir, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if err = os.Mkdir(dir, 0700); err != nil {
		return nil, err
	}
	life, cancel := context.WithCancel(ctx)
	s := &Screencast{tab: t, cancel: cancel, done: make(chan struct{}), directory: dir, periodic: true}
	capture := func() error {
		result, err := (proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatJpeg}).Call(t.page.Context(life))
		if err != nil {
			return err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("frame-%06d.jpg", s.frames+1)), result.Data, 0600); err != nil {
			return err
		}
		s.frames++
		s.timestamps = append(s.timestamps, time.Now())
		return nil
	}
	if err = capture(); err != nil {
		cancel()
		return nil, err
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-life.Done():
				return
			case <-ticker.C:
				if err := capture(); err != nil {
					if life.Err() == nil {
						s.mu.Lock()
						s.err = err
						s.mu.Unlock()
					}
					return
				}
			}
		}
	}()
	return s, nil
}

// TimelineVideo preserves intervals between recorded frames, including the last
// still frame until Stop. It works with both periodic and repaint recordings.
func (s *Screencast) TimelineVideo(ctx context.Context, ffmpeg, path string) error {
	select {
	case <-s.done:
	default:
		return fmt.Errorf("stop screencast before exporting video")
	}
	s.mu.Lock()
	times := append([]time.Time(nil), s.timestamps...)
	end := s.stopped
	captureErr := s.err
	s.mu.Unlock()
	if captureErr != nil {
		return captureErr
	}
	if len(times) == 0 {
		return fmt.Errorf("recording has no frames")
	}
	if end.IsZero() {
		return fmt.Errorf("call Stop before timeline export")
	}
	var text strings.Builder
	text.WriteString("ffconcat version 1.0\n")
	for i, start := range times {
		next := end
		if i+1 < len(times) {
			next = times[i+1]
		}
		duration := next.Sub(start).Seconds()
		if duration < 0.001 {
			duration = 0.001
		}
		fmt.Fprintf(&text, "file 'frame-%06d.jpg'\nduration %.9f\n", i+1, duration)
	}
	fmt.Fprintf(&text, "file 'frame-%06d.jpg'\n", len(times))
	manifest, err := os.CreateTemp(s.directory, "timeline-*.ffconcat")
	if err != nil {
		return err
	}
	name := manifest.Name()
	defer os.Remove(name)
	if _, err = manifest.WriteString(text.String()); err != nil {
		manifest.Close()
		return err
	}
	if err = manifest.Close(); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-n", "-f", "concat", "-safe", "1", "-i", name, "-vsync", "vfr", "-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2", "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, output)
	}
	return nil
}

// StartRecording selects the original non-interactive screencast modes. Frames
// remain available in directory; use Video or TimelineVideo for video exports.
func (t *ChromiumTab) StartRecording(ctx context.Context, directory, mode string) (*Screencast, error) {
	switch mode {
	case "video", "imgs":
		return t.StartPeriodicScreencast(ctx, directory, 200*time.Millisecond)
	case "frugal_video", "frugal_imgs":
		return t.StartScreencast(ctx, directory)
	default:
		return nil, fmt.Errorf("unknown frame recording mode %q; use StartDisplayRecording for js_video", mode)
	}
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
