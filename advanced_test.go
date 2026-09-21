package drissionpage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestINI(t *testing.T) {
	m, e := ReadINI(strings.NewReader("[chromium_options]\nbrowser_path=chrome\narguments=['--headless', '--user-data-dir=C:\\\\tmp']\nprefs={'a': True, 'b': {'c': 2}}\n[timeouts]\nbase=4.5\n[session_options]\nheaders={'User-Agent': 'Go测试'}\n[others]\nretry_times=2\n"))
	if e != nil {
		t.Fatal(e)
	}
	o, e := m.ChromiumOptions()
	if e != nil {
		t.Fatal(e)
	}
	if !o.Headless || o.Timeout != 4500*time.Millisecond {
		t.Fatal(o)
	}
	s, e := m.SessionOptions()
	if e != nil || s.Headers.Get("User-Agent") != "Go测试" {
		t.Fatalf("%+v %v", s, e)
	}
	path := filepath.Join(t.TempDir(), "config.ini")
	if e = m.Save(path); e != nil {
		t.Fatal(e)
	}
	again, e := LoadINI(path)
	if e != nil || again.Get("chromium_options", "arguments") != m.Get("chromium_options", "arguments") {
		t.Fatal(e)
	}
	if _, e = literalJSON("__import__('os').system('bad')"); e == nil {
		t.Fatal("accepted executable expression")
	}
	m.Set("chromium_options", "extensions", "['ext-a']")
	m.Set("chromium_options", "user", "Profile 2")
	m.Set("timeouts", "page_load", "12")
	m.Set("timeouts", "script", "3.5")
	m.Set("others", "retry_interval", "0.25")
	o, e = m.ChromiumOptions()
	if e != nil || o.PageLoadTimeout != 12*time.Second || o.ScriptTimeout != 3500*time.Millisecond || o.RetryTimes != 2 || o.RetryInterval != 250*time.Millisecond || len(o.Extensions) != 1 || !strings.Contains(strings.Join(o.Arguments, " "), "--profile-directory=Profile 2") {
		t.Fatalf("INI fields: %+v %v", o, e)
	}
	m.Set("timeouts", "script", "NaN")
	if _, e = m.ChromiumOptions(); e == nil {
		t.Fatal("NaN timeout accepted")
	}
}
func TestStreamingAndLifetimes(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "shared", Value: "yes", Path: "/"})
		fmt.Fprint(w, fixture)
	})
	mux.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		websocket.Message.Send(ws, "hello-ws")
		var msg string
		websocket.Message.Receive(ws, &msg)
	}))
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: hello\ndata: hello-sse\nid: 1\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	b, e := NewChromium(ctx, NewChromiumOptions().SetBrowserPath(path).SetHeadless(true))
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	creation, stop := context.WithTimeout(ctx, 5*time.Second)
	tab, e := b.NewTab(creation, server.URL)
	stop()
	if e != nil {
		t.Fatal(e)
	}
	l, e := tab.Listen(ctx, ListenFilter{})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Stop()
	if _, e = tab.RunJS(ctx, `()=>{window.ws=new WebSocket(location.origin.replace('http','ws')+'/ws');window.ws.onmessage=()=>window.ws.send('ack')}`); e != nil {
		t.Fatal(e)
	}
	stream, e := l.NextStream(ctx)
	if e != nil || stream.Data != "hello-ws" {
		t.Fatalf("websocket %+v %v", stream, e)
	}
	if _, e = tab.RunJS(ctx, `()=>{window.es=new EventSource('/sse')}`); e != nil {
		t.Fatal(e)
	}
	for {
		stream, e = l.NextStream(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if stream.Kind == "sse" {
			break
		}
	}
	if stream.Data != "hello-sse" || stream.EventName != "hello" {
		t.Fatal(stream)
	}
	if _, e = tab.RunJS(ctx, `()=>{window.es.close();window.ws.close()}`); e != nil {
		t.Fatal(e)
	}
	session, e := tab.ToSession(ctx, true)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	cookies, e := session.Cookies(server.URL)
	if e != nil || len(cookies) == 0 {
		t.Fatalf("cookies %v %v", cookies, e)
	}
	cast, e := tab.StartScreencast(ctx, filepath.Join(t.TempDir(), "frames"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tab.RunJS(ctx, `()=>{document.body.style.background='red'}`); e != nil {
		t.Fatal(e)
	}
	deadline, c := context.WithTimeout(ctx, 5*time.Second)
	defer c()
	if e = WaitUntil(deadline, 20*time.Millisecond, func() (bool, error) { return cast.Frames() > 0, nil }); e != nil {
		t.Fatal(e)
	}
	if e = cast.Stop(ctx); e != nil {
		t.Fatal(e)
	}
	files, e := filepath.Glob(filepath.Join(cast.directory, "*.jpg"))
	if e != nil || len(files) == 0 {
		t.Fatalf("frames %v %v", files, e)
	}
	if ffmpeg := os.Getenv("DRISSIONPAGE_FFMPEG"); ffmpeg != "" {
		output := filepath.Join(t.TempDir(), "recording.mp4")
		if e = cast.Video(ctx, ffmpeg, output, 10); e != nil {
			t.Fatal(e)
		}
		info, e := os.Stat(output)
		if e != nil || info.Size() < 100 {
			t.Fatalf("video export %v", e)
		}
	}
	for _, locator := range []string{"#link", ".item", "你好 Go"} {
		if _, e = tab.Ele(ctx, locator); e != nil {
			t.Fatalf("native search %s %v", locator, e)
		}
	}
}
