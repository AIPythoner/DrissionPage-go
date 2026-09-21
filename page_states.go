package drissionpage

import (
	"context"
	"github.com/go-rod/rod/lib/proto"
	"sync"
)

type PageStates struct {
	Alive, Loading, HasAlert bool
	ReadyState               string
	AlertMessage, AlertType  string
}

// PageStateObserver tracks dialogs without running JavaScript in a blocked
// renderer. Create it before opening a dialog; Stop releases its subscription.
type PageStateObserver struct {
	mu     sync.RWMutex
	state  PageStates
	cancel context.CancelFunc
}

func (t *ChromiumTab) WatchState(ctx context.Context) (*PageStateObserver, error) {
	life, cancel := context.WithCancel(ctx)
	state := &PageStateObserver{cancel: cancel, state: PageStates{Alive: true}}
	page := t.page.Context(life)
	wait := page.EachEvent(func(event *proto.PageFrameStartedLoading) {
		if event.FrameID == page.FrameID {
			state.mu.Lock()
			state.state.Loading = true
			state.state.ReadyState = "loading"
			state.mu.Unlock()
		}
	}, func(event *proto.PageFrameStoppedLoading) {
		if event.FrameID == page.FrameID {
			state.mu.Lock()
			state.state.Loading = false
			state.state.ReadyState = "complete"
			state.mu.Unlock()
		}
	}, func(event *proto.PageJavascriptDialogOpening) {
		state.mu.Lock()
		state.state.HasAlert = true
		state.state.AlertMessage = event.Message
		state.state.AlertType = string(event.Type)
		state.mu.Unlock()
	}, func(*proto.PageJavascriptDialogClosed) {
		state.mu.Lock()
		state.state.HasAlert = false
		state.mu.Unlock()
	})
	ready, err := t.ReadyState(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	state.state.ReadyState = ready
	state.state.Loading = ready == "loading"
	go func() { wait(); state.mu.Lock(); state.state.Alive = false; state.mu.Unlock() }()
	return state, nil
}
func (s *PageStateObserver) Snapshot() PageStates { s.mu.RLock(); defer s.mu.RUnlock(); return s.state }
func (s *PageStateObserver) Stop()                { s.cancel() }

func (t *ChromiumTab) ShowTrail(ctx context.Context, on bool) error {
	_, err := t.RunJS(ctx, `on=>{const key='__drissionpage_trail';const old=globalThis[key];if(old){removeEventListener('mousemove',old.move,true);old.element.remove();delete globalThis[key]};if(!on)return;const element=document.createElement('div');element.style.cssText='position:fixed;pointer-events:none;z-index:2147483647;width:12px;height:12px;border:2px solid red;border-radius:50%;transform:translate(-50%,-50%)';document.documentElement.append(element);const move=e=>{element.style.left=e.clientX+'px';element.style.top=e.clientY+'px'};addEventListener('mousemove',move,true);globalThis[key]={element,move}}`, on)
	return err
}
