package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"strings"
)

func (e *ChromiumElement) MiddleClick(ctx context.Context) error {
	return e.mouseClick(ctx, proto.InputMouseButtonMiddle, 1)
}
func (e *ChromiumElement) MultiClick(ctx context.Context, count int) error {
	if count < 1 {
		return fmt.Errorf("click count must be positive")
	}
	return e.mouseClick(ctx, proto.InputMouseButtonLeft, count)
}
func (e *ChromiumElement) Value(ctx context.Context) (json.RawMessage, error) {
	return e.Property(ctx, "value")
}
func (e *ChromiumElement) SetValue(ctx context.Context, value any) error {
	return e.SetProperty(ctx, "value", value)
}
func (e *ChromiumElement) SetInnerHTML(ctx context.Context, markup string) error {
	return e.SetProperty(ctx, "innerHTML", markup)
}
func (e *ChromiumElement) InputJS(ctx context.Context, text string, clear bool) error {
	_, err := e.RunJS(ctx, `function(value,clear){this.value=(clear?'':this.value)+value;this.dispatchEvent(new Event('change',{bubbles:true}))}`, text, clear)
	return err
}
func (e *ChromiumElement) ScrollToCenter(ctx context.Context) error {
	_, err := e.RunJS(ctx, `function(){this.scrollIntoView({block:'center',inline:'center',behavior:'instant'})}`)
	return err
}
func (t *ChromiumTab) ScrollToTop(ctx context.Context) error { return t.ScrollTo(ctx, 0, 0) }
func (t *ChromiumTab) ScrollToHalf(ctx context.Context) error {
	_, err := t.RunJS(ctx, `()=>window.scrollTo(0,(document.scrollingElement.scrollHeight-innerHeight)/2)`)
	return err
}
func (t *ChromiumTab) ScrollToRightmost(ctx context.Context) error {
	_, err := t.RunJS(ctx, `()=>window.scrollTo(document.scrollingElement.scrollWidth,scrollY)`)
	return err
}
func (t *ChromiumTab) ScrollToLeftmost(ctx context.Context) error {
	_, err := t.RunJS(ctx, `()=>window.scrollTo(0,scrollY)`)
	return err
}
func (t *ChromiumTab) SetSmoothScroll(ctx context.Context, on bool) error {
	_, err := t.RunJS(ctx, `on=>document.documentElement.style.scrollBehavior=on?'smooth':'auto'`, on)
	return err
}
func (t *ChromiumTab) RunJSLoaded(ctx context.Context, script string, args ...any) (json.RawMessage, error) {
	if err := t.WaitLoad(ctx); err != nil {
		return nil, err
	}
	return t.RunJS(ctx, script, args...)
}
func (t *ChromiumTab) RunCDPLoaded(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if err := t.WaitLoad(ctx); err != nil {
		return nil, err
	}
	return t.RunCDP(ctx, method, params)
}

type JSOutcome struct {
	Value json.RawMessage
	Err   error
}

// RunAsyncJS starts one goroutine and returns a buffered, single-result channel.
// The supplied context controls evaluation and should be canceled when abandoned.
func (t *ChromiumTab) RunAsyncJS(ctx context.Context, script string, args ...any) <-chan JSOutcome {
	done := make(chan JSOutcome, 1)
	go func() { defer close(done); value, err := t.RunJS(ctx, script, args...); done <- JSOutcome{value, err} }()
	return done
}
func (e *ChromiumElement) RunAsyncJS(ctx context.Context, script string, args ...any) <-chan JSOutcome {
	done := make(chan JSOutcome, 1)
	go func() { defer close(done); value, err := e.RunJS(ctx, script, args...); done <- JSOutcome{value, err} }()
	return done
}
func (t *ChromiumTab) SEles(ctx context.Context, locator any) ([]*SessionElement, error) {
	snapshot, err := t.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Eles(locator)
}
func (e *ChromiumElement) SEle(ctx context.Context, locator any, index ...int) (*SessionElement, error) {
	snapshot, err := e.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Ele(locator, index...)
}
func (e *ChromiumElement) SEles(ctx context.Context, locator any) ([]*SessionElement, error) {
	snapshot, err := e.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Eles(locator)
}
func (b *Chromium) CommandLine(ctx context.Context) ([]string, error) {
	result, err := (proto.BrowserGetBrowserCommandLine{}).Call(b.browser.Context(ctx))
	if err != nil {
		return nil, err
	}
	return result.Arguments, nil
}

type BrowserStates struct{ Alive, Headless, Existed, Incognito, Guest bool }

func (b *Chromium) States(ctx context.Context) (*BrowserStates, error) {
	states := &BrowserStates{Existed: b.launcher == nil, Incognito: b.browser.BrowserContextID != ""}
	if b.closed {
		return states, nil
	}
	_, err := (proto.BrowserGetVersion{}).Call(b.browser.Context(ctx))
	if err != nil {
		return states, err
	}
	states.Alive = true
	args, err := b.CommandLine(ctx)
	if err != nil {
		return states, err
	}
	for _, arg := range args {
		states.Headless = states.Headless || strings.HasPrefix(arg, "--headless")
		states.Incognito = states.Incognito || arg == "--incognito"
		states.Guest = states.Guest || arg == "--guest"
	}
	return states, nil
}
func (m *OptionsManager) Remove(section, key string) { delete(m.Sections[section], key) }
func (m *OptionsManager) Section(section string) map[string]string {
	copy := map[string]string{}
	for key, value := range m.Sections[section] {
		copy[key] = value
	}
	return copy
}
