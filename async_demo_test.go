package drissionpage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The HTML is copied byte-for-byte from DrissionPage-async/tests/demo/site.
// This server implements the endpoints used below; it does not run Python.
func asyncDemoServer() *httptest.Server {
	files := http.FileServer(http.Dir("testdata/async-demo/site"))
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/echo":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			query := map[string]string{}
			for k := range r.URL.Query() {
				query[k] = r.URL.Query().Get(k)
			}
			raw, _ := io.ReadAll(r.Body)
			var body any
			if json.Unmarshal(raw, &body) != nil {
				body = string(raw)
			}
			json.NewEncoder(w).Encode(map[string]any{"method": r.Method, "query": query, "body": body, "cookies": r.Header.Get("Cookie"), "ua": r.UserAgent(), "headers": r.Header})
		case "/api/slow":
			select {
			case <-time.After(1200 * time.Millisecond):
				fmt.Fprint(w, "slow-ok")
			case <-r.Context().Done():
			}
		case "/api/redirect":
			http.Redirect(w, r, "/api/echo?redirected=1", 302)
		case "/api/setcookie":
			http.SetCookie(w, &http.Cookie{Name: "session_cookie", Value: r.URL.Query().Get("v"), Path: "/"})
			fmt.Fprint(w, "cookie-set")
		case "/download/sample.txt":
			w.Header().Set("Content-Disposition", `attachment; filename="sample.txt"`)
			w.Header().Set("Content-Type", "application/octet-stream")
			fmt.Fprint(w, strings.Repeat("DrissionPageAsync 下载测试内容\n", 200))
		case "/download/big.bin":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(make([]byte, 512*1024))
		case "/img/dot.png":
			w.Header().Set("Content-Type", "image/png")
			data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")
			w.Write(data)
		default:
			if r.URL.Path == "/" || r.URL.Path == "/index.html" {
				http.SetCookie(w, &http.Cookie{Name: "dpa_cookie", Value: "cookie_value", Path: "/"})
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				data, err := os.ReadFile("testdata/async-demo/site/index.html")
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				w.Write(data)
				return
			}
			files.ServeHTTP(w, r)
		}
	}))
}

type demoCase struct {
	t    *testing.T
	ctx  context.Context
	tab  *ChromiumTab
	base string
}

func (d demoCase) ok(err error) {
	d.t.Helper()
	if err != nil {
		d.t.Fatal(err)
	}
}
func (d demoCase) ele(id string) *ChromiumElement {
	d.t.Helper()
	e, err := d.tab.Ele(d.ctx, "css:#"+id)
	d.ok(err)
	return e
}
func (d demoCase) click(id string) { d.t.Helper(); d.ok(d.ele(id).Click(d.ctx)) }
func (d demoCase) text(id, want string) {
	d.t.Helper()
	d.ok(d.tab.WaitJS(d.ctx, `(id,want)=>document.getElementById(id)!==null&&document.getElementById(id).textContent===want`, id, want))
}
func (d demoCase) js(script string, args ...any) json.RawMessage {
	d.t.Helper()
	v, err := d.tab.RunJS(d.ctx, script, args...)
	d.ok(err)
	return v
}
func (d demoCase) truth(script string, args ...any) {
	d.t.Helper()
	v := d.js(script, args...)
	if string(v) != "true" {
		d.t.Fatalf("condition failed: %s -> %s", script, v)
	}
}

func TestAsyncHTMLDemo(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER for original HTML demo")
	}
	server := asyncDemoServer()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	options := NewChromiumOptions().SetBrowserPath(path).SetHeadless(os.Getenv("DRISSIONPAGE_HEADFUL") != "1")
	options.Timeout = 4 * time.Second
	b, err := NewChromium(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	run := func(name string, fn func(demoCase)) {
		t.Run(name, func(t *testing.T) {
			caseCtx, caseCancel := context.WithTimeout(ctx, 25*time.Second)
			defer caseCancel()
			tab, err := b.NewTab(caseCtx, server.URL+"/index.html")
			if err != nil {
				t.Fatal(err)
			}
			defer tab.Close(ctx)
			d := demoCase{t, caseCtx, tab, server.URL}
			d.ok(tab.SetViewport(ctx, 1100, 800))
			fn(d)
		})
	}

	run("01_locators_attributes", func(d demoCase) {
		d.text("title", "你好，DrissionPage")
		for _, loc := range []any{CSS(".box"), XPath("//div[contains(concat(' ',normalize-space(@class),' '),' box ')]"), "tag:div@@data-role@@data-idx"} {
			els, e := d.tab.Eles(d.ctx, loc)
			d.ok(e)
			if len(els) != 3 {
				d.t.Fatalf("locator %v: %d", loc, len(els))
			}
		}
		attr, ok, e := d.ele("attr-ele").Attr(d.ctx, "data-a")
		d.ok(e)
		if !ok || attr != "va" {
			d.t.Fatal(attr, ok)
		}
		d.ok(d.ele("attr-ele").SetAttr(d.ctx, "data-a", "changed"))
		d.truth(`()=>document.querySelector('#attr-ele').dataset.a==='changed'`)
		visible, e := d.ele("hidden-box").Visible(d.ctx)
		d.ok(e)
		if visible {
			d.t.Fatal("hidden element visible")
		}
		pseudo, e := d.ele("pseudo-ele").Pseudo(d.ctx, "before", "content")
		d.ok(e)
		if !strings.Contains(pseudo, "PRE-") {
			d.t.Fatal(pseudo)
		}
	})
	run("02_forms_select_upload", func(d demoCase) {
		for id, value := range map[string]string{"in-text": "世界", "in-pass": "secret", "in-num": "42", "in-area": "第一行\n第二行", "in-echo": "输入事件"} {
			d.ok(d.ele(id).Input(d.ctx, value))
			d.truth(`(id,v)=>document.getElementById(id).value===v`, id, value)
		}
		d.text("echo-out", "输入事件")
		d.ok(d.ele("chk1").Check(d.ctx, true))
		d.ok(d.ele("chk2").Check(d.ctx, false))
		d.click("rd2")
		d.ok(d.ele("sel-single").SelectByValue(d.ctx, "b"))
		d.ok(d.ele("sel-multi").SelectByValue(d.ctx, "m1", "m3"))
		d.click("btn-submit")
		d.truth(`()=>{const f=JSON.parse(document.getElementById('form-out').textContent);return f.text==='世界'&&f.chk1&&!f.chk2&&f.rd==='R2'&&f.single==='b'&&f.multi.join(',')==='m1,m3'}`)
		d.ok(d.ele("sel-multi").InvertSelection(d.ctx))
		d.truth(`()=>Array.from(document.getElementById('sel-multi').selectedOptions,o=>o.value).join(',')==='m2,m4'`)
		d.ok(d.ele("sel-multi").ClearSelection(d.ctx))
		d.truth(`()=>document.getElementById('sel-multi').selectedOptions.length===0`)
		first, _ := filepath.Abs("testdata/async-demo/files/upload1.txt")
		second, _ := filepath.Abs("testdata/async-demo/files/upload2.txt")
		d.ok(d.ele("in-file").SetFiles(d.ctx, first))
		d.text("file-out", "upload1.txt")
		d.ok(d.ele("btn-file-trigger").ClickToUpload(d.ctx, first, second))
		d.text("file-out", "upload1.txt|upload2.txt")
	})
	run("03_click_events", func(d demoCase) {
		d.click("btn-click")
		d.click("btn-click")
		d.text("click-count", "2")
		d.ok(d.ele("btn-dbl").DoubleClick(d.ctx))
		d.text("dbl-out", "dblclick")
		d.ok(d.ele("btn-right").RightClick(d.ctx))
		d.text("right-out", "contextmenu")
		d.ok(d.ele("btn-middle").ClickAt(d.ctx, 10, 10, "middle", 1))
		d.text("middle-out", "middle")
		d.ok(d.ele("btn-hover").Hover(d.ctx))
		d.text("hover-out", "hovered")
		d.click("btn-delay")
		d.text("delay-out", "done")
		d.ok(d.ele("covered-btn").ClickJS(d.ctx))
		d.text("covered-out", "clicked")
		d.click("btn-uncover")
		d.click("covered-btn")
	})
	run("04_drag_mouse_wheel", func(d demoCase) {
		d.ok(d.ele("drag-block").DragTo(d.ctx, d.ele("drop-zone"), 200*time.Millisecond))
		d.text("drop-out", "dropped")
		d.ok(d.tab.Actions(d.ctx).On(d.ele("drag-area")).Scroll(0, 100).Do())
		d.ok(d.tab.WaitJS(d.ctx, `()=>Number(document.getElementById('wheel-out').textContent)>0`))
		d.truth(`()=>document.getElementById('mouse-out').textContent.includes(',')`)
	})
	run("05_keyboard_actions", func(d demoCase) {
		e := d.ele("key-input")
		d.ok(e.Focus(d.ctx))
		d.ok(e.Type(d.ctx, input.KeyA, input.Backspace, input.ArrowLeft))
		d.truth(`()=>document.getElementById('key-log').textContent.includes('Backspace')`)
		d.ok(d.tab.Actions(d.ctx).KeyDown(input.ControlLeft).Type(input.KeyB).KeyUp(input.ControlLeft).Do())
		d.text("combo-out", "ctrl+b")
		d.ok(e.Input(d.ctx, "abc"))
		d.truth(`()=>document.getElementById('key-input').value==='abc'`)
	})
	run("06_dynamic_waits", func(d demoCase) {
		d.click("btn-spawn")
		d.text("spawned", "我出现了")
		d.click("btn-remove")
		d.ok(d.tab.WaitDeleted(d.ctx, "css:#to-remove"))
		hide := d.ele("to-hide")
		d.click("btn-hide")
		d.ok(hide.WaitHidden(d.ctx))
		enable := d.ele("to-enable")
		d.click("btn-enable")
		d.ok(enable.WaitState(d.ctx, "enabled", true))
		d.click("btn-move")
		d.ok(d.ele("moving-box").WaitStable(d.ctx, 200*time.Millisecond))
	})
	run("07_scroll", func(d demoCase) {
		d.ok(d.tab.ScrollToBottom(d.ctx))
		d.ok(d.tab.WaitJS(d.ctx, `()=>scrollY>0`))
		d.ok(d.tab.ScrollTo(d.ctx, 0, 0))
		d.truth(`()=>scrollY===0`)
		d.ok(d.ele("deep-ele").ScrollIntoView(d.ctx))
		d.truth(`()=>document.getElementById('scroll-box').scrollTop>0`)
		d.ok(d.ele("scroll-box").ScrollTo(d.ctx, 0, 0))
		d.truth(`()=>document.getElementById('scroll-box').scrollTop===0`)
	})
	run("08_frames_nested_srcdoc", func(d demoCase) {
		f, e := d.tab.GetFrame(d.ctx, "css:#frm-main")
		d.ok(e)
		fi, e := f.Ele(d.ctx, "css:#f-input")
		d.ok(e)
		d.ok(fi.Input(d.ctx, "Go-frame"))
		fb, e := f.Ele(d.ctx, "css:#f-btn")
		d.ok(e)
		d.ok(fb.Click(d.ctx))
		d.ok(f.WaitJS(d.ctx, `()=>document.getElementById('f-out').textContent==='f-clicked:Go-frame'`))
		nested, e := f.GetFrame(d.ctx, "css:#f-nested")
		d.ok(e)
		nb, e := nested.Ele(d.ctx, "css:#n-btn")
		d.ok(e)
		d.ok(nb.Click(d.ctx))
		d.ok(nested.WaitJS(d.ctx, `()=>document.getElementById('n-out').textContent==='n-clicked'`))
		sd, e := d.tab.GetFrame(d.ctx, "css:#frm-srcdoc")
		d.ok(e)
		p, e := sd.Ele(d.ctx, "css:#sd-text")
		d.ok(e)
		text, e := p.Text(d.ctx)
		d.ok(e)
		if text != "srcdoc 内部文本" {
			d.t.Fatal(text)
		}
	})
	run("09_shadow", func(d demoCase) {
		root, e := d.ele("shadow-host").ShadowRoot(d.ctx)
		d.ok(e)
		field, e := root.Ele(d.ctx, "css:#sr-input")
		d.ok(e)
		d.ok(field.Input(d.ctx, "Go-shadow"))
		button, e := root.Ele(d.ctx, "css:#sr-btn")
		d.ok(e)
		d.ok(button.Click(d.ctx))
		d.text("sr-report", "sr-clicked:Go-shadow")
	})
	run("10_dialogs", func(d demoCase) {
		for _, c := range []struct {
			id           string
			accept       bool
			prompt, want string
		}{{"btn-alert", true, "", "alert-done"}, {"btn-confirm", true, "", "confirm:true"}, {"btn-confirm", false, "", "confirm:false"}, {"btn-prompt", true, "Go", "prompt:Go"}} {
			stop, e := d.tab.AutoHandleAlerts(d.ctx, c.accept, c.prompt)
			d.ok(e)
			d.click(c.id)
			d.text("alert-out", c.want)
			stop()
		}
	})
	run("11_network", func(d demoCase) {
		l, e := d.tab.Listen(d.ctx, ListenFilter{URLs: []string{"/api/"}})
		d.ok(e)
		defer l.Stop()
		for _, c := range []struct{ id, want, method string }{{"btn-fetch", "fetch:fetch", "GET"}, {"btn-post", "post:7", "POST"}, {"btn-xhr", "xhr:xhr", "GET"}, {"btn-slow", "slow-done", "GET"}} {
			d.click(c.id)
			d.text("net-out", c.want)
			packet, e := l.Next(d.ctx)
			d.ok(e)
			if packet.Method != c.method || len(packet.Body) == 0 || packet.Response.Status != 200 {
				d.t.Fatalf("packet: %+v", packet)
			}
		}
		d.ok(l.WaitSilent(d.ctx, 100*time.Millisecond))
	})
	run("12_navigation", func(d demoCase) {
		d.click("btn-push")
		d.ok(d.tab.WaitURL(d.ctx, "pushed=1", false))
		d.click("btn-title")
		d.ok(d.tab.WaitTitle(d.ctx, "DPA 标题已改", false))
		newtab, e := d.ele("btn-open").ClickForNewTab(d.ctx)
		d.ok(e)
		defer newtab.Close(d.ctx)
		d.ok(newtab.WaitTitle(d.ctx, "第二页", false))
		d.click("link")
		d.ok(d.tab.WaitTitle(d.ctx, "第二页", false))
		d.ok(d.tab.Back(d.ctx))
		d.ok(d.tab.WaitURL(d.ctx, "index.html", false))
	})
	run("13_download", func(d demoCase) {
		manager, e := b.Downloads(d.ctx, d.t.TempDir())
		d.ok(e)
		defer manager.Close()
		m, e := d.ele("link-download").ClickToDownload(d.ctx, manager)
		d.ok(e)
		path, e := manager.Finalize(d.ctx, m.GUID, "demo.txt", FileError)
		d.ok(e)
		data, e := os.ReadFile(path)
		d.ok(e)
		if !strings.Contains(string(data), "下载测试内容") {
			d.t.Fatal("download body")
		}
	})
	run("14_console", func(d demoCase) {
		c, e := d.tab.Console(d.ctx)
		d.ok(e)
		defer c.Stop()
		for _, entry := range []struct{ id, kind, mark string }{{"btn-log", "log", "DPA-CONSOLE-TEST"}, {"btn-cerr", "error", "DPA-CONSOLE-ERROR"}, {"btn-warn", "warning", "DPA-CONSOLE-WARN"}} {
			d.click(entry.id)
			for {
				m, e := c.Next(d.ctx)
				d.ok(e)
				if len(m.Args) == 0 || !strings.Contains(string(m.Args[0]), entry.mark) {
					continue
				}
				if m.Type != entry.kind {
					d.t.Fatal(m.Type)
				}
				if entry.id == "btn-cerr" && (len(m.Args) != 2 || string(m.Args[1]) != "{\"a\":1}") {
					d.t.Fatalf("object: %s", m.Args)
				}
				break
			}
		}
		event := make(chan struct{}, 1)
		life, stop := context.WithCancel(d.ctx)
		defer stop()
		wait := d.tab.page.Context(life).EachEvent(func(e *proto.RuntimeExceptionThrown) {
			if strings.Contains(e.ExceptionDetails.Exception.Description, "DPA-ERROR-TEST") {
				event <- struct{}{}
			}
		})
		go wait()
		d.click("btn-err")
		select {
		case <-event:
		case <-time.After(3 * time.Second):
			d.t.Fatal("exception event missing")
		}
	})
	run("15_relations_geometry", func(d demoCase) {
		row := d.ele("row2")
		children, e := row.Children(d.ctx)
		d.ok(e)
		if len(children) != 3 {
			d.t.Fatal(len(children))
		}
		prev, e := row.Prev(d.ctx)
		d.ok(e)
		id, _, e := prev.Attr(d.ctx, "id")
		d.ok(e)
		if id != "row1" {
			d.t.Fatal(id)
		}
		center := d.ele("d-center")
		d.ok(center.ScrollIntoView(d.ctx))
		east, e := center.Neighbor(d.ctx, "east", CSS("#d-east"), 1)
		d.ok(e)
		text, e := east.Text(d.ctx)
		d.ok(e)
		if text != "东" {
			d.t.Fatal(text)
		}
		rect, e := center.Rect(d.ctx)
		d.ok(e)
		if rect.Width <= 0 || rect.Height <= 0 {
			d.t.Fatal(rect)
		}
	})
	run("16_storage_image_screenshot", func(d demoCase) {
		d.click("btn-storage")
		d.text("storage-out", "stored")
		local, e := d.tab.Storage(d.ctx, false)
		d.ok(e)
		session, e := d.tab.Storage(d.ctx, true)
		d.ok(e)
		if local["dpa_local"] != "L-VALUE" || session["dpa_session"] != "S-VALUE" {
			d.t.Fatal(local, session)
		}
		image, e := d.ele("img1").Src(d.ctx)
		d.ok(e)
		if len(image) < 8 || string(image[1:4]) != "PNG" {
			d.t.Fatal("image content")
		}
		shot, e := d.tab.Screenshot(d.ctx, "", false)
		d.ok(e)
		if len(shot) < 100 {
			d.t.Fatal("empty screenshot")
		}
		d.ok(d.tab.SaveMHTML(d.ctx, filepath.Join(d.t.TempDir(), "demo.mhtml")))
	})
	run("17_nested_lookup", func(d demoCase) {
		outer := d.ele("chain-outer")
		mid, e := outer.Ele(d.ctx, "css:#chain-mid")
		d.ok(e)
		button, e := mid.Ele(d.ctx, "css:#chain-btn")
		d.ok(e)
		d.ok(button.Click(d.ctx))
		d.text("chain-out", "1")
		items, e := mid.Eles(d.ctx, "css:.chain-item")
		d.ok(e)
		if len(items) != 2 {
			d.t.Fatal(len(items))
		}
		parent, e := button.Parent(d.ctx)
		d.ok(e)
		id, _, e := parent.Attr(d.ctx, "id")
		d.ok(e)
		if id != "chain-mid" {
			d.t.Fatal(id)
		}
	})
	run("18_session_hybrid_concurrency", func(d demoCase) {
		s, e := d.tab.ToSession(d.ctx, true)
		d.ok(e)
		defer s.Close()
		title, e := s.Title()
		d.ok(e)
		if title != "DPA 功能测试台" {
			d.t.Fatal(title)
		}
		d.ok(s.SetUserAgent("Go-demo"))
		r, e := s.Get(d.ctx, server.URL+"/api/echo")
		d.ok(e)
		var data map[string]any
		d.ok(r.JSON(&data))
		if data["ua"] != "Go-demo" || !strings.Contains(data["cookies"].(string), "dpa_cookie") {
			d.t.Fatal(data)
		}
		h, e := d.tab.Hybrid(d.ctx)
		d.ok(e)
		d.ok(h.ChangeMode(d.ctx, "s", true, true))
		d.ok(h.ChangeMode(d.ctx, "d", true, true))
		var wg sync.WaitGroup
		errs := make(chan error, 20)
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, e := d.tab.RunJS(d.ctx, `()=>1`)
				if e == nil && string(v) != "1" {
					e = fmt.Errorf("unexpected result %s", v)
				}
				errs <- e
			}()
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			d.ok(e)
		}
	})
}
