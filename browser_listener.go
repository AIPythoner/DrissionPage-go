package drissionpage

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type browserHooks struct {
	mu        sync.Mutex
	next      uint64
	callbacks map[uint64]func(*ChromiumTab)
	order     []string
}

func newBrowserHooks() *browserHooks {
	return &browserHooks{callbacks: map[uint64]func(*ChromiumTab){}}
}
func (h *browserHooks) register(f func(*ChromiumTab)) func() {
	h.mu.Lock()
	h.next++
	id := h.next
	h.callbacks[id] = f
	h.mu.Unlock()
	return func() { h.mu.Lock(); delete(h.callbacks, id); h.mu.Unlock() }
}
func (h *browserHooks) notify(t *ChromiumTab) {
	h.mu.Lock()
	h.order = append(h.order, t.ID())
	callbacks := make([]func(*ChromiumTab), 0, len(h.callbacks))
	for _, f := range h.callbacks {
		callbacks = append(callbacks, f)
	}
	h.mu.Unlock()
	for _, f := range callbacks {
		f(t)
	}
}

type BrowserListener struct {
	packets    *eventQueue[*DataPacket]
	streams    *eventQueue[*StreamMessage]
	errs       *eventQueue[error]
	cancel     context.CancelFunc
	unregister func()
	mu         sync.Mutex
	children   map[string]*Listener
	paused     atomic.Bool
	once       sync.Once
	filter     ListenFilter
}

// Listen aggregates tabs. NewTab installs capture before navigating. Tabs opened
// by external clients are subscribed when their target-created event arrives.
func (b *Chromium) Listen(ctx context.Context, filter ListenFilter) (*BrowserListener, error) {
	life, cancel := context.WithCancel(ctx)
	l := &BrowserListener{packets: newEventQueue[*DataPacket](), streams: newEventQueue[*StreamMessage](), errs: newEventQueue[error](), cancel: cancel, children: map[string]*Listener{}, filter: cloneListenFilter(filter)}
	l.packets.setLimit(filter.BufferSize)
	l.streams.setLimit(filter.BufferSize)
	l.errs.setLimit(filter.BufferSize)
	add := func(t *ChromiumTab) {
		if life.Err() != nil {
			return
		}
		if b.browser.BrowserContextID != "" && t.browser.browser.BrowserContextID != b.browser.BrowserContextID {
			return
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		if _, ok := l.children[t.ID()]; ok {
			return
		}
		child, e := t.Listen(life, l.filter)
		if e != nil {
			l.errs.push(e)
			return
		}
		if l.paused.Load() {
			child.Pause(false)
		}
		l.children[t.ID()] = child
		go func() {
			for {
				p, e := child.Next(life)
				if e != nil {
					if child.Err() != nil {
						l.errs.push(child.Err())
					}
					return
				}
				p.TabID = t.ID()
				l.packets.push(p)
			}
		}()
		go func() {
			for {
				m, e := child.NextStream(life)
				if e != nil {
					return
				}
				m.TabID = t.ID()
				l.streams.push(m)
			}
		}()
	}
	l.unregister = b.hooks.register(add)
	tabs, e := b.Tabs(life)
	if e != nil {
		l.Stop()
		return nil, e
	}
	for _, t := range tabs {
		add(t)
	}
	wait := b.browser.Context(life).EachEvent(func(e *proto.TargetTargetCreated) {
		if e.TargetInfo.Type != proto.TargetTargetInfoTypePage {
			return
		}
		if b.browser.BrowserContextID != "" && e.TargetInfo.BrowserContextID != b.browser.BrowserContextID {
			return
		}
		t, err := b.GetTab(life, string(e.TargetInfo.TargetID))
		if err != nil {
			l.errs.push(err)
			return
		}
		add(t)
	}, func(e *proto.TargetTargetDestroyed) {
		l.mu.Lock()
		child := l.children[string(e.TargetID)]
		delete(l.children, string(e.TargetID))
		l.mu.Unlock()
		if child != nil {
			child.Stop()
		}
	})
	if e := (proto.TargetSetDiscoverTargets{Discover: true}).Call(b.browser.Context(life)); e != nil {
		l.Stop()
		return nil, e
	}
	go func() { wait(); l.Stop() }()
	return l, nil
}
func (l *BrowserListener) Next(ctx context.Context) (*DataPacket, error) { return l.packets.pop(ctx) }
func (l *BrowserListener) NextStream(ctx context.Context) (*StreamMessage, error) {
	return l.streams.pop(ctx)
}
func (l *BrowserListener) NextError(ctx context.Context) (error, error) { return l.errs.pop(ctx) }
func (l *BrowserListener) Pause(clear bool) {
	l.paused.Store(true)
	l.mu.Lock()
	for _, child := range l.children {
		child.Pause(clear)
	}
	l.mu.Unlock()
	if clear {
		l.Clear()
	}
}
func (l *BrowserListener) Resume() {
	l.paused.Store(false)
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, child := range l.children {
		child.Resume()
	}
}
func (l *BrowserListener) Clear() { l.packets.clear(); l.streams.clear(); l.errs.clear() }
func (l *BrowserListener) Stop() {
	l.once.Do(func() {
		l.cancel()
		if l.unregister != nil {
			l.unregister()
		}
		l.mu.Lock()
		for _, child := range l.children {
			child.Stop()
		}
		l.mu.Unlock()
		l.packets.close()
		l.streams.close()
		l.errs.close()
	})
}
func (l *BrowserListener) WaitSilent(ctx context.Context, quiet time.Duration) error {
	since := time.Now()
	for {
		active := int64(0)
		l.mu.Lock()
		for _, child := range l.children {
			active += child.active.Load()
		}
		l.mu.Unlock()
		if active > 0 {
			since = time.Now()
		}
		if time.Since(since) >= quiet {
			return nil
		}
		if e := waitDuration(ctx, 20*time.Millisecond); e != nil {
			return e
		}
	}
}

// SetFilter applies future-request matching to current and subsequently added tabs.
func (l *BrowserListener) SetFilter(filter ListenFilter) error {
	if _, err := compileNetworkFilter(filter); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, child := range l.children {
		if err := child.SetFilter(filter); err != nil {
			return err
		}
	}
	l.filter = cloneListenFilter(filter)
	return nil
}
