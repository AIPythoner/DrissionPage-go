package drissionpage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// LoadFile loads local HTML without making a network request.
func (p *SessionPage) LoadFile(path string) error {
	data, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return p.LoadHTML(string(data), "")
}
func (p *SessionPage) LoadHTML(markup, baseURL string) error {
	doc, e := MakeSessionElement(markup, baseURL)
	if e != nil {
		return e
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	p.document = doc
	p.response = &Response{URL: baseURL, StatusCode: 200, Headers: make(http.Header), Body: []byte(markup), Text: markup}
	return nil
}
func (p *SessionPage) RawData() ([]byte, error) {
	r, e := p.Response()
	if e != nil {
		return nil, e
	}
	return r.Body, nil
}
func (p *SessionPage) Save(path string) error {
	r, e := p.Response()
	if e != nil {
		return e
	}
	return os.WriteFile(path, r.Body, 0600)
}
func (p *SessionPage) UserAgent() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.options.Headers.Get("User-Agent")
}
func (p *SessionPage) DeleteCookie(target, name string) error {
	u, e := url.Parse(target)
	if e != nil {
		return e
	}
	if all, e := p.AllCookies(); e == nil {
		for _, c := range all {
			if c.Name == name && (u.Hostname() == c.Domain || strings.HasSuffix(u.Hostname(), "."+c.Domain)) {
				c.MaxAge = -1
				c.Expires = time.Unix(1, 0)
				p.Client().Jar.SetCookies(u, []*http.Cookie{c})
			}
		}
	} else {
		p.Client().Jar.SetCookies(u, []*http.Cookie{{Name: name, MaxAge: -1, Expires: time.Unix(1, 0), Path: "/"}})
	}
	return nil
}
func (p *SessionPage) ClearCookies(target string) error {
	cookies, e := p.Cookies(target)
	if e != nil {
		return e
	}
	for _, c := range cookies {
		if e = p.DeleteCookie(target, c.Name); e != nil {
			return e
		}
	}
	return nil
}
func (p *SessionPage) Head(ctx context.Context, target string, options ...RequestOptions) (*Response, error) {
	return p.Request(ctx, http.MethodHead, target, options...)
}
func (p *SessionPage) Put(ctx context.Context, target string, options ...RequestOptions) (*Response, error) {
	return p.Request(ctx, http.MethodPut, target, options...)
}
func (p *SessionPage) Delete(ctx context.Context, target string, options ...RequestOptions) (*Response, error) {
	return p.Request(ctx, http.MethodDelete, target, options...)
}
func (p *SessionPage) WithOptions(options *SessionOptions) (*SessionPage, error) {
	if options == nil {
		return nil, fmt.Errorf("options required")
	}
	copy := *options
	client := *p.Client()
	copy.Client = &client
	client.Timeout = options.Timeout
	return NewSessionPage(&copy)
}
