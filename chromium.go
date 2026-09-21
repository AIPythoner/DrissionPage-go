package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AIPythoner/DrissionPage-go/internal/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
)

type ChromiumOptions struct {
	BrowserPath     string
	Address         string
	UserDataPath    string
	Headless        bool
	Arguments       []string
	Proxy           string
	Preferences     map[string]any
	Timeout         time.Duration
	PageLoadTimeout time.Duration
	ScriptTimeout   time.Duration
	Extensions      []string
	RetryTimes      int
	RetryInterval   time.Duration
	LoadMode        string
}

func NewChromiumOptions() *ChromiumOptions {
	return &ChromiumOptions{Timeout: 10 * time.Second, PageLoadTimeout: 30 * time.Second, ScriptTimeout: 30 * time.Second, LoadMode: "normal"}
}
func (o *ChromiumOptions) SetArgument(arg string) *ChromiumOptions {
	o.Arguments = append(o.Arguments, arg)
	return o
}
func (o *ChromiumOptions) SetHeadless(on bool) *ChromiumOptions { o.Headless = on; return o }
func (o *ChromiumOptions) SetBrowserPath(path string) *ChromiumOptions {
	o.BrowserPath = path
	return o
}
func (o *ChromiumOptions) SetAddress(address string) *ChromiumOptions { o.Address = address; return o }
func (o *ChromiumOptions) Save(path string) error {
	data, e := json.MarshalIndent(o, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, data, 0600)
}
func LoadChromiumOptions(path string) (*ChromiumOptions, error) {
	data, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	o := NewChromiumOptions()
	e = json.Unmarshal(data, o)
	return o, e
}
func (o *SessionOptions) Save(path string) error {
	data, e := json.MarshalIndent(o, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, data, 0600)
}
func LoadSessionOptions(path string) (*SessionOptions, error) {
	data, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	o := NewSessionOptions()
	e = json.Unmarshal(data, o)
	return o, e
}

type Chromium struct {
	browser          *rod.Browser
	options          ChromiumOptions
	launcher         *launcher.Launcher
	cancel           context.CancelFunc
	owned            bool
	temporaryProfile bool
	closeOnce        sync.Once
	closeErr         error
	hooks            *browserHooks
	endpoint         string
	connectionCancel context.CancelFunc
	closed           bool
}

// NewChromium starts a browser or connects to Address. ctx controls its lifetime.
// With Address set, Close only disconnects; Quit explicitly closes that browser.
func NewChromium(ctx context.Context, options ...*ChromiumOptions) (*Chromium, error) {
	o := *NewChromiumOptions()
	if len(options) > 0 && options[0] != nil {
		o = *options[0]
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	if o.PageLoadTimeout <= 0 {
		o.PageLoadTimeout = 30 * time.Second
	}
	if o.ScriptTimeout <= 0 {
		o.ScriptTimeout = 30 * time.Second
	}
	if o.RetryTimes < 0 {
		return nil, fmt.Errorf("negative retry count")
	}
	if o.LoadMode == "" {
		o.LoadMode = "normal"
	}
	if o.LoadMode != "normal" && o.LoadMode != "eager" && o.LoadMode != "none" {
		return nil, fmt.Errorf("invalid load mode %q", o.LoadMode)
	}
	lifetime, cancel := context.WithCancel(ctx)
	b := &Chromium{options: o, cancel: cancel, owned: o.Address == "", hooks: newBrowserHooks()}
	address := o.Address
	if address == "" {
		bin := o.BrowserPath
		if bin == "" {
			var ok bool
			bin, ok = launcher.LookPath()
			if !ok {
				cancel()
				return nil, fmt.Errorf("Chromium not found; set BrowserPath")
			}
		}
		l := launcher.New().Context(lifetime).Bin(bin).Headless(o.Headless)
		if o.UserDataPath != "" {
			l.UserDataDir(o.UserDataPath)
		} else {
			b.temporaryProfile = true
		}
		if o.Proxy != "" {
			l.Proxy(o.Proxy)
		}
		if o.Preferences != nil {
			data, e := browserPreferences(o)
			if e != nil {
				cancel()
				return nil, e
			}
			l.Preferences(string(data))
		}
		if len(o.Extensions) > 0 {
			l.Delete("disable-extensions")
			l.Set("load-extension", strings.Join(o.Extensions, ","))
		}
		for _, arg := range o.Arguments {
			parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
			if len(parts) == 2 {
				l.Set(flags.Flag(parts[0]), parts[1])
			} else {
				l.Set(flags.Flag(parts[0]))
			}
		}
		b.launcher = l
		var e error
		address, e = l.Launch()
		if e != nil {
			cancel()
			return nil, e
		}
	} else if !strings.HasPrefix(address, "ws://") && !strings.HasPrefix(address, "wss://") {
		if !strings.Contains(address, "://") {
			address = "http://" + address
		}
		req, e := http.NewRequestWithContext(lifetime, http.MethodGet, strings.TrimRight(address, "/")+"/json/version", nil)
		if e != nil {
			cancel()
			return nil, e
		}
		client := http.Client{Timeout: o.Timeout}
		res, e := client.Do(req)
		if e != nil {
			cancel()
			return nil, e
		}
		var v struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		e = json.NewDecoder(res.Body).Decode(&v)
		res.Body.Close()
		if e != nil {
			cancel()
			return nil, e
		}
		if v.WebSocketDebuggerURL == "" {
			cancel()
			return nil, fmt.Errorf("debug endpoint has no websocket URL")
		}
		address = v.WebSocketDebuggerURL
	}
	b.endpoint = address
	connection, disconnect := context.WithCancel(lifetime)
	b.connectionCancel = disconnect
	b.browser = rod.New().Context(connection).ControlURL(address).NoDefaultDevice()
	if e := b.browser.Connect(); e != nil {
		cancel()
		if b.launcher != nil {
			b.launcher.Kill()
			if b.temporaryProfile {
				b.launcher.Cleanup()
			}
		}
		return nil, e
	}
	return b, nil
}
func (b *Chromium) Raw() *rod.Browser { return b.browser }
func (b *Chromium) NewTab(ctx context.Context, target string) (*ChromiumTab, error) {
	return b.OpenTab(ctx, TabOptions{URL: target})
}

type TabOptions struct {
	URL                           string
	NewWindow, Background, Hidden bool
}

func (b *Chromium) OpenTab(ctx context.Context, options TabOptions) (*ChromiumTab, error) {
	target := options.URL
	if target == "" {
		target = "about:blank"
	}
	params := map[string]any{"url": "about:blank"}
	if options.NewWindow {
		params["newWindow"] = true
	}
	if options.Background {
		params["background"] = true
	}
	if options.Hidden {
		params["hidden"] = true
	}
	if b.browser.BrowserContextID != "" {
		params["browserContextId"] = b.browser.BrowserContextID
	}
	data, e := b.RunCDP(ctx, "Target.createTarget", params)
	if e != nil {
		return nil, e
	}
	var created proto.TargetCreateTargetResult
	if e = json.Unmarshal(data, &created); e != nil {
		return nil, e
	}
	p, e := b.browser.PageFromTarget(created.TargetID)
	if e != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = (proto.TargetCloseTarget{TargetID: created.TargetID}).Call(b.browser.Context(cleanup))
		return nil, e
	}
	t := &ChromiumTab{page: p.Context(b.browser.GetContext()), browser: b}
	if b.hooks != nil {
		b.hooks.notify(t)
	}
	if target != "about:blank" {
		if e = t.Get(ctx, target); e != nil {
			_ = t.Close(context.Background())
			return nil, e
		}
	}
	return t, nil
}
func (b *Chromium) Tabs(ctx context.Context) ([]*ChromiumTab, error) {
	list, e := (proto.TargetGetTargets{}).Call(b.browser.Context(ctx))
	if e != nil {
		return nil, e
	}
	out := make([]*ChromiumTab, 0, len(list.TargetInfos))
	for _, info := range list.TargetInfos {
		if info.Type != proto.TargetTargetInfoTypePage {
			continue
		}
		if b.browser.BrowserContextID != "" && info.BrowserContextID != b.browser.BrowserContextID {
			continue
		}
		p, e := b.browser.PageFromTarget(info.TargetID)
		if e != nil {
			return nil, e
		}
		out = append(out, &ChromiumTab{page: p.Context(b.browser.GetContext()), browser: b})
	}
	return out, nil
}
func (b *Chromium) GetTab(ctx context.Context, id string) (*ChromiumTab, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	p, e := b.browser.PageFromTarget(proto.TargetTargetID(id))
	if e != nil {
		return nil, e
	}
	return &ChromiumTab{page: p.Context(b.browser.GetContext()), browser: b}, nil
}

type ContextOptions struct {
	Proxy, ProxyBypass string
	DisposeOnDetach    bool
}

func (b *Chromium) NewContext(ctx context.Context, options ...ContextOptions) (*Chromium, error) {
	o := ContextOptions{DisposeOnDetach: true}
	if len(options) > 0 {
		o = options[0]
	}
	created, e := (proto.TargetCreateBrowserContext{ProxyServer: o.Proxy, ProxyBypassList: o.ProxyBypass, DisposeOnDetach: o.DisposeOnDetach}).Call(b.browser.Context(ctx))
	if e != nil {
		return nil, e
	}
	r := b.browser.Context(b.browser.GetContext())
	r.BrowserContextID = created.BrowserContextID
	life, cancel := context.WithCancel(b.browser.GetContext())
	return &Chromium{browser: r.Context(life), options: b.options, cancel: cancel, owned: true, hooks: b.hooks}, nil
}
func (b *Chromium) RunCDP(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return b.browser.Call(ctx, "", method, params)
}
func (b *Chromium) Cookies(ctx context.Context) ([]*proto.NetworkCookie, error) {
	return b.browser.Context(ctx).GetCookies()
}
func (b *Chromium) SetCookies(ctx context.Context, cookies []*proto.NetworkCookieParam) error {
	return b.browser.Context(ctx).SetCookies(cookies)
}
func (b *Chromium) Close() error {
	b.closeOnce.Do(func() {
		b.closed = true
		if b.owned {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			b.closeErr = b.browser.Context(ctx).Close()
		}
		if b.connectionCancel != nil {
			b.connectionCancel()
		}
		b.cancel()
		if b.launcher != nil {
			b.launcher.Kill()
			if b.temporaryProfile {
				b.launcher.Cleanup()
			}
		}
	})
	return b.closeErr
}
func (b *Chromium) Quit(ctx context.Context) error {
	b.closeOnce.Do(func() {
		b.closed = true
		b.closeErr = b.browser.Context(ctx).Close()
		if b.connectionCancel != nil {
			b.connectionCancel()
		}
		b.cancel()
		if b.launcher != nil {
			b.launcher.Kill()
			if b.temporaryProfile {
				b.launcher.Cleanup()
			}
		}
	})
	return b.closeErr
}
