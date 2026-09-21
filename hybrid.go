package drissionpage

import (
	"context"
	"fmt"
	"sync"
)

// HybridPage is the mode-switching facade. Typed ChromiumTab and SessionPage
// remain available when the caller wants a specific backend without a union.
type HybridPage struct {
	tab     *ChromiumTab
	session *SessionPage
	mu      sync.RWMutex
	mode    string
}

func (t *ChromiumTab) Hybrid(ctx context.Context) (*HybridPage, error) {
	s, e := t.ToSession(ctx, false)
	if e != nil {
		return nil, e
	}
	return &HybridPage{tab: t, session: s, mode: "d"}, nil
}
func (p *HybridPage) Mode() string              { p.mu.RLock(); defer p.mu.RUnlock(); return p.mode }
func (p *HybridPage) BrowserTab() *ChromiumTab  { return p.tab }
func (p *HybridPage) SessionPage() *SessionPage { return p.session }
func (p *HybridPage) ChangeMode(ctx context.Context, mode string, navigate, copyCookies bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if mode == "" {
		if p.mode == "d" {
			mode = "s"
		} else {
			mode = "d"
		}
	}
	if mode != "s" && mode != "d" {
		return fmt.Errorf("mode must be d or s")
	}
	if mode == p.mode {
		return nil
	}
	if mode == "s" {
		if copyCookies {
			if e := p.tab.CookiesToSession(ctx, p.session); e != nil {
				return e
			}
		}
		if navigate {
			u, e := p.tab.URL(ctx)
			if e != nil {
				return e
			}
			if _, e = p.session.Get(ctx, u); e != nil {
				return e
			}
		}
	} else {
		u, e := p.session.URL()
		if e != nil && (copyCookies || navigate) {
			return e
		}
		if copyCookies {
			if e = p.tab.CookiesFromSession(ctx, p.session, u); e != nil {
				return e
			}
		}
		if navigate {
			if e = p.tab.Get(ctx, u); e != nil {
				return e
			}
		}
	}
	p.mode = mode
	return nil
}
func (p *HybridPage) Get(ctx context.Context, target string, options ...RequestOptions) error {
	if p.Mode() == "s" {
		_, e := p.session.Get(ctx, target, options...)
		return e
	}
	return p.tab.Get(ctx, target)
}
func (p *HybridPage) Post(ctx context.Context, target string, options ...RequestOptions) (*Response, error) {
	if e := p.tab.CookiesToSession(ctx, p.session); e != nil {
		return nil, e
	}
	return p.session.Post(ctx, target, options...)
}
func (p *HybridPage) HTML(ctx context.Context) (string, error) {
	if p.Mode() == "s" {
		return p.session.HTML()
	}
	return p.tab.HTML(ctx)
}
func (p *HybridPage) URL(ctx context.Context) (string, error) {
	if p.Mode() == "s" {
		return p.session.URL()
	}
	return p.tab.URL(ctx)
}
func (p *HybridPage) Title(ctx context.Context) (string, error) {
	if p.Mode() == "s" {
		return p.session.Title()
	}
	return p.tab.Title(ctx)
}
func (p *HybridPage) Ele(ctx context.Context, locator any, index ...int) (*PageElement, error) {
	if p.Mode() == "s" {
		e, err := p.session.Ele(locator, index...)
		if err != nil {
			return nil, err
		}
		return &PageElement{session: e}, nil
	}
	e, err := p.tab.Ele(ctx, locator, index...)
	if err != nil {
		return nil, err
	}
	return &PageElement{browser: e}, nil
}
func (p *HybridPage) Eles(ctx context.Context, locator any) ([]*PageElement, error) {
	out := make([]*PageElement, 0)
	if p.Mode() == "s" {
		els, e := p.session.Eles(locator)
		if e != nil {
			return nil, e
		}
		for _, el := range els {
			out = append(out, &PageElement{session: el})
		}
	} else {
		els, e := p.tab.Eles(ctx, locator)
		if e != nil {
			return nil, e
		}
		for _, el := range els {
			out = append(out, &PageElement{browser: el})
		}
	}
	return out, nil
}
func (p *HybridPage) Close(ctx context.Context) error { p.session.Close(); return p.tab.Close(ctx) }

type PageElement struct {
	browser *ChromiumElement
	session *SessionElement
}

func (e *PageElement) BrowserElement() (*ChromiumElement, error) {
	if e.browser == nil {
		return nil, fmt.Errorf("operation requires browser mode")
	}
	return e.browser, nil
}
func (e *PageElement) Text(ctx context.Context) (string, error) {
	if e.session != nil {
		return e.session.Text(), nil
	}
	return e.browser.Text(ctx)
}
func (e *PageElement) HTML(ctx context.Context) (string, error) {
	if e.session != nil {
		return e.session.HTML(), nil
	}
	return e.browser.HTML(ctx)
}
func (e *PageElement) Attr(ctx context.Context, name string) (string, bool, error) {
	if e.session != nil {
		v, ok := e.session.Attr(name)
		return v, ok, nil
	}
	return e.browser.Attr(ctx, name)
}
func (e *PageElement) Ele(ctx context.Context, locator any, index ...int) (*PageElement, error) {
	if e.session != nil {
		el, err := e.session.Ele(locator, index...)
		if err != nil {
			return nil, err
		}
		return &PageElement{session: el}, nil
	}
	el, err := e.browser.Ele(ctx, locator, index...)
	if err != nil {
		return nil, err
	}
	return &PageElement{browser: el}, nil
}
func (e *PageElement) Click(ctx context.Context) error {
	el, err := e.BrowserElement()
	if err != nil {
		return err
	}
	return el.Click(ctx)
}
func (e *PageElement) Input(ctx context.Context, text string, clear ...bool) error {
	el, err := e.BrowserElement()
	if err != nil {
		return err
	}
	return el.Input(ctx, text, clear...)
}
