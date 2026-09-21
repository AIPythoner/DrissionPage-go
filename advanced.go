package drissionpage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AIPythoner/DrissionPage-go/internal/rod"
	"github.com/go-rod/rod/lib/proto"
)

type ChromiumContext = Chromium
type ShadowRoot = ChromiumElement
type TabFilter struct{ Title, URL, Type string }

func (b *Chromium) LatestTab(ctx context.Context) (*ChromiumTab, error) {
	tabs, e := b.Tabs(ctx)
	if e != nil {
		return nil, e
	}
	if len(tabs) == 0 {
		return nil, ErrElementNotFound
	}
	byID := map[string]*ChromiumTab{}
	for _, t := range tabs {
		byID[t.ID()] = t
	}
	b.hooks.mu.Lock()
	order := append([]string(nil), b.hooks.order...)
	b.hooks.mu.Unlock()
	for i := len(order) - 1; i >= 0; i-- {
		if t := byID[order[i]]; t != nil {
			return t, nil
		}
	}
	return tabs[0], nil
}

func (b *Chromium) FindTabs(ctx context.Context, filter TabFilter) ([]*ChromiumTab, error) {
	tabs, e := b.Tabs(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]*ChromiumTab, 0)
	for _, t := range tabs {
		info, e := t.page.Context(ctx).Info()
		if e != nil {
			return nil, e
		}
		if strings.Contains(info.Title, filter.Title) && strings.Contains(info.URL, filter.URL) && (filter.Type == "" || string(info.Type) == filter.Type) {
			out = append(out, t)
		}
	}
	return out, nil
}
func (b *Chromium) TabIDs(ctx context.Context) ([]string, error) {
	tabs, e := b.Tabs(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]string, 0, len(tabs))
	for _, t := range tabs {
		out = append(out, t.ID())
	}
	return out, nil
}
func (b *Chromium) CloseTabs(ctx context.Context, ids []string, others bool) error {
	tabs, e := b.Tabs(ctx)
	if e != nil {
		return e
	}
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	for _, t := range tabs {
		if set[t.ID()] != others {
			if e = t.Close(ctx); e != nil {
				return e
			}
		}
	}
	return nil
}
func (b *Chromium) WaitNewTab(ctx context.Context, previous []string) (*ChromiumTab, error) {
	known := map[string]bool{}
	for _, id := range previous {
		known[id] = true
	}
	ctx, c := context.WithTimeout(ctx, b.options.Timeout)
	defer c()
	for {
		tabs, e := b.Tabs(ctx)
		if e != nil {
			return nil, e
		}
		for _, t := range tabs {
			if !known[t.ID()] {
				return t, nil
			}
		}
		if e = waitDuration(ctx, 50*time.Millisecond); e != nil {
			return nil, e
		}
	}
}
func (e *ChromiumElement) ClickForNewTab(ctx context.Context) (*ChromiumTab, error) {
	ids, err := e.tab.browser.TabIDs(ctx)
	if err != nil {
		return nil, err
	}
	if err = e.Click(ctx); err != nil {
		return nil, err
	}
	return e.tab.browser.WaitNewTab(ctx, ids)
}
func (b *Chromium) ClearCache(ctx context.Context, cookies bool) error {
	if e := (proto.NetworkClearBrowserCache{}).Call(b.browser.Context(ctx)); e != nil {
		return e
	}
	if cookies {
		return (proto.NetworkClearBrowserCookies{}).Call(b.browser.Context(ctx))
	}
	return nil
}
func (b *Chromium) Version(ctx context.Context) (*proto.BrowserGetVersionResult, error) {
	return b.browser.Context(ctx).Version()
}
func (t *ChromiumTab) Browser() *Chromium { return t.browser }
func (t *ChromiumTab) ActiveElement(ctx context.Context) (*ChromiumElement, error) {
	p, c := t.operation(ctx)
	defer c()
	el, e := p.ElementByJS(rod.Eval(`()=>document.activeElement`))
	if e != nil {
		return nil, e
	}
	return &ChromiumElement{el.Context(t.page.GetContext()), t}, nil
}
func (t *ChromiumTab) GetFrame(ctx context.Context, locator any, index ...int) (*ChromiumFrame, error) {
	el, e := t.Ele(ctx, locator, index...)
	if e != nil {
		return nil, e
	}
	return el.Frame(ctx)
}
func (t *ChromiumTab) Frames(ctx context.Context) ([]*ChromiumFrame, error) {
	els, e := t.Eles(ctx, CSS("iframe,frame"))
	if e != nil {
		return nil, e
	}
	out := make([]*ChromiumFrame, 0, len(els))
	for _, el := range els {
		f, e := el.Frame(ctx)
		if e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, nil
}
func (t *ChromiumTab) NewElement(ctx context.Context, markup string) (*ChromiumElement, error) {
	p, c := t.operation(ctx)
	defer c()
	el, e := p.ElementByJS(rod.Eval(`(markup)=>{const template=document.createElement('template');template.innerHTML=markup.trim();const el=template.content.firstElementChild;if(!el)throw new Error('markup has no element');document.body.append(el);return el}`, markup))
	if e != nil {
		return nil, e
	}
	return &ChromiumElement{el.Context(t.page.GetContext()), t}, nil
}
func (t *ChromiumTab) RunExpression(ctx context.Context, expression string) (json.RawMessage, error) {
	p, c := t.operation(ctx)
	defer c()
	r, e := (proto.RuntimeEvaluate{Expression: expression, ReturnByValue: true, AwaitPromise: true}).Call(p)
	if e != nil {
		return nil, e
	}
	if r.ExceptionDetails != nil {
		return nil, fmt.Errorf("JavaScript: %s", r.ExceptionDetails.Text)
	}
	return r.Result.Value.MarshalJSON()
}
func (t *ChromiumTab) ReadyState(ctx context.Context) (string, error) {
	v, e := t.RunJS(ctx, `()=>document.readyState`)
	if e != nil {
		return "", e
	}
	var s string
	e = json.Unmarshal(v, &s)
	return s, e
}
func (t *ChromiumTab) UserAgent(ctx context.Context) (string, error) {
	v, e := t.RunJS(ctx, `()=>navigator.userAgent`)
	if e != nil {
		return "", e
	}
	var s string
	e = json.Unmarshal(v, &s)
	return s, e
}
func (t *ChromiumTab) JSON(ctx context.Context, out any) error {
	v, e := t.RunJS(ctx, `()=>JSON.parse(document.body.innerText)`)
	if e != nil {
		return e
	}
	return json.Unmarshal(v, out)
}
func (t *ChromiumTab) SaveMHTML(ctx context.Context, path string) error {
	p, c := t.operation(ctx)
	defer c()
	r, e := (proto.PageCaptureSnapshot{Format: "mhtml"}).Call(p)
	if e != nil {
		return e
	}
	return os.WriteFile(path, []byte(r.Data), 0600)
}
func (t *ChromiumTab) GetBlob(ctx context.Context, target string) ([]byte, error) {
	data, e := t.RunJS(ctx, `async (url)=>{const r=await fetch(url);if(!r.ok)throw new Error('HTTP '+r.status);const b=await r.blob();return await new Promise((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(reader.result.split(',')[1]);reader.onerror=()=>reject(reader.error);reader.readAsDataURL(b)})}`, target)
	if e != nil {
		return nil, e
	}
	var encoded string
	if e = json.Unmarshal(data, &encoded); e != nil {
		return nil, e
	}
	return base64.StdEncoding.DecodeString(encoded)
}
func (e *ChromiumElement) Src(ctx context.Context) ([]byte, error) {
	target, ok, err := e.Attr(ctx, "src")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("element has no src")
	}
	if !strings.HasPrefix(target, "blob:") && !strings.HasPrefix(target, "data:") {
		p, cancel := e.tab.operation(ctx)
		defer cancel()
		if tree, err := (proto.PageGetResourceTree{}).Call(p); err == nil {
			var find func(*proto.PageFrameResourceTree) proto.PageFrameID
			find = func(tree *proto.PageFrameResourceTree) proto.PageFrameID {
				for _, resource := range tree.Resources {
					if resource.URL == target {
						return tree.Frame.ID
					}
				}
				for _, child := range tree.ChildFrames {
					if id := find(child); id != "" {
						return id
					}
				}
				return ""
			}
			if id := find(tree.FrameTree); id != "" {
				if content, err := (proto.PageGetResourceContent{FrameID: id, URL: target}).Call(p); err == nil {
					if content.Base64Encoded {
						return base64.StdEncoding.DecodeString(content.Content)
					}
					return []byte(content.Content), nil
				}
			}
		}
	}
	return e.tab.GetBlob(ctx, target)
}
func (e *ChromiumElement) Save(ctx context.Context, path string) error {
	data, err := e.Src(ctx)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
func (e *ChromiumElement) RawText(ctx context.Context) (string, error) {
	v, err := e.Property(ctx, "textContent")
	if err != nil {
		return "", err
	}
	var s string
	err = json.Unmarshal(v, &s)
	return s, err
}
func (e *ChromiumElement) InnerHTML(ctx context.Context) (string, error) {
	v, err := e.Property(ctx, "innerHTML")
	if err != nil {
		return "", err
	}
	var s string
	err = json.Unmarshal(v, &s)
	return s, err
}
func (e *ChromiumElement) Pseudo(ctx context.Context, which, style string) (string, error) {
	v, err := e.RunJS(ctx, `function(which,style){return getComputedStyle(this,which).getPropertyValue(style)}`, which, style)
	if err != nil {
		return "", err
	}
	var s string
	err = json.Unmarshal(v, &s)
	return s, err
}
func (e *ChromiumElement) Snapshot(ctx context.Context) (*SessionElement, error) {
	h, err := e.HTML(ctx)
	if err != nil {
		return nil, err
	}
	u, err := e.tab.URL(ctx)
	if err != nil {
		return nil, err
	}
	doc, err := MakeSessionElement(h, u)
	if err != nil {
		return nil, err
	}
	tag, err := e.Tag(ctx)
	if err != nil {
		return nil, err
	}
	return doc.Ele(By("tag name", tag))
}
func (e *ChromiumElement) DragTo(ctx context.Context, target *ChromiumElement, duration time.Duration) error {
	if err := e.ScrollIntoView(ctx); err != nil {
		return err
	}
	r, err := target.Rect(ctx)
	if err != nil {
		return err
	}
	return e.dragPoint(ctx, r.X+r.Width/2, r.Y+r.Height/2, duration)
}
func (e *ChromiumElement) Drag(ctx context.Context, x, y float64, duration time.Duration) error {
	if err := e.ScrollIntoView(ctx); err != nil {
		return err
	}
	r, err := e.Rect(ctx)
	if err != nil {
		return err
	}
	return e.dragPoint(ctx, r.X+r.Width/2+x, r.Y+r.Height/2+y, duration)
}
func (e *ChromiumElement) dragPoint(ctx context.Context, x, y float64, d time.Duration) error {
	r, err := e.Rect(ctx)
	if err != nil {
		return err
	}
	a := e.tab.Actions(ctx).MoveTo(r.X+r.Width/2, r.Y+r.Height/2).Hold("left")
	defer e.tab.Actions(context.Background()).Release("left")
	steps := 20
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		a.MoveTo(r.X+r.Width/2+(x-r.X-r.Width/2)*f, r.Y+r.Height/2+(y-r.Y-r.Height/2)*f).Wait(d / time.Duration(steps))
		if a.err != nil {
			return a.err
		}
	}
	return a.Release("left").Do()
}

// ToSession creates an independent HTTP page with the browser's cookies and UA.
// It is the Go equivalent of cookies_to_session + switching to session mode.
func (t *ChromiumTab) ToSession(ctx context.Context, navigate bool) (*SessionPage, error) {
	ua, e := t.UserAgent(ctx)
	if e != nil {
		return nil, e
	}
	o := NewSessionOptions()
	o.Headers.Set("User-Agent", ua)
	s, e := NewSessionPage(o)
	if e != nil {
		return nil, e
	}
	if e = t.CookiesToSession(ctx, s); e != nil {
		s.Close()
		return nil, e
	}
	if navigate {
		u, e := t.URL(ctx)
		if e != nil {
			s.Close()
			return nil, e
		}
		if _, e = s.Get(ctx, u); e != nil {
			s.Close()
			return nil, e
		}
	}
	return s, nil
}
func (t *ChromiumTab) CookiesToSession(ctx context.Context, s *SessionPage) error {
	cookies, e := t.Cookies(ctx)
	if e != nil {
		return e
	}
	for _, c := range cookies {
		scheme := "http"
		if c.Secure {
			scheme = "https"
		}
		u := scheme + "://" + strings.TrimPrefix(c.Domain, ".") + c.Path
		cookie := &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Secure: c.Secure, HttpOnly: c.HTTPOnly}
		if c.Expires > 0 {
			cookie.Expires = time.Unix(int64(c.Expires), 0)
		}
		if e = s.SetCookies(u, []*http.Cookie{cookie}); e != nil {
			return e
		}
	}
	return nil
}
func (t *ChromiumTab) CookiesFromSession(ctx context.Context, s *SessionPage, target string) error {
	cookies, e := s.Cookies(target)
	if e != nil {
		return e
	}
	params := make([]*proto.NetworkCookieParam, 0, len(cookies))
	for _, c := range cookies {
		params = append(params, &proto.NetworkCookieParam{Name: c.Name, Value: c.Value, URL: target})
	}
	return t.SetCookies(ctx, params)
}
func MakeAbsoluteLink(link, base string) (string, error) {
	u, e := url.Parse(base)
	if e != nil {
		return "", e
	}
	v, e := url.Parse(link)
	if e != nil {
		return "", e
	}
	return u.ResolveReference(v).String(), nil
}
func WaitUntil(ctx context.Context, interval time.Duration, condition func() (bool, error)) error {
	for {
		ok, e := condition()
		if e != nil {
			return e
		}
		if ok {
			return nil
		}
		if e = waitDuration(ctx, interval); e != nil {
			return e
		}
	}
}
