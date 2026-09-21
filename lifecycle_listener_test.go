package drissionpage

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBoundedQueue(t *testing.T) {
	q := newEventQueue[int]()
	q.setLimit(2)
	q.push(1)
	q.push(2)
	q.push(3)
	if s := q.stats(); s.Pending != 2 || s.Dropped != 1 {
		t.Fatal(s)
	}
	if v, e := q.pop(context.Background()); v != 2 || e != nil {
		t.Fatal(v, e)
	}
	q.close()
	if v, e := q.pop(context.Background()); v != 3 || e != nil {
		t.Fatal(v, e)
	}
	if _, e := q.pop(context.Background()); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}

func TestFrameRebindingAndFileDrop(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/frame" {
			fmt.Fprint(w, `<p id="value">frame-ready</p>`)
			return
		}
		fmt.Fprint(w, `<iframe id="frame" src="/frame"></iframe><div id="drop" style="width:200px;height:100px">drop here</div><p id="out"></p><script>const d=document.getElementById('drop');d.ondragover=e=>e.preventDefault();d.ondrop=e=>{e.preventDefault();document.getElementById('out').textContent=Array.from(e.dataTransfer.files,f=>f.name).join(',')}</script>`)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b, e := NewChromium(ctx, NewChromiumOptions().SetBrowserPath(path).SetHeadless(true).SetArgument("--site-per-process"))
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	tab, e := b.NewTab(ctx, server.URL)
	if e != nil {
		t.Fatal(e)
	}
	frame, e := tab.GetFrame(ctx, "css:#frame")
	if e != nil {
		t.Fatal(e)
	}
	initial, e := frame.Resolve(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for i, url := range []string{strings.Replace(server.URL, "127.0.0.1", "localhost", 1) + "/frame", server.URL + "/frame"} {
		if e = frame.SetAttr(ctx, "src", url); e != nil {
			t.Fatal(e)
		}
		e = WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
			current, err := frame.Resolve(ctx)
			if err != nil {
				return false, nil
			}
			actual, err := current.URL(ctx)
			if err != nil || actual != url {
				return false, nil
			}
			if (current.page.SessionID != initial.page.SessionID) != (i == 0) {
				return false, nil
			}
			element, err := frame.Ele(ctx, "css:#value")
			if err != nil {
				return false, nil
			}
			text, err := element.Text(ctx)
			return text == "frame-ready", err
		})
		if e != nil {
			t.Fatalf("frame transition %d: %v", i, e)
		}
	}
	file := filepath.Join(t.TempDir(), "drop.txt")
	if e = os.WriteFile(file, []byte("file-drop"), 0600); e != nil {
		t.Fatal(e)
	}
	zone, e := tab.Ele(ctx, "css:#drop")
	if e != nil {
		t.Fatal(e)
	}
	if e = zone.DropFiles(ctx, file); e != nil {
		t.Fatal(e)
	}
	if e = tab.WaitJS(ctx, `()=>document.getElementById('out').textContent==='drop.txt'`); e != nil {
		t.Fatal(e)
	}
}

func TestRedirectExtraOrdering(t *testing.T) {
	c := &extraChain{}
	c.requests = append(c.requests, &proto.NetworkRequestWillBeSentExtraInfo{RequestID: "redirect"})
	c.responses = append(c.responses, &proto.NetworkResponseReceivedExtraInfo{StatusCode: 302})
	first := c.add()
	c.expect(true)
	cached := c.add()
	c.expect(false)
	last := c.add()
	c.expect(true)
	c.requests = append(c.requests, &proto.NetworkRequestWillBeSentExtraInfo{RequestID: "last"})
	c.responses = append(c.responses, &proto.NetworkResponseReceivedExtraInfo{StatusCode: 200})
	c.ended = true
	c.drain()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	a, e := first.Response(ctx)
	if e != nil || a.StatusCode != 302 {
		t.Fatal(a, e)
	}
	z, e := last.Response(ctx)
	if e != nil || z.StatusCode != 200 {
		t.Fatal(z, e)
	}
	if _, e = cached.Request(ctx); !errors.Is(e, ErrNoExtraInfo) {
		t.Fatal(e)
	}
	if !c.complete() {
		t.Fatal("chain retained")
	}
}

func TestReconnectAndRedirect(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/first" {
			w.Header().Set("X-Hop", "first")
			http.Redirect(w, r, "/last", 302)
			return
		}
		w.Header().Set("X-Hop", "last")
		fmt.Fprint(w, "<title>reconnected</title>")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b, e := NewChromium(ctx, NewChromiumOptions().SetBrowserPath(path).SetHeadless(true))
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	tab, e := b.NewTab(ctx, "")
	if e != nil {
		t.Fatal(e)
	}
	id := tab.ID()
	if e = b.Disconnect(); e != nil {
		t.Fatal(e)
	}
	if e = b.Reconnect(ctx); e != nil {
		t.Fatal(e)
	}
	tab, e = b.GetTab(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	l, e := tab.Listen(ctx, ListenFilter{URLs: []string{server.URL}, ResourceTypes: []string{"Document"}})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Stop()
	if e = tab.Get(ctx, server.URL+"/first"); e != nil {
		t.Fatal(e)
	}
	packets, e := l.Wait(ctx, 2)
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []int{302, 200} {
		extra, e := packets[i].Extras.Response(ctx)
		if e != nil || extra.StatusCode != want {
			t.Fatalf("hop %d: %+v %v", i, extra, e)
		}
	}
	if packets[0].Extras == packets[1].Extras {
		t.Fatal("redirect extras shared")
	}
	title, e := tab.Title(ctx)
	if e != nil || title != "reconnected" {
		t.Fatal(title, e)
	}
}
