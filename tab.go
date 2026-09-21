package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/AIPythoner/DrissionPage-go/internal/rod"
	"github.com/go-rod/rod/lib/proto"
)

type ChromiumTab struct {
	page    *rod.Page
	browser *Chromium
}
type ChromiumFrame struct {
	*ChromiumTab
	element *ChromiumElement
}

func (t *ChromiumTab) Raw() *rod.Page { return t.page }
func (t *ChromiumTab) ID() string     { return string(t.page.TargetID) }
func (t *ChromiumTab) operation(ctx context.Context) (*rod.Page, context.CancelFunc) {
	c, cancel := context.WithTimeout(ctx, t.browser.options.Timeout)
	return t.page.Context(c), cancel
}
func (t *ChromiumTab) Get(ctx context.Context, target string) error {
	var err error
	for attempt := 0; attempt <= t.browser.options.RetryTimes; attempt++ {
		if attempt > 0 {
			if err = waitDuration(ctx, t.browser.options.RetryInterval); err != nil {
				return err
			}
		}
		err = t.navigateOnce(ctx, target)
		if err == nil || ctx.Err() != nil {
			return err
		}
	}
	return err
}
func (t *ChromiumTab) navigateOnce(ctx context.Context, target string) error {
	loadCtx, cancel := context.WithTimeout(ctx, t.browser.options.PageLoadTimeout)
	p := t.page.Context(loadCtx)
	defer cancel()
	if e := p.Navigate(target); e != nil {
		return e
	}
	switch t.browser.options.LoadMode {
	case "none":
		return nil
	case "eager":
		return p.Wait(rod.Eval(`() => document.readyState !== 'loading'`))
	default:
		return p.WaitLoad()
	}
}
func (t *ChromiumTab) HTML(ctx context.Context) (string, error) {
	p, c := t.operation(ctx)
	defer c()
	return p.HTML()
}
func (t *ChromiumTab) Title(ctx context.Context) (string, error) {
	p, c := t.operation(ctx)
	defer c()
	r, e := p.Eval(`() => document.title`)
	if e != nil {
		return "", e
	}
	return r.Value.Str(), nil
}
func (t *ChromiumTab) URL(ctx context.Context) (string, error) {
	p, c := t.operation(ctx)
	defer c()
	r, e := p.Eval(`() => location.href`)
	if e != nil {
		return "", e
	}
	return r.Value.Str(), nil
}
func (t *ChromiumTab) RunJS(ctx context.Context, function string, args ...any) (json.RawMessage, error) {
	jsCtx, c := context.WithTimeout(ctx, t.browser.options.ScriptTimeout)
	p := t.page.Context(jsCtx)
	defer c()
	r, e := p.Eval(function, args...)
	if e != nil {
		return nil, e
	}
	return r.Value.MarshalJSON()
}
func (t *ChromiumTab) RunCDP(ctx context.Context, method string, params any) (json.RawMessage, error) {
	p, c := t.operation(ctx)
	defer c()
	return t.browser.browser.Call(p.GetContext(), string(p.SessionID), method, params)
}
func (t *ChromiumTab) Eles(ctx context.Context, value any) ([]*ChromiumElement, error) {
	loc, e := ParseLocator(value)
	if e != nil {
		return nil, e
	}
	p, c := t.operation(ctx)
	defer c()
	var els rod.Elements
	switch loc.Kind {
	case "ax":
		els, e = accessibilityElements(p, loc.Value)
	case "css":
		els, e = p.Elements(loc.Value)
	case "xpath":
		els, e = p.ElementsX(loc.Value)
	case "search":
		// DOM.performSearch covers CSS, XPath and plain text, including shadow DOM.
		if err := (proto.DOMEnable{}).Call(p); err != nil {
			return nil, err
		}
		if _, err := (proto.DOMGetDocument{}).Call(p); err != nil {
			return nil, err
		}
		var r *proto.DOMPerformSearchResult
		r, e = (proto.DOMPerformSearch{Query: loc.Value, IncludeUserAgentShadowDOM: true}).Call(p)
		if e == nil {
			defer (proto.DOMDiscardSearchResults{SearchID: r.SearchID}).Call(p)
			if r.ResultCount > 0 {
				var found *proto.DOMGetSearchResultsResult
				found, e = (proto.DOMGetSearchResults{SearchID: r.SearchID, FromIndex: 0, ToIndex: r.ResultCount}).Call(p)
				if e == nil {
					for _, id := range found.NodeIDs {
						el, err := p.ElementFromNode(&proto.DOMNode{NodeID: id})
						if err != nil {
							return nil, err
						}
						els = append(els, el)
					}
				}
			}
		}
	}
	if e != nil {
		return nil, e
	}
	out := make([]*ChromiumElement, 0, len(els))
	for _, el := range els {
		out = append(out, &ChromiumElement{el.Context(t.page.GetContext()), t})
	}
	return out, nil
}
func (t *ChromiumTab) Ele(ctx context.Context, value any, index ...int) (*ChromiumElement, error) {
	i := 1
	if len(index) > 0 {
		i = index[0]
	}
	if i == 0 {
		return nil, ErrInvalidIndex
	}
	ctx, cancel := context.WithTimeout(ctx, t.browser.options.Timeout)
	defer cancel()
	for {
		els, e := t.Eles(ctx, value)
		if e != nil {
			return nil, e
		}
		el, e := selectIndex(els, i)
		if e == nil {
			return el, nil
		}
		if e = waitDuration(ctx, 50*time.Millisecond); e != nil {
			return nil, fmt.Errorf("%w: %v: %w", ErrElementNotFound, value, e)
		}
	}
}
func (t *ChromiumTab) SEle(ctx context.Context, value any, index ...int) (*SessionElement, error) {
	s, e := t.Snapshot(ctx)
	if e != nil {
		return nil, e
	}
	return s.Ele(value, index...)
}
func (t *ChromiumTab) Snapshot(ctx context.Context) (*SessionElement, error) {
	h, e := t.HTML(ctx)
	if e != nil {
		return nil, e
	}
	u, e := t.URL(ctx)
	if e != nil {
		return nil, e
	}
	return MakeSessionElement(h, u)
}
func (t *ChromiumTab) Screenshot(ctx context.Context, path string, fullPage bool) ([]byte, error) {
	p, c := t.operation(ctx)
	defer c()
	data, e := p.Screenshot(fullPage, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if e == nil && path != "" {
		e = os.WriteFile(path, data, 0600)
	}
	return data, e
}
func (t *ChromiumTab) PDF(ctx context.Context, path string, options ...proto.PagePrintToPDF) ([]byte, error) {
	p, c := t.operation(ctx)
	defer c()
	o := proto.PagePrintToPDF{PrintBackground: true}
	if len(options) > 0 {
		o = options[0]
	}
	r, e := p.PDF(&o)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	data, e := io.ReadAll(r)
	if e == nil && path != "" {
		e = os.WriteFile(path, data, 0600)
	}
	return data, e
}
func (t *ChromiumTab) Save(ctx context.Context, path string) error {
	h, e := t.HTML(ctx)
	if e != nil {
		return e
	}
	return os.WriteFile(path, []byte(h), 0600)
}
func (t *ChromiumTab) Activate(ctx context.Context) error {
	_, e := t.page.Context(ctx).Activate()
	return e
}
func (t *ChromiumTab) Close(ctx context.Context) error { return t.page.Context(ctx).Close() }
func (t *ChromiumTab) Back(ctx context.Context) error {
	p, c := t.operation(ctx)
	defer c()
	return p.NavigateBack()
}
func (t *ChromiumTab) Forward(ctx context.Context) error {
	p, c := t.operation(ctx)
	defer c()
	return p.NavigateForward()
}
func (t *ChromiumTab) Refresh(ctx context.Context) error {
	p, c := t.operation(ctx)
	defer c()
	if e := (proto.PageReload{}).Call(p); e != nil {
		return e
	}
	return p.WaitLoad()
}
func (t *ChromiumTab) StopLoading(ctx context.Context) error {
	return (proto.PageStopLoading{}).Call(t.page.Context(ctx))
}
func (t *ChromiumTab) Cookies(ctx context.Context) ([]*proto.NetworkCookie, error) {
	p, c := t.operation(ctx)
	defer c()
	return p.Cookies(nil)
}
func (t *ChromiumTab) SetCookies(ctx context.Context, cookies []*proto.NetworkCookieParam) error {
	p, c := t.operation(ctx)
	defer c()
	return p.SetCookies(cookies)
}
func (t *ChromiumTab) WaitLoad(ctx context.Context) error {
	p, c := t.operation(ctx)
	defer c()
	return p.WaitLoad()
}
func (t *ChromiumTab) WaitJS(ctx context.Context, function string, args ...any) error {
	p, c := t.operation(ctx)
	defer c()
	return p.Wait(rod.Eval(function, args...))
}
func (t *ChromiumTab) Scroll(ctx context.Context, x, y float64) error {
	_, e := t.RunJS(ctx, `(x,y) => window.scrollBy(x,y)`, x, y)
	return e
}
func (t *ChromiumTab) SetViewport(ctx context.Context, width, height int) error {
	return t.page.Context(ctx).SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: width, Height: height, DeviceScaleFactor: 1})
}
func (t *ChromiumTab) SetHeaders(ctx context.Context, headers map[string]string) error {
	h := proto.NetworkHeaders{}
	for k, v := range headers {
		h[k] = toJSON(v)
	}
	return (proto.NetworkSetExtraHTTPHeaders{Headers: h}).Call(t.page.Context(ctx))
}
