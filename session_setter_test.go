package drissionpage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionRuntimeSettings(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/retry" && attempts.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(r.Header.Get("User-Agent") + "/" + r.Header.Get("X-Test")))
	}))
	defer server.Close()
	page, err := NewSessionPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()
	headers := http.Header{"x-test": {"original"}}
	if err = page.SetHeaders(headers); err != nil {
		t.Fatal(err)
	}
	headers["x-test"][0] = "mutated"
	if err = page.SetUserAgent("go-port"); err != nil {
		t.Fatal(err)
	}
	if err = page.SetRetry(1, 0); err != nil {
		t.Fatal(err)
	}
	response, err := page.Get(context.Background(), server.URL+"/retry")
	if err != nil || response.Text != "go-port/original" || attempts.Load() != 2 {
		t.Fatalf("response=%+v attempts=%d err=%v", response, attempts.Load(), err)
	}
	response, err = page.Get(context.Background(), server.URL, RequestOptions{Headers: http.Header{"X-Test": {"override"}}})
	if err != nil || response.Text != "go-port/override" {
		t.Fatalf("override: %+v %v", response, err)
	}
	if err = page.SetRetry(0, 0); err != nil {
		t.Fatal(err)
	}
	if err = page.SetTimeout(20 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	_, err = page.Get(context.Background(), server.URL+"/slow")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
	if page.SetTimeout(-time.Second) == nil || page.SetRetry(-1, 0) == nil {
		t.Fatal("negative settings accepted")
	}
	page.Close()
	if !errors.Is(page.SetHeader("X-Test", "closed"), ErrClosed) {
		t.Fatal("closed setter accepted")
	}
}
