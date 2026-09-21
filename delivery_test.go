package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProfilePreparation(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "copy")
	if err := writeProfileJSON(filepath.Join(source, "Profile 2", "Preferences"), map[string]any{"keep": 1, "group": map[string]any{"remove": true, "keep": 2}}); err != nil {
		t.Fatal(err)
	}
	if err := writeProfileJSON(filepath.Join(source, "Local State"), map[string]any{"other": 42, "browser": map[string]any{"enabled_labs_experiments": []string{"old@1", "replace@1"}}}); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(source, "Profile 2", "Preferences"))
	options := NewChromiumOptions().SetUser("Profile 2").CopySystemProfile(source).SetFlag("replace", 2).SetFlag("new", nil).RemovePrefFromFile("group.remove").SetPref("group.add", 3)
	options.UserDataPath = target
	if err := prepareBrowserProfile(*options); err != nil {
		t.Fatal(err)
	}
	var prefs map[string]any
	data, _ := os.ReadFile(filepath.Join(target, "Profile 2", "Preferences"))
	if err := json.Unmarshal(data, &prefs); err != nil {
		t.Fatal(err)
	}
	group := prefs["group"].(map[string]any)
	if _, ok := group["remove"]; ok || group["keep"] != float64(2) || group["add"] != float64(3) {
		t.Fatal(prefs)
	}
	state, _ := os.ReadFile(filepath.Join(target, "Local State"))
	if !strings.Contains(string(state), `["new","old@1","replace@2"]`) {
		t.Fatal(string(state))
	}
	unchanged, _ := os.ReadFile(filepath.Join(source, "Profile 2", "Preferences"))
	if string(unchanged) != string(original) {
		t.Fatal("source profile changed")
	}
	if err := copyProfile(source, filepath.Join(source, "nested")); err == nil {
		t.Fatal("recursive profile copy accepted")
	}
	options.SystemProfilePath = ""
	options.ClearFlags().ClearFlagsInFile()
	if err := prepareBrowserProfile(*options); err != nil {
		t.Fatal(err)
	}
	state, _ = os.ReadFile(filepath.Join(target, "Local State"))
	if !strings.Contains(string(state), `"enabled_labs_experiments":[]`) {
		t.Fatal(string(state))
	}
	options.SetUser("../escape")
	if err := prepareBrowserProfile(*options); err == nil {
		t.Fatal("invalid profile accepted")
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSessionHooksAndAdapters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "network") }))
	defer server.Close()
	page, err := NewSessionPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()
	adapter := func(text string) http.RoundTripper {
		return testRoundTripper(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(text)), Request: r}, nil
		})
	}
	if err = page.Mount(server.URL+"/", adapter("short")); err != nil {
		t.Fatal(err)
	}
	if err = page.Mount(server.URL+"/specific", adapter("long")); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err = page.SetResponseHooks(func(r *http.Response) error { calls++; r.Header.Set("X-Hook", "yes"); return nil }); err != nil {
		t.Fatal(err)
	}
	if err = page.SetVerifyTLS(true); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"/other": "short", "/specific": "long"} {
		res, err := page.Get(context.Background(), server.URL+path)
		if err != nil || res.Text != want || res.Headers.Get("X-Hook") != "yes" {
			t.Fatalf("%+v %v", res, err)
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
	if err = page.Mount(server.URL+"/", nil); err != nil {
		t.Fatal(err)
	}
	res, err := page.Get(context.Background(), server.URL+"/other")
	if err != nil || res.Text != "network" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestRecordingWaitersAndNativeWindow(t *testing.T) {
	browserPath := os.Getenv("DRISSIONPAGE_BROWSER")
	if browserPath == "" {
		t.Skip("set DRISSIONPAGE_BROWSER")
	}
	ffmpeg := os.Getenv("DRISSIONPAGE_FFMPEG")
	if ffmpeg == "" {
		t.Skip("set DRISSIONPAGE_FFMPEG")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	options := NewChromiumOptions().SetBrowserPath(browserPath).SetTempPath(t.TempDir())
	options.SetArgument("--auto-select-desktop-capture-source=DrissionPage Recording Test")
	options.SetArgument("--enable-usermedia-screen-capturing")
	options.SetArgument("--allow-http-screen-capture")
	options.SetPref("download.prompt_for_download", false)
	b, err := NewChromium(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><title>DrissionPage Recording Test</title><button id="title" onclick="document.title='Changed'">Title</button><button id="url" onclick="location.hash='changed'">URL</button><div id="moving" style="position:absolute;left:10px;top:100px;width:40px;height:40px;background:red"></div>`)
	}))
	defer server.Close()
	tab, err := b.NewTab(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if err = tab.Activate(ctx); err != nil {
			t.Fatal(err)
		}
		if err = WaitUntil(ctx, 50*time.Millisecond, func() (bool, error) { windows, err := tab.NativeWindows(ctx); return len(windows) > 0, err }); err != nil {
			t.Fatal(err)
		}
		if err = tab.HideWindow(ctx); err != nil {
			t.Fatal(err)
		}
		defer tab.ShowWindow(context.Background())
		windows, err := tab.NativeWindows(ctx)
		if err != nil || len(windows) == 0 || windows[0].Visible {
			t.Fatalf("hide: %+v %v", windows, err)
		}
		if err = tab.ShowWindow(ctx); err != nil {
			t.Fatal(err)
		}
		windows, err = tab.NativeWindows(ctx)
		if err != nil || len(windows) == 0 || !windows[0].Visible {
			t.Fatalf("show: %+v %v", windows, err)
		}
	}
	if runtime.GOOS == "windows" {
		element, err := tab.Ele(ctx, "#title")
		if err != nil {
			t.Fatal(err)
		}
		rect, err := element.ScreenRect(ctx)
		if err != nil || rect.Width <= 0 || rect.Height <= 0 {
			t.Fatalf("screen rect %+v %v", rect, err)
		}
	}
	dir := filepath.Join(t.TempDir(), "periodic")
	record, err := tab.StartPeriodicScreencast(ctx, dir, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err = WaitUntil(ctx, 50*time.Millisecond, func() (bool, error) { return record.Frames() >= 6, nil }); err != nil {
		t.Fatal(err)
	}
	if err = record.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err = record.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(t.TempDir(), "timeline.mp4")
	if err = record.TimelineVideo(ctx, ffmpeg, video); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(filepath.Dir(ffmpeg), "ffprobe"+filepath.Ext(ffmpeg))
	out, err := exec.CommandContext(ctx, probe, "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", video).Output()
	if err != nil {
		t.Fatal(err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || duration < 0.3 || duration > time.Since(start).Seconds()+1 {
		t.Fatalf("timeline duration %s %v", out, err)
	}
	display, err := tab.StartDisplayRecording(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer display.Cancel(context.Background())
	if _, err = tab.RunJS(ctx, `()=>new Promise(resolve=>{let n=0;const timer=setInterval(()=>{document.body.style.background=n++%2?'blue':'yellow';if(n===8){clearInterval(timer);resolve(true)}},100)})`); err != nil {
		t.Fatal(err)
	}
	if err = tab.WaitJS(ctx, `key=>globalThis[key].chunks.some(chunk=>chunk.size>0)`, display.key); err != nil {
		t.Fatal(err)
	}
	webm := filepath.Join(t.TempDir(), "display.webm")
	if err = display.Stop(ctx, webm); err != nil {
		t.Fatal(err)
	}
	if out, err = exec.CommandContext(ctx, probe, "-v", "error", "-show_entries", "stream=codec_type", "-of", "csv=p=0", webm).Output(); err != nil || !strings.Contains(string(out), "video") {
		t.Fatalf("webm %s %v", out, err)
	}
	if err = tab.WaitElements(ctx, []any{"#moving", "#url"}, false); err != nil {
		t.Fatal(err)
	}
	moving, err := tab.Ele(ctx, "#moving")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tab.RunJS(ctx, `()=>{setTimeout(()=>document.querySelector('#moving').style.left='120px',60)}`); err != nil {
		t.Fatal(err)
	}
	if err = moving.WaitStopMoving(ctx, 150*time.Millisecond, 0.1); err != nil {
		t.Fatal(err)
	}
	rect, err := moving.Rect(ctx)
	if err != nil || rect.X != 120 {
		t.Fatalf("moving %+v %v", rect, err)
	}
	button, err := tab.Ele(ctx, "#title")
	if err != nil {
		t.Fatal(err)
	}
	if err = button.ClickForTitleChange(ctx); err != nil {
		t.Fatal(err)
	}
	button, err = tab.Ele(ctx, "#url")
	if err != nil {
		t.Fatal(err)
	}
	if err = button.ClickForURLChange(ctx); err != nil {
		t.Fatal(err)
	}
	if err = tab.DuringLoadStart(ctx, func() error { return tab.Get(ctx, server.URL+"/next") }); err != nil {
		t.Fatal(err)
	}
	stop, err := tab.AutoHandleAlerts(ctx, true, "")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if err = tab.DuringAlert(ctx, func() error { _, err := tab.RunJS(ctx, `()=>alert('waiter')`); return err }); err != nil {
		t.Fatal(err)
	}
}
