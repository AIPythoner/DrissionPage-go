package drissionpage

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFilenamePolicies(t *testing.T) {
	for _, mode := range []FileExistsMode{FileRename, FileOverwrite, FileSkip, FileError} {
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "guid")
			destination := filepath.Join(dir, "result.txt")
			if e := os.WriteFile(source, []byte("new"), 0600); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(destination, []byte("old"), 0600); e != nil {
				t.Fatal(e)
			}
			d := &DownloadManager{directory: dir, missions: map[string]BrowserDownload{"guid": {GUID: "guid", Path: source, SuggestedFilename: "result.txt", State: "completed"}}}
			path, e := d.Finalize(context.Background(), "guid", "", mode)
			if mode == FileError {
				if !os.IsExist(e) {
					t.Fatalf("expected collision, got %v", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			data, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			want := "new"
			if mode == FileSkip {
				want = "old"
			}
			if string(data) != want {
				t.Fatalf("got %q", data)
			}
			if mode == FileRename && filepath.Base(path) != "result (1).txt" {
				t.Fatal(path)
			}
		})
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "guid")
	os.WriteFile(source, []byte("data"), 0600)
	d := &DownloadManager{directory: dir, missions: map[string]BrowserDownload{"guid": {GUID: "guid", Path: source, State: "completed"}}}
	if _, e := d.Finalize(context.Background(), "guid", "../escape.txt", FileOverwrite); e == nil {
		t.Fatal("accepted path traversal")
	}
	if _, e := os.Stat(source); e != nil {
		t.Fatal("changed file while rejecting filename", e)
	}
}
func TestCookieMetadataAndDeletion(t *testing.T) {
	p, e := NewSessionPage()
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if e = p.SetCookies("https://example.com/a/b", []*http.Cookie{{Name: "sid", Value: "secret", Path: "/a", HttpOnly: true, Secure: true}}); e != nil {
		t.Fatal(e)
	}
	all, e := p.AllCookies()
	if e != nil || len(all) != 1 || !all[0].HttpOnly || all[0].Path != "/a" {
		t.Fatalf("metadata %+v %v", all, e)
	}
	if e = p.DeleteCookie("https://example.com/a/b", "sid"); e != nil {
		t.Fatal(e)
	}
	all, e = p.AllCookies()
	if e != nil || len(all) != 0 {
		t.Fatalf("deletion %+v %v", all, e)
	}
}
