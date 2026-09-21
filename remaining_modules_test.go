package drissionpage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "stream" {
			t.Error("stream headers missing")
		}
		fmt.Fprint(w, "first\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	page, err := NewSessionPage(NewSessionOptions().SetHeader("X-Test", "stream"))
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := page.OpenStream(ctx, "GET", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	chunk := make([]byte, 6)
	if _, err = io.ReadFull(response.Body, chunk); err != nil || string(chunk) != "first\n" {
		t.Fatalf("%q %v", chunk, err)
	}
	cancel()
	if _, err = io.ReadAll(response.Body); err == nil {
		t.Fatal("stream did not cancel")
	}
	if _, err = page.Response(); !errors.Is(err, ErrNoResponse) {
		t.Fatal("stream replaced parsed document", err)
	}
}

func TestFramesHandlesSelectorsAndCapacity(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/slow":
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/frame":
			fmt.Fprint(w, `<!doctype html><div id="inside" style="position:absolute;left:10px;top:20px;width:30px;height:40px">frame</div>`)
		default:
			fmt.Fprint(w, `<!doctype html><iframe id="frame" style="position:absolute;left:100px;top:80px;width:300px;height:200px;border:4px solid" src="/frame"></iframe><select id="multi" multiple><option value="a">Alpha</option><option value="b">Beta</option><option value="c">Gamma</option></select><select id="single"><option>A</option><option>B</option></select>`)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	b, err := NewChromium(ctx, NewChromiumOptions().SetBrowserPath(path).SetHeadless(true).SetArgument("--site-per-process"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	tab, err := b.NewTab(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := tab.GetFrame(ctx, "#frame")
	if err != nil {
		t.Fatal(err)
	}
	inside, err := frame.Ele(ctx, "#inside")
	if err != nil {
		t.Fatal(err)
	}
	rect, err := inside.RootRect(ctx)
	if err != nil || rect.X != 114 || rect.Y != 104 || rect.Width != 30 {
		t.Fatalf("root rect %+v %v", rect, err)
	}
	other := strings.Replace(server.URL, "127.0.0.1", "localhost", 1) + "/frame"
	if err = frame.SetAttr(ctx, "src", other); err != nil {
		t.Fatal(err)
	}
	if err = WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) { url, err := frame.URL(ctx); return url == other, err }); err != nil {
		t.Fatal(err)
	}
	if err = frame.SetStorage(ctx, false, "frame-key", stringPointer("stored")); err != nil {
		t.Fatal(err)
	}
	storage, err := frame.Storage(ctx, false)
	if err != nil || storage["frame-key"] != "stored" {
		t.Fatalf("frame storage %+v %v", storage, err)
	}
	inside, err = frame.Ele(ctx, "#inside")
	if err != nil {
		t.Fatal(err)
	}
	rect, err = inside.RootRect(ctx)
	if err != nil || rect.X != 114 || rect.Y != 104 {
		t.Fatalf("oopif rect %+v %v", rect, err)
	}
	hit, err := tab.ElementAt(ctx, 125, 115)
	if err != nil {
		t.Fatal(err)
	}
	hitID, _, err := hit.Attr(ctx, "id")
	if err != nil || hitID != "inside" {
		t.Fatalf("frame hit %s %v", hitID, err)
	}
	handle, err := tab.EvalHandle(ctx, `return document.querySelector('#multi');`)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Release(ctx)
	multi, err := handle.Element(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := tab.RunJS(ctx, `function(el){return el.id}`, multi)
	if err != nil || string(result) != `"multi"` {
		t.Fatalf("argument %s %v", result, err)
	}
	if err = multi.SelectByValue(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err = multi.SelectByText(ctx, "Beta"); err != nil {
		t.Fatal(err)
	}
	selected, err := multi.SelectedOptions(ctx)
	if err != nil || len(selected) != 2 {
		t.Fatalf("additive select %+v %v", selected, err)
	}
	if err = multi.CancelByIndex(ctx, 1); err != nil {
		t.Fatal(err)
	}
	selected, err = multi.SelectedOptions(ctx)
	if err != nil || len(selected) != 1 || selected[0].Value != "b" {
		t.Fatalf("cancel %+v %v", selected, err)
	}
	single, err := tab.Ele(ctx, "#single")
	if err != nil {
		t.Fatal(err)
	}
	if err = single.ClearSelection(ctx); err == nil {
		t.Fatal("cleared a single select")
	}
	observer, err := tab.WatchState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Stop()
	if _, err = tab.RunJS(ctx, `()=>{setTimeout(()=>alert('observed'),40);return true}`); err != nil {
		t.Fatal(err)
	}
	if err = WaitUntil(ctx, 10*time.Millisecond, func() (bool, error) { return observer.Snapshot().HasAlert, nil }); err != nil {
		t.Fatal(err)
	}
	if state := observer.Snapshot(); state.AlertMessage != "observed" {
		t.Fatal(state)
	}
	if err = tab.HandleAlert(ctx, true, ""); err != nil {
		t.Fatal(err)
	}
	if err = WaitUntil(ctx, 10*time.Millisecond, func() (bool, error) { return !observer.Snapshot().HasAlert, nil }); err != nil {
		t.Fatal(err)
	}
	dynamic, err := tab.Listen(ctx, ListenFilter{URLs: []string{"/missing"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = dynamic.SetURLs(false, "/frame"); err != nil {
		t.Fatal(err)
	}
	if err = dynamic.SetMethods("POST"); err != nil {
		t.Fatal(err)
	}
	if _, err = tab.RunJS(ctx, `()=>fetch('/frame',{method:'POST'}).then(r=>r.text())`); err != nil {
		t.Fatal(err)
	}
	packet, err := dynamic.Next(ctx)
	if err != nil || packet.Method != "POST" {
		t.Fatalf("dynamic filter %+v %v", packet, err)
	}
	dynamic.Stop()
	listener, err := tab.Listen(ctx, ListenFilter{MaxActiveRequests: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()
	if _, err = tab.RunJS(ctx, `()=>{for(let i=0;i<5;i++)fetch('/slow?i='+i);return true}`); err != nil {
		t.Fatal(err)
	}
	if _, err = listener.Next(ctx); !errors.Is(err, ErrListenerCapacity) {
		t.Fatalf("capacity %v", err)
	}
}
func stringPointer(value string) *string { return &value }
