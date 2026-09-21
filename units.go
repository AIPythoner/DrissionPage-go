package drissionpage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

type ElementStates struct{ Selected, Checked, Displayed, Enabled, Alive, InViewport, WholeInViewport, Covered, Clickable, HasRect bool }

func (e *ChromiumElement) States(ctx context.Context) (*ElementStates, error) {
	data, err := e.RunJS(ctx, `function(){const target=this.nodeType===11?this.host:this;const r=target.getBoundingClientRect(),s=getComputedStyle(target),v=r.width>0&&r.height>0&&s.visibility!=='hidden'&&s.display!=='none',x=Math.min(innerWidth-1,Math.max(0,r.x+r.width/2)),y=Math.min(innerHeight-1,Math.max(0,r.y+r.height/2)),top=this.ownerDocument.elementFromPoint(x,y),covered=!!top&&top!==this&&!this.contains(top);return {Selected:!!this.selected,Checked:!!this.checked,Displayed:v,Enabled:!this.disabled,Alive:this.isConnected,InViewport:r.bottom>0&&r.right>0&&r.top<innerHeight&&r.left<innerWidth,WholeInViewport:r.top>=0&&r.left>=0&&r.bottom<=innerHeight&&r.right<=innerWidth,Covered:covered,Clickable:v&&!this.disabled&&!covered,HasRect:r.width>0&&r.height>0}}`)
	if err != nil {
		if errors.Is(err, cdp.ErrObjNotFound) || errors.Is(err, cdp.ErrCtxDestroyed) || errors.Is(err, cdp.ErrCtxNotFound) {
			return &ElementStates{}, nil
		}
		return nil, err
	}
	var s ElementStates
	err = json.Unmarshal(data, &s)
	return &s, err
}
func (e *ChromiumElement) WaitState(ctx context.Context, state string, want bool) error {
	ctx, c := context.WithTimeout(ctx, e.tab.settings().Timeout)
	defer c()
	for {
		s, err := e.States(ctx)
		if err != nil {
			return err
		}
		var got bool
		switch state {
		case "selected":
			got = s.Selected
		case "checked":
			got = s.Checked
		case "displayed":
			got = s.Displayed
		case "enabled":
			got = s.Enabled
		case "alive":
			got = s.Alive
		case "in_viewport":
			got = s.InViewport
		case "covered":
			got = s.Covered
		case "clickable":
			got = s.Clickable
		case "has_rect":
			got = s.HasRect
		default:
			return fmt.Errorf("unknown state %q", state)
		}
		if got == want {
			return nil
		}
		if err = waitDuration(ctx, 50*time.Millisecond); err != nil {
			return err
		}
	}
}
func (e *ChromiumElement) SetAttr(ctx context.Context, name, value string) error {
	_, err := e.RunJS(ctx, `function(k,v){this.setAttribute(k,v)}`, name, value)
	return err
}
func (e *ChromiumElement) RemoveAttr(ctx context.Context, name string) error {
	_, err := e.RunJS(ctx, `function(k){this.removeAttribute(k)}`, name)
	return err
}
func (e *ChromiumElement) SetProperty(ctx context.Context, name string, value any) error {
	_, err := e.RunJS(ctx, `function(k,v){this[k]=v}`, name, value)
	return err
}
func (e *ChromiumElement) SetStyle(ctx context.Context, name, value string) error {
	_, err := e.RunJS(ctx, `function(k,v){this.style.setProperty(k,v)}`, name, value)
	return err
}
func (e *ChromiumElement) Style(ctx context.Context, name string) (string, error) {
	data, err := e.RunJS(ctx, `function(k){return getComputedStyle(this).getPropertyValue(k)}`, name)
	if err != nil {
		return "", err
	}
	var s string
	err = json.Unmarshal(data, &s)
	return s, err
}
func (e *ChromiumElement) Tag(ctx context.Context) (string, error) {
	data, err := e.RunJS(ctx, `function(){return this.tagName.toLowerCase()}`)
	if err != nil {
		return "", err
	}
	var s string
	err = json.Unmarshal(data, &s)
	return s, err
}
func (e *ChromiumElement) Attrs(ctx context.Context) (map[string]string, error) {
	data, err := e.RunJS(ctx, `function(){return Object.fromEntries(Array.from(this.attributes,a=>[a.name,a.value]))}`)
	if err != nil {
		return nil, err
	}
	var attrs map[string]string
	err = json.Unmarshal(data, &attrs)
	return attrs, err
}
func (e *ChromiumElement) Scroll(ctx context.Context, x, y float64) error {
	_, err := e.RunJS(ctx, `function(x,y){this.scrollBy(x,y)}`, x, y)
	return err
}
func (e *ChromiumElement) ScrollTo(ctx context.Context, x, y float64) error {
	_, err := e.RunJS(ctx, `function(x,y){this.scrollTo(x,y)}`, x, y)
	return err
}
func (t *ChromiumTab) ScrollTo(ctx context.Context, x, y float64) error {
	_, err := t.RunJS(ctx, `(x,y)=>scrollTo(x,y)`, x, y)
	return err
}
func (t *ChromiumTab) ScrollToBottom(ctx context.Context) error {
	_, err := t.RunJS(ctx, `()=>scrollTo(scrollX,document.scrollingElement.scrollHeight)`)
	return err
}
func (t *ChromiumTab) Storage(ctx context.Context, session bool) (map[string]string, error) {
	data, err := t.RunJS(ctx, `(session)=>{const storage=session?sessionStorage:localStorage;const result={};for(let i=0;i<storage.length;i++){const key=storage.key(i);result[key]=storage.getItem(key)}return result}`, session)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	err = json.Unmarshal(data, &out)
	return out, err
}
func (t *ChromiumTab) SetStorage(ctx context.Context, session bool, key string, value *string) error {
	_, err := t.RunJS(ctx, `(s,k,v)=>{const storage=s?sessionStorage:localStorage;if(v===null)storage.removeItem(k);else storage.setItem(k,v)}`, session, key, value)
	return err
}
func (t *ChromiumTab) ClearStorage(ctx context.Context, session bool) error {
	_, err := t.RunJS(ctx, `(s)=>(s?sessionStorage:localStorage).clear()`, session)
	return err
}
func (t *ChromiumTab) SetUserAgent(ctx context.Context, ua, platform string) error {
	return (proto.NetworkSetUserAgentOverride{UserAgent: ua, Platform: platform}).Call(t.page.Context(ctx))
}
func (t *ChromiumTab) SetBlockedURLs(ctx context.Context, urls ...string) error {
	return t.page.Context(ctx).SetBlockedURLs(urls)
}
func (t *ChromiumTab) SetWindow(ctx context.Context, bounds *proto.BrowserBounds) error {
	return t.page.Context(ctx).SetWindow(bounds)
}
func (t *ChromiumTab) DeleteCookie(ctx context.Context, name, target, domain, path string) error {
	return (proto.NetworkDeleteCookies{Name: name, URL: target, Domain: domain, Path: path}).Call(t.page.Context(ctx))
}
func (t *ChromiumTab) ClearCookies(ctx context.Context) error {
	return (proto.NetworkClearBrowserCookies{}).Call(t.page.Context(ctx))
}
func (b *Chromium) SetPermission(ctx context.Context, name, setting, origin string) error {
	_, e := b.RunCDP(ctx, "Browser.setPermission", map[string]any{"permission": map[string]any{"name": name}, "setting": setting, "origin": origin, "browserContextId": b.browser.BrowserContextID})
	return e
}
func (b *Chromium) ResetPermissions(ctx context.Context) error {
	return (proto.BrowserResetPermissions{BrowserContextID: b.browser.BrowserContextID}).Call(b.browser.Context(ctx))
}
func (t *ChromiumTab) WaitURL(ctx context.Context, text string, exclude bool) error {
	return t.WaitJS(ctx, `(s,exclude)=>location.href.includes(s)!==exclude`, text, exclude)
}
func (t *ChromiumTab) WaitTitle(ctx context.Context, text string, exclude bool) error {
	return t.WaitJS(ctx, `(s,exclude)=>document.title.includes(s)!==exclude`, text, exclude)
}
func (t *ChromiumTab) WaitDeleted(ctx context.Context, locator any) error {
	ctx, c := context.WithTimeout(ctx, t.settings().Timeout)
	defer c()
	for {
		els, e := t.Eles(ctx, locator)
		if e != nil {
			return e
		}
		if len(els) == 0 {
			return nil
		}
		if e = waitDuration(ctx, 50*time.Millisecond); e != nil {
			return e
		}
	}
}
func (t *ChromiumTab) HandleAlert(ctx context.Context, accept bool, prompt string) error {
	return (proto.PageHandleJavaScriptDialog{Accept: accept, PromptText: prompt}).Call(t.page.Context(ctx))
}
func (t *ChromiumTab) InitJS(ctx context.Context, script string) (string, error) {
	r, e := (proto.PageAddScriptToEvaluateOnNewDocument{Source: script}).Call(t.page.Context(ctx))
	if e != nil {
		return "", e
	}
	return string(r.Identifier), nil
}
func (t *ChromiumTab) RemoveInitJS(ctx context.Context, id string) error {
	return (proto.PageRemoveScriptToEvaluateOnNewDocument{Identifier: proto.PageScriptIdentifier(id)}).Call(t.page.Context(ctx))
}

type Actions struct {
	tab  *ChromiumTab
	ctx  context.Context
	err  error
	x, y float64
}

func (t *ChromiumTab) Actions(ctx context.Context) *Actions { return &Actions{tab: t, ctx: ctx} }
func (a *Actions) Do() error                                { return a.err }
func (a *Actions) MoveTo(x, y float64) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Mouse.MoveTo(proto.Point{X: x, Y: y})
		a.x = x
		a.y = y
	}
	return a
}
func (a *Actions) Move(x, y float64) *Actions { return a.MoveTo(a.x+x, a.y+y) }
func (a *Actions) On(e *ChromiumElement) *Actions {
	if a.err != nil {
		return a
	}
	r, err := e.RootRect(a.ctx)
	if err != nil {
		a.err = err
		return a
	}
	a.tab = e.tab.topTab()
	return a.MoveTo(r.X+r.Width/2, r.Y+r.Height/2)
}
func (a *Actions) Click(button string, count int) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Mouse.Click(proto.InputMouseButton(strings.ToLower(button)), count)
	}
	return a
}
func (a *Actions) Hold(button string) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Mouse.Down(proto.InputMouseButton(button), 1)
	}
	return a
}
func (a *Actions) Release(button string) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Mouse.Up(proto.InputMouseButton(button), 1)
	}
	return a
}
func (a *Actions) Scroll(x, y float64) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Mouse.Scroll(x, y, 1)
	}
	return a
}
func (a *Actions) KeyDown(key input.Key) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Keyboard.Press(key)
	}
	return a
}
func (a *Actions) KeyUp(key input.Key) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Keyboard.Release(key)
	}
	return a
}
func (a *Actions) Type(keys ...input.Key) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).Keyboard.Type(keys...)
	}
	return a
}
func (a *Actions) Input(text string) *Actions {
	if a.err == nil {
		a.err = a.tab.page.Context(a.ctx).InsertText(text)
	}
	return a
}
func (a *Actions) Wait(d time.Duration) *Actions {
	if a.err == nil {
		a.err = waitDuration(a.ctx, d)
	}
	return a
}
