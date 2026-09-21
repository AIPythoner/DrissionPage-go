package drissionpage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type DownloadState string

const (
	DownloadRunning   DownloadState = "running"
	DownloadCompleted DownloadState = "completed"
	DownloadCanceled  DownloadState = "canceled"
	DownloadFailed    DownloadState = "failed"
)

type DownloadProgress struct {
	State           DownloadState
	Received, Total int64
	Path            string
	Err             error
}
type DownloadMission struct {
	mu       sync.RWMutex
	progress DownloadProgress
	done     chan struct{}
	cancel   context.CancelFunc
}

func (m *DownloadMission) Progress() DownloadProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.progress
}
func (m *DownloadMission) Cancel() { m.cancel() }
func (m *DownloadMission) Wait(ctx context.Context) (DownloadProgress, error) {
	select {
	case <-ctx.Done():
		return m.Progress(), ctx.Err()
	case <-m.done:
		p := m.Progress()
		return p, p.Err
	}
}
func (m *DownloadMission) finish(state DownloadState, path string, e error) {
	m.mu.Lock()
	m.progress.State = state
	m.progress.Path = path
	m.progress.Err = e
	m.mu.Unlock()
	close(m.done)
}

type progressWriter struct {
	writer  io.Writer
	mission *DownloadMission
}

func (w progressWriter) Write(p []byte) (int, error) {
	n, e := w.writer.Write(p)
	w.mission.mu.Lock()
	w.mission.progress.Received += int64(n)
	w.mission.mu.Unlock()
	return n, e
}

// Download streams into a temporary file next to path, then renames it only
// after a successful response. An existing destination is never overwritten.
func (p *SessionPage) Download(ctx context.Context, target, path string) (*DownloadMission, error) {
	p.mu.RLock()
	closed := p.closed
	headers := p.options.Headers.Clone()
	client := *p.client
	if p.requestTimeout != nil {
		client.Timeout = *p.requestTimeout
	}
	username, password := p.options.Username, p.options.Password
	p.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	absolute, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	if _, e = os.Stat(absolute); e == nil {
		return nil, fmt.Errorf("destination exists: %s", absolute)
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	f, e := os.CreateTemp(filepath.Dir(absolute), ".drission-download-*")
	if e != nil {
		return nil, e
	}
	life, cancel := context.WithCancel(ctx)
	m := &DownloadMission{progress: DownloadProgress{State: DownloadRunning, Path: absolute}, done: make(chan struct{}), cancel: cancel}
	go func() {
		defer cancel()
		defer os.Remove(f.Name())
		defer f.Close()
		fail := func(e error) {
			state := DownloadFailed
			if life.Err() != nil {
				state = DownloadCanceled
				e = life.Err()
			}
			m.finish(state, absolute, e)
		}
		req, e := http.NewRequestWithContext(life, http.MethodGet, target, nil)
		if e != nil {
			fail(e)
			return
		}
		req.Header = headers
		if username != "" {
			req.SetBasicAuth(username, password)
		}
		res, e := client.Do(req)
		if e != nil {
			fail(e)
			return
		}
		defer res.Body.Close()
		if res.StatusCode >= 400 {
			fail(&HTTPError{res.StatusCode, target})
			return
		}
		m.mu.Lock()
		m.progress.Total = res.ContentLength
		m.mu.Unlock()
		if _, e = io.Copy(progressWriter{f, m}, res.Body); e != nil {
			fail(e)
			return
		}
		if e = f.Sync(); e != nil {
			fail(e)
			return
		}
		if e = f.Close(); e != nil {
			fail(e)
			return
		}
		// Hard linking is atomic and refuses an existing destination on all supported
		// platforms. The temporary file resides on the same filesystem.
		if e = os.Link(f.Name(), absolute); e != nil {
			fail(e)
			return
		}
		m.finish(DownloadCompleted, absolute, nil)
	}()
	return m, nil
}

type BrowserDownload struct {
	GUID, URL, SuggestedFilename, Path string
	FrameID                            string
	Received, Total                    float64
	State                              string
}
type DownloadManager struct {
	queue      *eventQueue[BrowserDownload]
	cancel     context.CancelFunc
	browser    *Chromium
	directory  string
	mu         sync.RWMutex
	missions   map[string]BrowserDownload
	clickSlots map[string]chan struct{}
}

func (b *Chromium) Downloads(ctx context.Context, directory string) (*DownloadManager, error) {
	dir, e := filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	life, cancel := context.WithCancel(ctx)
	r := b.browser.Context(life)
	d := &DownloadManager{queue: newEventQueue[BrowserDownload](), cancel: cancel, browser: b, directory: dir, missions: make(map[string]BrowserDownload)}
	wait := r.EachEvent(func(e *proto.BrowserDownloadWillBegin) {
		d.mu.Lock()
		m := BrowserDownload{GUID: e.GUID, URL: e.URL, SuggestedFilename: e.SuggestedFilename, Path: filepath.Join(dir, e.GUID), State: "inProgress"}
		m.FrameID = string(e.FrameID)
		d.missions[e.GUID] = m
		d.mu.Unlock()
		d.queue.push(m)
	}, func(e *proto.BrowserDownloadProgress) {
		d.mu.Lock()
		m := d.missions[e.GUID]
		m.GUID = e.GUID
		m.Received = e.ReceivedBytes
		m.Total = e.TotalBytes
		m.State = string(e.State)
		d.missions[e.GUID] = m
		d.mu.Unlock()
		d.queue.push(m)
	})
	if e := (proto.BrowserSetDownloadBehavior{Behavior: proto.BrowserSetDownloadBehaviorBehaviorAllowAndName, BrowserContextID: r.BrowserContextID, DownloadPath: dir, EventsEnabled: true}).Call(r); e != nil {
		cancel()
		return nil, e
	}
	go func() { defer d.queue.close(); wait() }()
	return d, nil
}
func (d *DownloadManager) Next(ctx context.Context) (BrowserDownload, error) { return d.queue.pop(ctx) }
func (d *DownloadManager) Missions() []BrowserDownload {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]BrowserDownload, 0, len(d.missions))
	for _, m := range d.missions {
		out = append(out, m)
	}
	return out
}
func (d *DownloadManager) Cancel(ctx context.Context, guid string) error {
	return (proto.BrowserCancelDownload{GUID: guid, BrowserContextID: d.browser.browser.BrowserContextID}).Call(d.browser.browser.Context(ctx))
}
func (d *DownloadManager) Wait(ctx context.Context, guid string) (BrowserDownload, error) {
	for {
		d.mu.RLock()
		m, ok := d.missions[guid]
		d.mu.RUnlock()
		if ok && m.State != "inProgress" {
			if m.State == "canceled" {
				return m, context.Canceled
			}
			return m, nil
		}
		if e := waitDuration(ctx, 25*time.Millisecond); e != nil {
			return m, e
		}
	}
}
func (d *DownloadManager) Close() { d.cancel(); d.queue.close() }
