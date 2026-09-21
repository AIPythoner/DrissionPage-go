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
)

func TestTextAndRelations(t *testing.T) {
	doc, e := MakeSessionElement(`<div id="content"><p>one <b>bold</b></p><p>two<br>three</p><script>hidden script</script><table><tr><td>A</td><td>B</td></tr></table></div>`)
	if e != nil {
		t.Fatal(e)
	}
	el, e := doc.Ele("@id=content")
	if e != nil {
		t.Fatal(e)
	}
	if got := el.Text(); got != "one bold\ntwo\nthree\nA\tB" {
		t.Fatalf("formatted text %q", got)
	}
	p, e := el.Ele("tag:p", 2)
	if e != nil {
		t.Fatal(e)
	}
	previous, e := p.Relative(PreviousSibling, 1, ElementFilter{Tag: "p"})
	if e != nil || previous.Text() != "one bold" {
		t.Fatalf("relative %v %v", previous, e)
	}
	count, e := el.XPathValues("count(.//p)")
	if e != nil || count != float64(2) {
		t.Fatalf("xpath %v %v", count, e)
	}
	found, e := doc.Ele(XPath(p.Path()))
	if e != nil || found.Text() != p.Text() {
		t.Fatalf("path lookup %v %v", found, e)
	}
}
func TestHybridAndAdvanced(t *testing.T) {
	path := os.Getenv("DRISSIONPAGE_BROWSER")
	if path == "" {
		t.Skip("set DRISSIONPAGE_BROWSER")
	}
	frameServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<h2 id="cross">cross origin</h2>`)
	}))
	defer frameServer.Close()
	frameURL := strings.Replace(frameServer.URL, "127.0.0.1", "localhost", 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Replace(fixture, "</body>", `<iframe id="crossframe" src="`+frameURL+`"></iframe></body>`, 1))
	}))
	defer server.Close()
	ctx, c := context.WithTimeout(context.Background(), 40*time.Second)
	defer c()
	b, e := NewChromium(ctx, NewChromiumOptions().SetHeadless(true).SetBrowserPath(path).SetArgument("--site-per-process"))
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	tab, e := b.NewTab(ctx, server.URL)
	if e != nil {
		t.Fatal(e)
	}
	ax, e := tab.Ele(ctx, "ax:role=button@name=提交")
	if e != nil {
		t.Fatal(e)
	}
	if text, e := ax.Text(ctx); e != nil || text != "提交" {
		t.Fatalf("ax %s %v", text, e)
	}
	cross, e := tab.GetFrame(ctx, "@id=crossframe")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = cross.Ele(ctx, "@id=cross"); e != nil {
		h, he := cross.HTML(ctx)
		t.Logf("frame HTML: %s %v", h, he)
		d, de := tab.RunCDP(ctx, "Target.getTargets", nil)
		t.Logf("targets %s %v", d, de)
		t.Fatal("cross origin frame", e)
	}
	if cross.page.SessionID == tab.page.SessionID {
		t.Fatal("test requires an isolated iframe CDP session")
	}
	if e = cross.Get(ctx, frameURL+"/next"); e != nil {
		t.Fatal(e)
	}
	if _, e = cross.Ele(ctx, "@id=cross"); e != nil {
		t.Fatal(e)
	}
	if u, e := tab.URL(ctx); e != nil || u != server.URL+"/" {
		t.Fatalf("frame navigation changed parent: %s %v", u, e)
	}
	if e = cross.Close(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = tab.Title(ctx); e != nil {
		t.Fatal("frame close shut down parent", e)
	}
	el, e := tab.NewElement(ctx, `<select id="multi" multiple><option value="1">one</option><option value="2">two</option></select>`)
	if e != nil {
		t.Fatal(e)
	}
	if e = el.SelectAll(ctx); e != nil {
		t.Fatal(e)
	}
	selected, e := el.SelectedOptions(ctx)
	if e != nil || len(selected) != 2 {
		t.Fatalf("select all %+v %v", selected, e)
	}
	if e = el.InvertSelection(ctx); e != nil {
		t.Fatal(e)
	}
	selected, e = el.SelectedOptions(ctx)
	if e != nil || len(selected) != 0 {
		t.Fatalf("invert %+v %v", selected, e)
	}
	if e = el.SelectByIndex(ctx, -1); e != nil {
		t.Fatal(e)
	}
	option, e := el.SelectedOption(ctx)
	if e != nil || option.Value != "2" {
		t.Fatalf("index %+v %v", option, e)
	}
	geometry, e := tab.Rect(ctx)
	if e != nil || geometry.ViewportWidth <= 0 {
		t.Fatalf("geometry %+v %v", geometry, e)
	}
	hybrid, e := tab.Hybrid(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer hybrid.session.Close()
	if e = hybrid.ChangeMode(ctx, "s", true, true); e != nil {
		t.Fatal(e)
	}
	sEl, e := hybrid.Ele(ctx, "@id=link")
	if e != nil {
		t.Fatal(e)
	}
	if text, e := sEl.Text(ctx); e != nil || text != "你好 Go" {
		t.Fatalf("session mode %s %v", text, e)
	}
	if e = hybrid.ChangeMode(ctx, "d", true, true); e != nil {
		t.Fatal(e)
	}
	if title, e := hybrid.Title(ctx); e != nil || title != "迁移测试" {
		t.Fatalf("browser mode %s %v", title, e)
	}
	pdf, e := tab.PDF(ctx, "")
	if e != nil || !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("pdf %v", e)
	}
	mhtml := filepath.Join(t.TempDir(), "page.mhtml")
	if e = tab.SaveMHTML(ctx, mhtml); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(mhtml)
	if e != nil || !strings.Contains(string(data), "multipart/related") {
		t.Fatalf("mhtml %v", e)
	}
}
