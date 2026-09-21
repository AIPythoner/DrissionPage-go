package drissionpage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"
)

type SessionOptions struct {
	Timeout               time.Duration
	Headers               http.Header
	Proxy                 string
	RetryTimes            int
	RetryInterval         time.Duration
	MaxBodyBytes          int64
	Client                *http.Client `json:"-"`
	Params                url.Values
	Username              string
	Password              string `json:"-"`
	Encoding              string
	HTTPProxy, HTTPSProxy string
	VerifyTLS, TrustEnv   *bool
	MaxRedirects          *int
	CAFile                string
	CertFile, KeyFile     string
	Hooks                 []ResponseHook               `json:"-"`
	Adapters              map[string]http.RoundTripper `json:"-"`
}

func NewSessionOptions() *SessionOptions {
	return &SessionOptions{Timeout: 30 * time.Second, RetryInterval: time.Second, MaxBodyBytes: 64 << 20, Headers: make(http.Header)}
}

type RequestOptions struct {
	Headers  http.Header
	Params   url.Values
	Data     []byte
	JSON     any
	Form     url.Values
	Files    map[string]string
	Cookies  []*http.Cookie
	Username string
	Password string
	// RetryUnsafe opts into retrying methods other than GET/HEAD/OPTIONS.
	RetryUnsafe bool
}
type Response struct {
	URL        string
	StatusCode int
	Headers    http.Header
	Body       []byte
	Text       string
}

func (r *Response) JSON(out any) error { return json.Unmarshal(r.Body, out) }

type SessionPage struct {
	client         *http.Client
	options        SessionOptions
	mu             sync.RWMutex
	response       *Response
	document       *SessionElement
	closed         bool
	requestTimeout *time.Duration
}

func NewSessionPage(options ...*SessionOptions) (*SessionPage, error) {
	o := *NewSessionOptions()
	if len(options) > 0 && options[0] != nil {
		o = *options[0]
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	if o.MaxBodyBytes <= 0 {
		o.MaxBodyBytes = 64 << 20
	}
	if o.RetryTimes < 0 {
		return nil, fmt.Errorf("negative retry count")
	}
	o.Headers = o.Headers.Clone()
	o.Params = cloneValues(o.Params)
	client := &http.Client{Timeout: o.Timeout}
	if o.Client != nil {
		*client = *o.Client
	}
	if client.Jar == nil {
		jar, err := newTrackedJar()
		if err != nil {
			return nil, err
		}
		client.Jar = jar
	}
	if o.Proxy != "" {
		u, err := url.Parse(o.Proxy)
		if err != nil {
			return nil, err
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(u)
		client.Transport = transport
	}
	page := &SessionPage{client: client, options: o}
	if err := page.applyOptions(o); err != nil {
		page.Close()
		return nil, err
	}
	return page, nil
}
func (p *SessionPage) Client() *http.Client { p.mu.RLock(); defer p.mu.RUnlock(); return p.client }
func (p *SessionPage) Get(ctx context.Context, target string, options ...RequestOptions) (*Response, error) {
	return p.Request(ctx, http.MethodGet, target, options...)
}
func (p *SessionPage) Post(ctx context.Context, target string, options ...RequestOptions) (*Response, error) {
	return p.Request(ctx, http.MethodPost, target, options...)
}
func (p *SessionPage) Request(ctx context.Context, method, target string, options ...RequestOptions) (*Response, error) {
	p.mu.RLock()
	closed := p.closed
	settings := p.options
	settings.Headers = p.options.Headers.Clone()
	settings.Params = cloneValues(p.options.Params)
	client := *p.client
	if p.requestTimeout != nil {
		client.Timeout = *p.requestTimeout
	}
	p.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	o := RequestOptions{}
	if len(options) > 0 {
		o = options[0]
	}
	var err error
	retries := settings.RetryTimes
	method = strings.ToUpper(method)
	if method != "GET" && method != "HEAD" && method != "OPTIONS" && !o.RetryUnsafe {
		retries = 0
	}
	var result *Response
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			if err = waitDuration(ctx, settings.RetryInterval); err != nil {
				return result, err
			}
		}
		req, e := buildSessionRequest(ctx, method, target, settings, o)
		if e != nil {
			return nil, e
		}
		res, e := client.Do(req)
		if e != nil {
			err = e
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		data, e := io.ReadAll(io.LimitReader(res.Body, settings.MaxBodyBytes+1))
		res.Body.Close()
		if e != nil {
			err = e
			continue
		}
		if int64(len(data)) > settings.MaxBodyBytes {
			return nil, fmt.Errorf("response exceeds %d bytes", settings.MaxBodyBytes)
		}
		decoded := data
		var reader io.Reader
		if len(data) == 0 {
			e = nil
		} else if settings.Encoding != "" {
			reader, e = charset.NewReaderLabel(settings.Encoding, bytes.NewReader(data))
		} else {
			reader, e = charset.NewReader(bytes.NewReader(data), res.Header.Get("Content-Type"))
		}
		if e != nil {
			return nil, e
		}
		if reader != nil {
			decoded, e = io.ReadAll(reader)
			if e != nil {
				return nil, e
			}
		}
		result = &Response{res.Request.URL.String(), res.StatusCode, res.Header.Clone(), data, string(decoded)}
		if res.StatusCode >= 400 {
			err = &HTTPError{res.StatusCode, result.URL}
			if res.StatusCode >= 500 || res.StatusCode == 429 {
				continue
			}
		} else {
			err = nil
		}
		break
	}
	if result != nil {
		doc, e := MakeSessionElement(result.Text, result.URL)
		if e != nil {
			return result, e
		}
		p.mu.Lock()
		if !p.closed {
			copy := *result
			copy.Body = bytes.Clone(result.Body)
			copy.Headers = result.Headers.Clone()
			p.response = &copy
			p.document = doc
		}
		p.mu.Unlock()
	}
	return result, err
}
func waitDuration(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (p *SessionPage) Response() (*Response, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.response == nil {
		return nil, ErrNoResponse
	}
	r := *p.response
	r.Body = bytes.Clone(r.Body)
	r.Headers = r.Headers.Clone()
	return &r, nil
}
func (p *SessionPage) HTML() (string, error) {
	r, e := p.Response()
	if e != nil {
		return "", e
	}
	return r.Text, nil
}
func (p *SessionPage) URL() (string, error) {
	r, e := p.Response()
	if e != nil {
		return "", e
	}
	return r.URL, nil
}
func (p *SessionPage) JSON(out any) error {
	r, e := p.Response()
	if e != nil {
		return e
	}
	return r.JSON(out)
}
func (p *SessionPage) Eles(locator any) ([]*SessionElement, error) {
	p.mu.RLock()
	doc := p.document
	p.mu.RUnlock()
	if doc == nil {
		return nil, ErrNoResponse
	}
	return doc.Eles(locator)
}
func (p *SessionPage) Ele(locator any, index ...int) (*SessionElement, error) {
	i := 1
	if len(index) > 0 {
		i = index[0]
	}
	els, e := p.Eles(locator)
	if e != nil {
		return nil, e
	}
	return selectIndex(els, i)
}
func (p *SessionPage) Title() (string, error) {
	el, e := p.Ele("tag:title")
	if e != nil {
		return "", e
	}
	return el.Text(), nil
}
func (p *SessionPage) Cookies(target string) ([]*http.Cookie, error) {
	u, e := url.Parse(target)
	if e != nil {
		return nil, e
	}
	return p.Client().Jar.Cookies(u), nil
}
func (p *SessionPage) SetCookies(target string, cookies []*http.Cookie) error {
	u, e := url.Parse(target)
	if e != nil {
		return e
	}
	p.Client().Jar.SetCookies(u, cookies)
	return nil
}
func (p *SessionPage) Close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.Client().CloseIdleConnections()
}

func buildSessionRequest(ctx context.Context, method, target string, settings SessionOptions, o RequestOptions) (*http.Request, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, vs := range settings.Params {
		if _, exists := q[k]; !exists {
			q[k] = append([]string(nil), vs...)
		}
	}
	for k, vs := range o.Params {
		q[k] = append([]string(nil), vs...)
	}
	u.RawQuery = q.Encode()
	body := o.Data
	contentType := ""
	if o.JSON != nil {
		body, err = json.Marshal(o.JSON)
		contentType = "application/json"
	} else if o.Form != nil {
		body = []byte(o.Form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}
	if err != nil {
		return nil, err
	}
	if len(o.Files) > 0 {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for k, vs := range o.Form {
			for _, v := range vs {
				if err = w.WriteField(k, v); err != nil {
					return nil, err
				}
			}
		}
		for field, path := range o.Files {
			f, e := os.Open(path)
			if e != nil {
				return nil, e
			}
			part, e := w.CreateFormFile(field, filepath.Base(path))
			if e == nil {
				_, e = io.Copy(part, f)
			}
			f.Close()
			if e != nil {
				return nil, e
			}
		}
		if err = w.Close(); err != nil {
			return nil, err
		}
		body = buf.Bytes()
		contentType = w.FormDataContentType()
	}
	req, e := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header = settings.Headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, vs := range o.Headers {
		req.Header[k] = append([]string(nil), vs...)
	}
	for _, c := range o.Cookies {
		req.AddCookie(c)
	}
	if o.Username != "" {
		req.SetBasicAuth(o.Username, o.Password)
	} else if settings.Username != "" {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	return req, nil
}
