package drissionpage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(500)
			return
		}
		fmt.Fprint(w, strings.Repeat("download", 10000))
	}))
	defer server.Close()
	p, e := NewSessionPage()
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	path := filepath.Join(t.TempDir(), "output.txt")
	m, e := p.Download(context.Background(), server.URL, path)
	if e != nil {
		t.Fatal(e)
	}
	progress, e := m.Wait(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if progress.Received != 80000 || progress.State != DownloadCompleted {
		t.Fatal(progress)
	}
	if _, e = p.Download(context.Background(), server.URL, path); e == nil {
		t.Fatal("existing destination accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.txt")
	m, e = p.Download(context.Background(), server.URL+"/bad", bad)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Wait(context.Background()); e == nil {
		t.Fatal("missing HTTP failure")
	}
	if _, e = os.Stat(bad); !os.IsNotExist(e) {
		t.Fatal("partial output published")
	}
}
func TestSessionRequestBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if e := r.ParseMultipartForm(1 << 20); e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, _, e := r.FormFile("file")
			if e != nil {
				t.Error(e)
				return
			}
			defer f.Close()
			io.Copy(w, f)
			return
		}
		io.Copy(w, r.Body)
	}))
	defer server.Close()
	p, e := NewSessionPage()
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	r, e := p.Post(context.Background(), server.URL, RequestOptions{JSON: map[string]string{"hello": "world"}})
	if e != nil || string(r.Body) != `{"hello":"world"}` {
		t.Fatalf("%v %v", r, e)
	}
	path := filepath.Join(t.TempDir(), "upload.txt")
	os.WriteFile(path, []byte("file-content"), 0600)
	r, e = p.Post(context.Background(), server.URL, RequestOptions{Files: map[string]string{"file": path}})
	if e != nil || string(r.Body) != "file-content" {
		t.Fatalf("%v %v", r, e)
	}
}
func TestBrowserModules(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true}`)
		case "/download":
			w.Header().Set("Content-Disposition", `attachment; filename="test.txt"`)
			fmt.Fprint(w, "browser-download")
		default:
			fmt.Fprint(w, fixture)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	b, e := NewChromium(ctx, NewChromiumOptions().SetBrowserPath(path).SetHeadless(true))
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	tab, e := b.NewTab(ctx, server.URL)
	if e != nil {
		t.Fatal(e)
	}
	listener, e := tab.Listen(ctx, ListenFilter{URLs: []string{"/api"}})
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Stop()
	if _, e = tab.RunJS(ctx, `()=>fetch('/api').then(r=>r.json())`); e != nil {
		t.Fatal(e)
	}
	packet, e := listener.Next(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if packet.Response.Status != 200 || string(packet.Body) != `{"ok":true}` || packet.BodyError != nil {
		t.Fatalf("packet: %+v", packet)
	}
	extraCtx, extraCancel := context.WithTimeout(ctx, 3*time.Second)
	extra, extraErr := packet.Extras.Response(extraCtx)
	extraCancel()
	if extraErr != nil || extra.StatusCode != 200 {
		t.Fatalf("extra info %+v %v", extra, extraErr)
	}
	console, e := tab.Console(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer console.Stop()
	if _, e = tab.RunJS(ctx, `()=>console.log('from-go')`); e != nil {
		t.Fatal(e)
	}
	msg, e := console.Next(ctx)
	if e != nil || len(msg.Args) != 1 || string(msg.Args[0]) != `"from-go"` {
		t.Fatalf("console: %+v %v", msg, e)
	}
	v := "stored"
	if e = tab.SetStorage(ctx, false, "key", &v); e != nil {
		t.Fatal(e)
	}
	store, e := tab.Storage(ctx, false)
	if e != nil || store["key"] != v {
		debug, de := tab.RunJS(ctx, `()=>({local:localStorage.getItem('key'),session:sessionStorage.getItem('key'),url:location.href})`)
		t.Logf("storage debug %s %v", debug, de)
		t.Fatalf("storage: %+v %v", store, e)
	}
	sel, e := tab.Ele(ctx, "@id=select")
	if e != nil {
		t.Fatal(e)
	}
	if e = sel.SelectByValue(ctx, "b"); e != nil {
		t.Fatal(e)
	}
	value, e := sel.Property(ctx, "value")
	if e != nil || string(value) != `"b"` {
		t.Fatalf("selection: %s %v", value, e)
	}
	manager, e := b.Downloads(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer manager.Close()
	if _, e = tab.RunJS(ctx, `()=>{const a=document.createElement('a');a.href='/download';a.download='test.txt';document.body.append(a);a.click()}`); e != nil {
		t.Fatal(e)
	}
	start, e := manager.Next(ctx)
	if e != nil {
		t.Fatal(e)
	}
	done, e := manager.Wait(ctx, start.GUID)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(done.Path)
	if e != nil || string(data) != "browser-download" {
		t.Fatalf("download: %s %v", data, e)
	}
	isolated, e := b.NewContext(ctx)
	if e != nil {
		t.Fatal(e)
	}
	other, e := isolated.NewTab(ctx, server.URL)
	if e != nil {
		t.Fatal(e)
	}
	otherStore, e := other.Storage(ctx, false)
	if e != nil || len(otherStore) != 0 {
		t.Fatalf("isolation: %+v %v", otherStore, e)
	}
	if e = isolated.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = tab.Title(ctx); e != nil {
		t.Fatal("context close shut down parent", e)
	}
	all, e := b.Listen(ctx, ListenFilter{URLs: []string{"/api"}})
	if e != nil {
		t.Fatal(e)
	}
	defer all.Stop()
	created, e := b.NewTab(ctx, server.URL+"/api")
	if e != nil {
		t.Fatal(e)
	}
	packet, e = all.Next(ctx)
	if e != nil || packet.TabID != created.ID() || packet.Response == nil || packet.Response.Status != 200 {
		t.Fatalf("browser listener %+v %v", packet, e)
	}
}
