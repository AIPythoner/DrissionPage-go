package drissionpage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

const fixture = `<!doctype html><html><head><title>迁移测试</title></head><body><main id="root"><a id="link" href="/next">你好 Go</a><input id="name" value="old"><button id="button" onclick="document.querySelector('#result').textContent=document.querySelector('#name').value">提交</button><p id="result"></p><ul><li class="item" data-n="1">one</li><li class="item" data-n="2">two</li></ul><select id="select"><option value="a">A</option><option value="b">B</option></select><div id="shadow"></div><iframe srcdoc="<p id='inside'>frame</p>"></iframe></main><script>document.querySelector('#shadow').attachShadow({mode:'open'}).innerHTML='<b>shadow text</b>'</script></body></html>`

func TestStaticLocators(t *testing.T) {
	doc, e := MakeSessionElement(fixture, "https://example.com/base")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		loc  any
		want string
	}{{"#link", "你好 Go"}, {"css:li[data-n='2']", "two"}, {"xpath://li[1]", "one"}, {"@id=link", "你好 Go"}, {"tag:li@@class=item@@data-n=2", "two"}, {"text^你好", "你好 Go"}, {"text=one", "one"}, {By("id", "link"), "你好 Go"}} {
		t.Run(fmt.Sprint(tc.loc), func(t *testing.T) {
			el, e := doc.Ele(tc.loc)
			if e != nil {
				t.Fatal(e)
			}
			if el.Text() != tc.want {
				t.Fatalf("%q != %q", el.Text(), tc.want)
			}
		})
	}
	el, e := doc.Ele("tag:li", -1)
	if e != nil || el.Text() != "two" {
		t.Fatalf("negative index: %v", e)
	}
	link, _ := doc.Ele("#link")
	if href, _ := link.Attr("href"); href != "https://example.com/next" {
		t.Fatal(href)
	}
	if _, e := doc.Ele("tag:li", 0); !errors.Is(e, ErrInvalidIndex) {
		t.Fatal(e)
	}
	if _, e := ParseLocator("@@id=x@|class=y"); !errors.Is(e, ErrInvalidLocator) {
		t.Fatal(e)
	}
	quoted := `a'"b`
	qdoc, _ := MakeSessionElement(`<p>a'"b</p>`)
	if _, e = qdoc.Ele("text=" + quoted); e != nil {
		t.Fatal(e)
	}
}
func TestSession(t *testing.T) {
	var tries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "ok", Path: "/"})
			fmt.Fprint(w, fixture)
		case "/cookie":
			c, e := r.Cookie("sid")
			if e != nil || c.Value != "ok" {
				w.WriteHeader(401)
			} else {
				fmt.Fprint(w, `{"ok":true}`)
			}
		case "/retry":
			if tries.Add(1) < 2 {
				w.WriteHeader(503)
			} else {
				fmt.Fprint(w, "ok")
			}
		case "/echo":
			fmt.Fprint(w, "echo")
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	o := NewSessionOptions()
	o.RetryTimes = 1
	o.RetryInterval = time.Millisecond
	p, e := NewSessionPage(o)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	ctx := context.Background()
	if _, e = p.Get(ctx, server.URL); e != nil {
		t.Fatal(e)
	}
	if title, e := p.Title(); e != nil || title != "迁移测试" {
		t.Fatalf("%s %v", title, e)
	}
	if _, e = p.Get(ctx, server.URL+"/cookie"); e != nil {
		t.Fatal(e)
	}
	var data map[string]bool
	if e = p.JSON(&data); e != nil || !data["ok"] {
		t.Fatal(e)
	}
	if _, e = p.Get(ctx, server.URL+"/retry"); e != nil || tries.Load() != 2 {
		t.Fatalf("retry: %v", e)
	}
	if _, e = p.Get(ctx, server.URL+"/missing"); e == nil {
		t.Fatal("missing HTTP error")
	}
	p.Close()
	if _, e = p.Get(ctx, server.URL); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}
func TestBrowser(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER for real browser tests")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, fixture) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	o := NewChromiumOptions().SetBrowserPath(path).SetHeadless(true)
	o.Timeout = 5 * time.Second
	browser, e := NewChromium(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	defer browser.Close()
	tab, e := browser.NewTab(ctx, server.URL)
	if e != nil {
		t.Fatal(e)
	}
	if title, e := tab.Title(ctx); e != nil || title != "迁移测试" {
		t.Fatalf("title %s %v", title, e)
	}
	el, e := tab.Ele(ctx, "@id=name")
	if e != nil {
		t.Fatal(e)
	}
	if e = el.Input(ctx, "Go 自动化"); e != nil {
		t.Fatal(e)
	}
	button, e := tab.Ele(ctx, "css:#button")
	if e != nil {
		t.Fatal(e)
	}
	if e = button.Click(ctx); e != nil {
		t.Fatal(e)
	}
	result, e := tab.Ele(ctx, "@id=result")
	if e != nil {
		t.Fatal(e)
	}
	if text, e := result.Text(ctx); e != nil || text != "Go 自动化" {
		t.Fatalf("input/click %s %v", text, e)
	}
	frameEl, e := tab.Ele(ctx, "tag:iframe")
	if e != nil {
		t.Fatal(e)
	}
	frame, e := frameEl.Frame(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = frame.Ele(ctx, "@id=inside"); e != nil {
		t.Fatal(e)
	}
	shadowEl, e := tab.Ele(ctx, "@id=shadow")
	if e != nil {
		t.Fatal(e)
	}
	shadow, e := shadowEl.ShadowRoot(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = shadow.Ele(ctx, "css:b"); e != nil {
		t.Fatal(e)
	}
	if _, e = tab.RunJS(ctx, `()=>{const host=document.createElement('div');host.id='closed-shadow';document.body.append(host);host.attachShadow({mode:'closed'}).innerHTML='<b>closed content</b>'}`); e != nil {
		t.Fatal(e)
	}
	closedHost, e := tab.Ele(ctx, "@id=closed-shadow")
	if e != nil {
		t.Fatal(e)
	}
	closedRoot, e := closedHost.ShadowRoot(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = closedRoot.Ele(ctx, "css:b"); e != nil {
		t.Fatal(e)
	}
	if png, e := tab.Screenshot(ctx, "", false); e != nil || len(png) < 8 {
		t.Fatalf("screenshot %v", e)
	}
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	if _, e = tab.Ele(short, "@id=missing"); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("timeout %v", e)
	}
}
